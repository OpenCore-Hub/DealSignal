// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { DocumentsTable } from "./DocumentsTable";
import type { Document } from "@/types";

const { getDocumentsMock, getLinksMock } = vi.hoisted(() => ({
  getDocumentsMock: vi.fn(),
  getLinksMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getDocuments: getDocumentsMock,
    getLinks: getLinksMock,
  },
}));

vi.mock("@/lib/formatters", () => ({
  formatFileSize: vi.fn(() => "1 MB"),
  formatDate: vi.fn(() => "Jun 20, 2026"),
}));

const resources = {
  en: {
    documents: {
      filters: {
        all: "All Documents",
        recent: "Recently Accessed",
        popular: "High Popularity",
        unshared: "Unshared",
        archived: "Archived",
      },
      table: {
        emptyTitle: "Empty library",
        emptyDescription: "Upload a document to get started.",
        upload: "Upload Document",
        searchPlaceholder: "Search documents...",
        documentCount: "{{count}} documents",
        documentCountFiltered: "{{count}} documents · {{filtered}} filtered",
        noMatches: "No matching documents found",
        emptyFilter: "No documents in this view",
        clearFilter: "Clear filter",
      },
      columns: {
        file: "File",
        heat: "Heat",
        views: "Views",
        status: "Status",
        shareLinks: "Links",
        pages: "{{count}} pages",
        links: "{{count}} links",
        viewCount: "{{count}} views",
      },
      status: {
        uploading: "Uploading",
        processing: "Processing",
        ready: "Ready",
        failed: "Failed",
        archived: "Archived",
        pending: "Pending",
      },
    },
    common: { retry: "Retry", preview: "Preview", view: "View", addToDealRoom: "Add to Deal Room" },
  },
};

async function initI18n() {
  const instance = i18n.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["documents", "common"],
    defaultNS: "documents",
    resources,
    interpolation: { escapeValue: false },
  });
  return instance;
}

const mockDocs: Document[] = [
  {
    id: "doc_1",
    title: "Pitch Deck",
    sourceType: "pdf",
    fileName: "Pitch Deck.pdf",
    fileType: "pdf",
    fileSize: 1_000_000,
    pageCount: 10,
    status: "ready",
    createdAt: "2026-06-20T10:00:00Z",
    updatedAt: "2026-06-20T10:00:00Z",
  },
  {
    id: "doc_2",
    title: "Old Report",
    sourceType: "pdf",
    fileName: "Old Report.pdf",
    fileType: "pdf",
    fileSize: 500_000,
    pageCount: 5,
    status: "archived",
    createdAt: "2026-06-10T10:00:00Z",
    updatedAt: "2026-06-10T10:00:00Z",
  },
];

async function renderTable() {
  const instance = await initI18n();
  return render(
    <I18nextProvider i18n={instance}>
      <MemoryRouter initialEntries={["/acme/documents"]}>
        <Routes>
          <Route path="/:workspaceSlug/documents" element={<DocumentsTable />} />
        </Routes>
      </MemoryRouter>
    </I18nextProvider>
  );
}

describe("DocumentsTable", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getLinksMock.mockResolvedValue({ data: [] });
  });

  it("fetches all documents by default", async () => {
    getDocumentsMock.mockResolvedValue({ data: mockDocs });
    await renderTable();

    await waitFor(() => expect(getDocumentsMock).toHaveBeenCalledWith("all", undefined));
    expect(await screen.findByText("Pitch Deck")).toBeInTheDocument();
    expect(screen.getByText("Old Report")).toBeInTheDocument();
  });

  it("switches filters and refetches documents", async () => {
    getDocumentsMock.mockResolvedValue({ data: mockDocs });
    await renderTable();
    await waitFor(() => expect(getDocumentsMock).toHaveBeenCalledWith("all", undefined));

    fireEvent.click(screen.getByRole("tab", { name: "Archived" }));

    await waitFor(() => expect(getDocumentsMock).toHaveBeenLastCalledWith("archived", undefined));
  });

  it("hides search and top upload button when the library is empty", async () => {
    getDocumentsMock.mockResolvedValue({ data: [] });
    await renderTable();

    await waitFor(() => expect(getDocumentsMock).toHaveBeenCalledWith("all", undefined));
    expect(await screen.findByText("Empty library")).toBeInTheDocument();

    expect(screen.queryByPlaceholderText("Search documents...")).not.toBeInTheDocument();
    // The empty-state call-to-action still offers an upload button.
    expect(screen.getByRole("button", { name: "Upload Document" })).toBeInTheDocument();
  });

  it("shows search and upload button when documents exist", async () => {
    getDocumentsMock.mockResolvedValue({ data: mockDocs });
    await renderTable();

    await waitFor(() => expect(getDocumentsMock).toHaveBeenCalledWith("all", undefined));
    expect(await screen.findByText("Pitch Deck")).toBeInTheDocument();

    expect(screen.getByPlaceholderText("Search documents...")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Upload Document" })).toBeInTheDocument();
  });

  it("shows a filter-specific empty state when the filtered list is empty", async () => {
    getDocumentsMock.mockImplementation((filter: string | undefined) =>
      Promise.resolve({ data: filter === "archived" ? [] : mockDocs })
    );
    await renderTable();
    await waitFor(() => expect(getDocumentsMock).toHaveBeenCalledWith("all", undefined));

    fireEvent.click(screen.getByRole("tab", { name: "Archived" }));
    await waitFor(() => expect(getDocumentsMock).toHaveBeenLastCalledWith("archived", undefined));

    expect(await screen.findByText("No documents in this view")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Clear filter" })).toBeInTheDocument();
  });
});
