// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { DocumentsTable } from "./DocumentsTable";
import type { Document } from "@/types";

const { getDocumentsMock, getLinksMock, getPageSignedUrlMock } = vi.hoisted(() => ({
  getDocumentsMock: vi.fn(),
  getLinksMock: vi.fn(),
  getPageSignedUrlMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getDocuments: getDocumentsMock,
    getLinks: getLinksMock,
    getPageSignedUrl: getPageSignedUrlMock,
  },
}));

vi.mock("@/hooks/useDocumentUploadConflict", () => ({
  UploadCancelledError: class UploadCancelledError extends Error {
    constructor(message = "upload_cancelled") {
      super(message);
      this.name = "UploadCancelledError";
    }
  },
  useDocumentUploadConflict: () => ({
    uploadDocument: vi.fn(),
    conflictDialog: null,
    isAwaitingConflict: false,
  }),
}));

vi.mock("@/components/links/LinksTable", () => ({
  LinksTable: ({ documentId }: { documentId?: string }) => (
    <div data-testid="links-table">{documentId ? `filtered:${documentId}` : "all-links"}</div>
  ),
}));

vi.mock("@/lib/formatters", () => ({
  formatFileSize: vi.fn(() => "1 MB"),
  formatDate: vi.fn(() => "Jun 20, 2026"),
}));

const resources = {
  en: {
    documents: {
      filters: {
        all: "Documents",
        shared: "Shared",
        recent: "Recently Accessed",
        popular: "High Popularity",
        unshared: "Unshared",
        archived: "Archived",
      },
      table: {
        emptyTitle: "Empty library",
        emptyDescription: "Upload a document to get started.",
        upload: "Upload Document",
        downloadTemplate: "Download Template",
        searchPlaceholder: "Search documents...",
        documentCount: "{{count}} documents",
        documentCountFiltered: "{{count}} documents · {{filtered}} filtered",
        noMatches: "No matching documents found",
        emptyFilter: "No documents in this view",
        clearFilter: "Clear filter",
        templates: {
          ndaCN: "NDA (Chinese)",
          ndaEN: "NDA (English)",
        },
      },
      columns: {
        file: "File",
        heat: "Heat",
        views: "Views",
        status: "Status",
        shareLinks: "Links",
        pages: "{{count}} pages",
        pages_one: "{{count}} page",
        pages_other: "{{count}} pages",
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
    links: {
      page: {
        createLink: "Create Link",
      },
    },
    agreementDocuments: {
      page: {
        documentsTitle: "Agreements",
        documentsHint: "Upload PDF NDAs here.",
        fileCount_one: "{{count}} file",
        fileCount_other: "{{count}} files",
        emptyTitle: "Add your first NDA",
        emptyDescription: "Upload a PDF NDA.",
        pdfOnly: "Agreement files must be PDF.",
        uploadSuccess: "Agreement uploaded",
        previewUnavailable: "Preview unavailable",
      },
    },
    common: {
      retry: "Retry",
      preview: "Preview",
      view: "View",
      delete: "Delete",
      addToDealRoom: "Add to Deal Room",
    },
  },
};

async function initI18n() {
  const instance = i18n.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["documents", "common", "links", "agreementDocuments"],
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
    getPageSignedUrlMock.mockResolvedValue({
      page_number: 1,
      image_url: "https://example.test/page-1.png",
      expires_at: "2099-01-01T00:00:00Z",
      width: 800,
      height: 1100,
    });
  });

  it("fetches all documents by default", async () => {
    getDocumentsMock.mockResolvedValue({ data: mockDocs });
    await renderTable();

    await waitFor(() =>
      expect(getDocumentsMock).toHaveBeenCalledWith("all", undefined, {
        excludeDealRoom: true,
        excludeAgreement: true,
      }),
    );
    expect(await screen.findByText("Pitch Deck")).toBeInTheDocument();
    expect(screen.getByText("Old Report")).toBeInTheDocument();
  });

  it("switches filters and refetches documents", async () => {
    getDocumentsMock.mockResolvedValue({ data: mockDocs });
    await renderTable();
    await waitFor(() =>
      expect(getDocumentsMock).toHaveBeenCalledWith("all", undefined, {
        excludeDealRoom: true,
        excludeAgreement: true,
      }),
    );

    expect(screen.getByRole("tab", { name: "Documents" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Shared" })).toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: "Recently Accessed" })).not.toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: "Unshared" })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "Shared" }));
    expect(await screen.findByTestId("links-table")).toHaveTextContent("all-links");
    expect(screen.getByRole("button", { name: "Create Link" })).toBeInTheDocument();
    expect(getDocumentsMock).not.toHaveBeenCalledWith("shared", undefined, {
      excludeDealRoom: true,
      excludeAgreement: true,
    });

    fireEvent.click(screen.getByRole("tab", { name: "Archived" }));

    await waitFor(() =>
      expect(getDocumentsMock).toHaveBeenLastCalledWith("archived", undefined, {
        excludeDealRoom: true,
        excludeAgreement: true,
      }),
    );
  });

  it("hides search and top upload button when the library is empty", async () => {
    getDocumentsMock.mockResolvedValue({ data: [] });
    await renderTable();

    await waitFor(() =>
      expect(getDocumentsMock).toHaveBeenCalledWith("all", undefined, {
        excludeDealRoom: true,
        excludeAgreement: true,
      }),
    );
    expect(await screen.findByText("Empty library")).toBeInTheDocument();

    expect(screen.queryByPlaceholderText("Search documents...")).not.toBeInTheDocument();
    // The empty-state call-to-action still offers an upload button.
    expect(screen.getByRole("button", { name: "Upload Document" })).toBeInTheDocument();
  });

  it("keeps Share tab reachable when document fetch fails", async () => {
    getDocumentsMock.mockRejectedValue(new Error("boom"));
    await renderTable();

    expect(await screen.findByText("boom")).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Shared" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "Shared" }));
    expect(await screen.findByTestId("links-table")).toHaveTextContent("all-links");
  });

  it("shows search and upload on Documents tab and hides them on other tabs", async () => {
    getDocumentsMock.mockResolvedValue({ data: mockDocs });
    await renderTable();

    await waitFor(() =>
      expect(getDocumentsMock).toHaveBeenCalledWith("all", undefined, {
        excludeDealRoom: true,
        excludeAgreement: true,
      }),
    );
    expect(await screen.findByText("Pitch Deck")).toBeInTheDocument();

    expect(screen.getByPlaceholderText("Search documents...")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Upload Document" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "Shared" }));
    expect(await screen.findByTestId("links-table")).toBeInTheDocument();
    expect(screen.queryByPlaceholderText("Search documents...")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Upload Document" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create Link" })).toBeInTheDocument();
  });

  it("shows a filter-specific empty state when the filtered list is empty", async () => {
    getDocumentsMock.mockImplementation((filter: string | undefined) =>
      Promise.resolve({ data: filter === "archived" ? [] : mockDocs })
    );
    await renderTable();
    await waitFor(() =>
      expect(getDocumentsMock).toHaveBeenCalledWith("all", undefined, {
        excludeDealRoom: true,
        excludeAgreement: true,
      }),
    );

    fireEvent.click(screen.getByRole("tab", { name: "Archived" }));
    await waitFor(() =>
      expect(getDocumentsMock).toHaveBeenLastCalledWith("archived", undefined, {
        excludeDealRoom: true,
        excludeAgreement: true,
      }),
    );

    expect(await screen.findByText("No documents in this view")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Clear filter" })).toBeInTheDocument();
  });

  it("agreement category uses inline PDF upload instead of navigating away", async () => {
    getDocumentsMock.mockResolvedValue({ data: [] });
    const instance = await initI18n();
    render(
      <I18nextProvider i18n={instance}>
        <MemoryRouter initialEntries={["/acme/agreement-documents"]}>
          <Routes>
            <Route
              path="/:workspaceSlug/agreement-documents"
              element={<DocumentsTable category="agreement" />}
            />
            <Route path="/:workspaceSlug/documents/upload" element={<div>upload-page</div>} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>,
    );

    await waitFor(() =>
      expect(getDocumentsMock).toHaveBeenCalledWith("all", "agreement", {
        excludeDealRoom: true,
        excludeAgreement: false,
      }),
    );
    expect(await screen.findByText("Add your first NDA")).toBeInTheDocument();

    const input = screen.getByTestId("agreement-file-input") as HTMLInputElement;
    expect(input.accept).toMatch(/pdf/i);

    fireEvent.click(screen.getByTestId("agreement-upload-button"));
    expect(screen.queryByText("upload-page")).not.toBeInTheDocument();
    expect(input).toBeInTheDocument();
  });

  it("agreement category renders document cards instead of a table", async () => {
    getDocumentsMock.mockResolvedValue({ data: [mockDocs[0]] });
    const instance = await initI18n();
    render(
      <I18nextProvider i18n={instance}>
        <MemoryRouter initialEntries={["/acme/agreement-documents"]}>
          <Routes>
            <Route
              path="/:workspaceSlug/agreement-documents"
              element={<DocumentsTable category="agreement" />}
            />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>,
    );

    expect(await screen.findByTestId("agreement-doc-cards")).toBeInTheDocument();
    expect(await screen.findByTestId("agreement-doc-card-doc_1")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    expect(await screen.findByRole("img", { name: "Pitch Deck" })).toHaveAttribute(
      "src",
      "https://example.test/page-1.png",
    );
  });
});
