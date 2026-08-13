// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, within } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { DocumentsTable } from "./DocumentsTable";
import type { Document } from "@/types";

const {
  getDocumentsMock,
  getLinksMock,
  getPageSignedUrlMock,
  getDocumentDeleteImpactMock,
  archiveDocumentMock,
  createLinkMock,
} = vi.hoisted(() => ({
  getDocumentsMock: vi.fn(),
  getLinksMock: vi.fn(),
  getPageSignedUrlMock: vi.fn(),
  getDocumentDeleteImpactMock: vi.fn(),
  archiveDocumentMock: vi.fn(),
  createLinkMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getDocuments: getDocumentsMock,
    getLinks: getLinksMock,
    getPageSignedUrl: getPageSignedUrlMock,
    getDocumentDeleteImpact: getDocumentDeleteImpactMock,
    archiveDocument: archiveDocumentMock,
    unarchiveDocument: vi.fn(),
    createLink: createLinkMock,
  },
}));

vi.mock("@/lib/clipboard", () => ({
  copyToClipboard: vi.fn().mockResolvedValue(true),
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

vi.mock("@/hooks/useWorkspaceAccess", () => ({
  useWorkspaceAccess: () => ({
    role: "member",
    loading: false,
    canRead: true,
    canWrite: true,
    canManage: false,
    isGuest: false,
  }),
}));

vi.mock("@/components/links/LinksTable", () => ({
  LinksTable: ({
    documentId,
    toolbar,
  }: {
    documentId?: string;
    toolbar?: React.ReactNode;
  }) => (
    <div data-testid="links-table">
      {toolbar}
      <span>{documentId ? `filtered:${documentId}` : "all-links"}</span>
    </div>
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
        actions: "Actions",
        pages: "{{count}} pages",
        pages_one: "{{count}} page",
        pages_other: "{{count}} pages",
        links: "{{count}} links",
        viewCount: "{{count}} views",
        archived: "Document archived",
        archiveFailed: "Failed to update document status",
        archiveDisabled: "Only ready documents can be archived",
        archivedActionDisabled: "Unarchive first",
        downloadNotReady: "Not ready",
        downloadFailed: "Download failed",
        deleteBusy: "Busy",
      },
      share: {
        cta: "Share",
        title: "Share document",
        description: "Create a share link for “{{name}}”.",
        defaultsHint: "Uses create-link defaults.",
        createAndCopy: "Create",
        creating: "Creating…",
        advanced: "Advanced settings",
        copied: "Share link copied",
        createFailed: "Failed",
        notReady: "Not ready",
      },
      archive: {
        title: "Archive document?",
        description: "“{{name}}” will move to Archived in your library.",
        visitorRevoke:
          "Visitors will no longer be able to open this document through existing share links.",
        unarchiveDoesNotRestore:
          "Unarchiving later will not automatically reactivate those share links. Renew them from Links.",
        withLinks_one: "This document is on {{count}} active share link.",
        withLinks_other: "This document is on {{count}} active share links.",
        confirmLoading: "Archiving…",
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
      table: {
        searchPlaceholder: "Search links…",
        createdWithinLabel: "Create time",
        createdWithin: {
          all: "All",
          "24h": "Last 24 hours",
          "7d": "Last 7 days",
          "30d": "Last 30 days",
          "90d": "Last 90 days",
        },
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
      cancel: "Cancel",
      archive: "Archive",
      unarchive: "Unarchive",
      createLink: "Create Link",
      download: "Download",
      moreActions: "More actions",
      addToDealRoom: "Add to Data Room",
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
    getDocumentDeleteImpactMock.mockResolvedValue({
      active_link_count: 2,
      deal_room_count: 0,
    });
    archiveDocumentMock.mockResolvedValue({ ...mockDocs[0], status: "archived" });
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
      expect(getDocumentsMock).toHaveBeenCalledWith("all", "general"),
    );
    expect(await screen.findByText("Pitch Deck")).toBeInTheDocument();
    // Archived docs belong on the Archived tab, not Documents.
    expect(screen.queryByText("Old Report")).not.toBeInTheDocument();
  });

  it("opens library share dialog from row Share CTA", async () => {
    getDocumentsMock.mockResolvedValue({ data: mockDocs });
    createLinkMock.mockResolvedValue({
      id: "link_new",
      shortUrl: "https://example.test/v/new",
    });
    await renderTable();
    expect(await screen.findByText("Pitch Deck")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("document-row-share"));
    const dialog = await screen.findByTestId("document-share-dialog");
    expect(within(dialog).getByText("Share document")).toBeInTheDocument();
    fireEvent.click(within(dialog).getByTestId("document-share-create"));
    await waitFor(() => {
      expect(createLinkMock).toHaveBeenCalledWith(
        ["doc_1"],
        expect.objectContaining({ watermarkEnabled: true, expiryDays: 30 }),
      );
    });
  });

  it("does not open Share dialog from processing upload handoff", async () => {
    getDocumentsMock.mockResolvedValue({
      data: [{ ...mockDocs[0], status: "processing" }],
    });
    const instance = await initI18n();
    render(
      <I18nextProvider i18n={instance}>
        <MemoryRouter
          initialEntries={[
            "/acme/documents?shareDocumentId=doc_1&shareDocumentTitle=Pitch%20Deck&shareDocumentStatus=processing",
          ]}
        >
          <Routes>
            <Route path="/:workspaceSlug/documents" element={<DocumentsTable />} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>,
    );

    expect(await screen.findByText("Pitch Deck")).toBeInTheDocument();
    expect(screen.queryByTestId("document-share-dialog")).not.toBeInTheDocument();
  });

  it("opens Share dialog from ready upload handoff only", async () => {
    getDocumentsMock.mockResolvedValue({ data: mockDocs });
    const instance = await initI18n();
    render(
      <I18nextProvider i18n={instance}>
        <MemoryRouter
          initialEntries={[
            "/acme/documents?shareDocumentId=doc_1&shareDocumentTitle=Pitch%20Deck&shareDocumentStatus=ready",
          ]}
        >
          <Routes>
            <Route path="/:workspaceSlug/documents" element={<DocumentsTable />} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>,
    );

    expect(await screen.findByTestId("document-share-dialog")).toBeInTheDocument();
  });

  it("opens deferred Share dialog when processing handoff becomes ready", async () => {
    getDocumentsMock
      .mockResolvedValueOnce({
        data: [{ ...mockDocs[0], status: "processing" }],
      })
      .mockResolvedValue({ data: mockDocs });
    const instance = await initI18n();
    render(
      <I18nextProvider i18n={instance}>
        <MemoryRouter
          initialEntries={[
            "/acme/documents?shareDocumentId=doc_1&shareDocumentTitle=Pitch%20Deck&shareDocumentStatus=processing",
          ]}
        >
          <Routes>
            <Route path="/:workspaceSlug/documents" element={<DocumentsTable />} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>,
    );

    expect(await screen.findByText("Pitch Deck")).toBeInTheDocument();
    expect(screen.queryByTestId("document-share-dialog")).not.toBeInTheDocument();

    await waitFor(
      () => {
        expect(screen.getByTestId("document-share-dialog")).toBeInTheDocument();
      },
      { timeout: 5000 },
    );
  });

  it("confirms archive with visitor revoke copy and link count", async () => {
    getDocumentsMock.mockResolvedValue({ data: mockDocs });
    getLinksMock.mockResolvedValue({
      data: [
        {
          id: "link_1",
          documentId: "doc_1",
          name: "Share",
          shortUrl: "https://example.test/v/abc",
          accessCount: 3,
          isActive: true,
          createdAt: "2026-06-20T10:00:00Z",
          updatedAt: "2026-06-20T10:00:00Z",
        },
      ],
    });
    await renderTable();
    expect(await screen.findByText("Pitch Deck")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "More actions" }));
    const menu = await screen.findByRole("menu");
    fireEvent.click(within(menu).getByRole("menuitem", { name: /^Archive$/i }));

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Archive document?")).toBeInTheDocument();
    expect(
      within(dialog).getByText(
        "Visitors will no longer be able to open this document through existing share links.",
      ),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByText(
        "Unarchiving later will not automatically reactivate those share links. Renew them from Links.",
      ),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(
        within(dialog).getByText("This document is on 2 active share links."),
      ).toBeInTheDocument();
    });

    fireEvent.click(within(dialog).getByRole("button", { name: /^Archive$/i }));
    await waitFor(() => {
      expect(archiveDocumentMock).toHaveBeenCalledWith("doc_1");
    });
  });

  it("switches filters and refetches documents", async () => {
    getDocumentsMock.mockResolvedValue({ data: mockDocs });
    await renderTable();
    await waitFor(() =>
      expect(getDocumentsMock).toHaveBeenCalledWith("all", "general"),
    );

    expect(screen.getByRole("tab", { name: "Documents" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Shared" })).toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: "Recently Accessed" })).not.toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: "Unshared" })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "Shared" }));
    expect(await screen.findByTestId("links-table")).toHaveTextContent("all-links");
    expect(screen.getByRole("button", { name: "Create Link" })).toBeInTheDocument();
    expect(getDocumentsMock).not.toHaveBeenCalledWith("shared", "general");

    fireEvent.click(screen.getByRole("tab", { name: "Archived" }));

    await waitFor(() =>
      expect(getDocumentsMock).toHaveBeenLastCalledWith("archived", "general"),
    );
    expect(await screen.findByText("Old Report")).toBeInTheDocument();
    expect(screen.queryByText("Pitch Deck")).not.toBeInTheDocument();
  });

  it("hides search and top upload button when the library is empty", async () => {
    getDocumentsMock.mockResolvedValue({ data: [] });
    await renderTable();

    await waitFor(() =>
      expect(getDocumentsMock).toHaveBeenCalledWith("all", "general"),
    );
    expect(await screen.findByText("Empty library")).toBeInTheDocument();

    expect(screen.queryByPlaceholderText("Search documents...")).not.toBeInTheDocument();
    // The empty-state call-to-action still offers an upload button.
    expect(screen.getByRole("button", { name: "Upload Document" })).toBeInTheDocument();
  });

  it("keeps Share tab reachable when document fetch fails", async () => {
    getDocumentsMock.mockRejectedValue(new Error("boom"));
    await renderTable();

    expect(await screen.findByText("Failed to load")).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Shared" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "Shared" }));
    expect(await screen.findByTestId("links-table")).toHaveTextContent("all-links");
  });

  it("shows search and upload on Documents tab and hides them on other tabs", async () => {
    getDocumentsMock.mockResolvedValue({ data: mockDocs });
    await renderTable();

    await waitFor(() =>
      expect(getDocumentsMock).toHaveBeenCalledWith("all", "general"),
    );
    expect(await screen.findByText("Pitch Deck")).toBeInTheDocument();

    expect(screen.getByPlaceholderText("Search documents...")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Upload Document" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "Shared" }));
    expect(await screen.findByTestId("links-table")).toBeInTheDocument();
    expect(screen.queryByPlaceholderText("Search documents...")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Upload Document" })).not.toBeInTheDocument();
    expect(screen.getByPlaceholderText("Search links…")).toBeInTheDocument();
    expect(screen.getByTestId("share-created-within")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create Link" })).toBeInTheDocument();
  });

  it("shows a filter-specific empty state when the filtered list is empty", async () => {
    getDocumentsMock.mockImplementation((filter: string | undefined) =>
      Promise.resolve({ data: filter === "archived" ? [] : mockDocs })
    );
    await renderTable();
    await waitFor(() =>
      expect(getDocumentsMock).toHaveBeenCalledWith("all", "general"),
    );

    fireEvent.click(screen.getByRole("tab", { name: "Archived" }));
    await waitFor(() =>
      expect(getDocumentsMock).toHaveBeenLastCalledWith("archived", "general"),
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
      expect(getDocumentsMock).toHaveBeenCalledWith("all", "agreement"),
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
    // A-8: Agreements must not get the library Share CTA.
    expect(screen.queryByTestId("document-row-share")).not.toBeInTheDocument();
    expect(screen.queryByTestId("document-share-dialog")).not.toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    expect(await screen.findByRole("img", { name: "Pitch Deck" })).toHaveAttribute(
      "src",
      "https://example.test/page-1.png",
    );
  });
});
