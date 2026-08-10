// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, within } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { useDocumentColumns, type DocumentRow } from "./DocumentsColumns";
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
} from "@tanstack/react-table";

const {
  getDocumentDownloadUrlMock,
  archiveDocumentMock,
  unarchiveDocumentMock,
  deleteOnDelete,
  archiveOnArchive,
} = vi.hoisted(() => ({
  getDocumentDownloadUrlMock: vi.fn(),
  archiveDocumentMock: vi.fn(),
  unarchiveDocumentMock: vi.fn(),
  deleteOnDelete: vi.fn(),
  archiveOnArchive: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getDocumentDownloadUrl: getDocumentDownloadUrlMock,
    archiveDocument: archiveDocumentMock,
    unarchiveDocument: unarchiveDocumentMock,
  },
}));

vi.mock("@/lib/formatters", () => ({
  formatFileSize: vi.fn(() => "1 MB"),
  formatDate: vi.fn(() => "Jun 20, 2026"),
}));

vi.mock("@/lib/clipboard", () => ({
  copyToClipboard: vi.fn(),
}));

const readyDoc: DocumentRow = {
  id: "doc_1",
  title: "Pitch Deck.pdf",
  sourceType: "pdf",
  fileName: "Pitch Deck.pdf",
  fileType: "pdf",
  fileSize: 1_000_000,
  pageCount: 10,
  status: "ready",
  createdAt: "2026-06-20T10:00:00Z",
  updatedAt: "2026-06-20T10:00:00Z",
  links: [],
  totalViews: 0,
  heatLevel: "cold",
};

async function initI18n() {
  const instance = i18n.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["documents", "common"],
    defaultNS: "documents",
    resources: {
      en: {
        documents: {
          columns: {
            file: "File",
            heat: "Heat",
            views: "Views",
            status: "Status",
            shareLinks: "Links",
            actions: "Actions",
            pages: "{{count}} pages",
            links: "{{count}} links",
            downloadNotReady: "Document is not ready for download",
            downloadFailed: "Failed to download document",
            deleteBusy: "Busy",
            archiveDisabled: "Only ready documents can be archived",
            archivedActionDisabled: "Unarchive the document to use this action",
          },
          status: {
            ready: "Ready",
            uploading: "Uploading",
            processing: "Processing",
            failed: "Failed",
            archived: "Archived",
            pending: "Pending",
          },
        },
        common: {
          moreActions: "More actions",
          preview: "Preview",
          view: "View",
          createLink: "Create Link",
          copyLink: "Copy Link",
          addToDealRoom: "Add to Deal Room",
          archive: "Archive",
          unarchive: "Unarchive",
          download: "Download",
          delete: "Delete",
        },
      },
    },
    interpolation: { escapeValue: false },
  });
  return instance;
}

function ActionsHarness({ doc }: { doc: DocumentRow }) {
  const navigate = vi.fn();
  const columns = useDocumentColumns({
    workspaceSlug: "acme",
    navigate,
    onArchive: archiveOnArchive,
    onDelete: deleteOnDelete,
  });
  const table = useReactTable({
    data: [doc],
    columns,
    getCoreRowModel: getCoreRowModel(),
  });
  const actionsCell = table.getRowModel().rows[0]!.getVisibleCells().find((c) => c.column.id === "actions");
  return <div>{actionsCell ? flexRender(actionsCell.column.columnDef.cell, actionsCell.getContext()) : null}</div>;
}

describe("useDocumentColumns download/delete", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getDocumentDownloadUrlMock.mockResolvedValue({
      download_url: "https://example.com/file.pdf",
      expires_at: "2026-06-20T10:15:00Z",
      filename: "Pitch Deck.pdf",
      content_type: "application/pdf",
    });
  });

  it("requests archive confirmation instead of archiving immediately", async () => {
    const i18nInstance = await initI18n();
    render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter>
          <ActionsHarness doc={readyDoc} />
        </MemoryRouter>
      </I18nextProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "More actions" }));
    const menu = await screen.findByRole("menu");
    fireEvent.click(within(menu).getByRole("menuitem", { name: /^Archive$/i }));
    expect(archiveOnArchive).toHaveBeenCalledWith(expect.objectContaining({ id: "doc_1" }));
    expect(archiveDocumentMock).not.toHaveBeenCalled();
  });

  it("downloads via signed URL and requests delete confirmation", async () => {
    const clickSpy = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => undefined);
    const i18nInstance = await initI18n();
    render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter>
          <ActionsHarness doc={readyDoc} />
        </MemoryRouter>
      </I18nextProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "More actions" }));
    const menu = await screen.findByRole("menu");
    fireEvent.click(within(menu).getByRole("menuitem", { name: /Download/i }));

    await waitFor(() => {
      expect(getDocumentDownloadUrlMock).toHaveBeenCalledWith("doc_1");
    });
    expect(clickSpy).toHaveBeenCalled();
    clickSpy.mockRestore();

    fireEvent.click(screen.getByRole("button", { name: "More actions" }));
    const menu2 = await screen.findByRole("menu");
    fireEvent.click(within(menu2).getByRole("menuitem", { name: /Delete/i }));
    expect(deleteOnDelete).toHaveBeenCalledWith(expect.objectContaining({ id: "doc_1" }));
  });

  it("shows Unarchive for archived documents and disables share actions", async () => {
    const i18nInstance = await initI18n();
    const archived: DocumentRow = {
      ...readyDoc,
      status: "archived",
    };
    function ArchivedActionsHarness() {
      const navigate = vi.fn();
      const columns = useDocumentColumns({
        workspaceSlug: "acme",
        navigate,
        onDelete: deleteOnDelete,
        onAddToDealRoom: vi.fn(),
      });
      const table = useReactTable({
        data: [archived],
        columns,
        getCoreRowModel: getCoreRowModel(),
      });
      const actionsCell = table
        .getRowModel()
        .rows[0]!
        .getVisibleCells()
        .find((c) => c.column.id === "actions");
      return (
        <div>
          {actionsCell ? flexRender(actionsCell.column.columnDef.cell, actionsCell.getContext()) : null}
        </div>
      );
    }
    render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter>
          <ArchivedActionsHarness />
        </MemoryRouter>
      </I18nextProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "More actions" }));
    const menu = await screen.findByRole("menu");
    expect(within(menu).getByRole("menuitem", { name: /^Unarchive$/i })).toBeInTheDocument();
    expect(within(menu).queryByRole("menuitem", { name: /^Archive$/i })).not.toBeInTheDocument();
    expect(within(menu).queryByRole("menuitem", { name: /Copy link/i })).not.toBeInTheDocument();
    expect(within(menu).getByRole("menuitem", { name: /Create link/i })).toHaveAttribute(
      "data-disabled",
    );
    expect(within(menu).getByRole("menuitem", { name: /Add to Deal Room/i })).toHaveAttribute(
      "data-disabled",
    );
    expect(within(menu).getByRole("menuitem", { name: /Download/i })).toHaveAttribute(
      "data-disabled",
    );
    expect(within(menu).getByRole("menuitem", { name: /^Unarchive$/i })).not.toHaveAttribute(
      "data-disabled",
    );
    expect(within(menu).getByRole("menuitem", { name: /Delete/i })).not.toHaveAttribute(
      "data-disabled",
    );
  });

  it("disables download while processing and keeps delete available after ready", async () => {
    const i18nInstance = await initI18n();
    const processing: DocumentRow = { ...readyDoc, status: "processing" };
    render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter>
          <ActionsHarness doc={processing} />
        </MemoryRouter>
      </I18nextProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "More actions" }));
    const menu = await screen.findByRole("menu");
    expect(within(menu).getByRole("menuitem", { name: /Download/i })).toHaveAttribute(
      "data-disabled",
    );
    expect(within(menu).getByRole("menuitem", { name: /Delete/i })).toHaveAttribute(
      "data-disabled",
    );
  });

  it("hides add-to-deal-room for agreement and deal_room categories", async () => {
    const i18nInstance = await initI18n();
    const onAddToDealRoom = vi.fn();
    function CategoryHarness({ doc }: { doc: DocumentRow }) {
      const navigate = vi.fn();
      const columns = useDocumentColumns({
        workspaceSlug: "acme",
        navigate,
        onAddToDealRoom,
      });
      const table = useReactTable({
        data: [doc],
        columns,
        getCoreRowModel: getCoreRowModel(),
      });
      const actionsCell = table.getRowModel().rows[0]!.getVisibleCells().find((c) => c.column.id === "actions");
      return <div>{actionsCell ? flexRender(actionsCell.column.columnDef.cell, actionsCell.getContext()) : null}</div>;
    }

    for (const category of ["agreement", "deal_room"] as const) {
      onAddToDealRoom.mockClear();
      const { unmount } = render(
        <I18nextProvider i18n={i18nInstance}>
          <MemoryRouter>
            <CategoryHarness doc={{ ...readyDoc, category }} />
          </MemoryRouter>
        </I18nextProvider>,
      );
      fireEvent.click(screen.getByRole("button", { name: "More actions" }));
      const menu = await screen.findByRole("menu");
      expect(within(menu).queryByRole("menuitem", { name: /Add to Deal Room/i })).not.toBeInTheDocument();
      unmount();
    }

    render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter>
          <CategoryHarness doc={{ ...readyDoc, category: "general" }} />
        </MemoryRouter>
      </I18nextProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: "More actions" }));
    const generalMenu = await screen.findByRole("menu");
    expect(within(generalMenu).getByRole("menuitem", { name: /Add to Deal Room/i })).toBeInTheDocument();
  });
});
