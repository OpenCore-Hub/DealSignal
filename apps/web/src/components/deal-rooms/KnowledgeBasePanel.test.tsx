// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { KnowledgeBasePanel } from "./KnowledgeBasePanel";
import { createTestI18n } from "@/i18n/test-utils";
import type { DealRoomDocumentItem, DealRoomFolder, DealRoomKnowledgeBase } from "@/types";

const {
  getDealRoomKnowledgeBaseMock,
  createDealRoomKnowledgeBaseMock,
  rebuildDealRoomKnowledgeBaseMock,
} = vi.hoisted(() => ({
  getDealRoomKnowledgeBaseMock: vi.fn(),
  createDealRoomKnowledgeBaseMock: vi.fn(),
  rebuildDealRoomKnowledgeBaseMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getDealRoomKnowledgeBase: getDealRoomKnowledgeBaseMock,
    createDealRoomKnowledgeBase: createDealRoomKnowledgeBaseMock,
    rebuildDealRoomKnowledgeBase: rebuildDealRoomKnowledgeBaseMock,
  },
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const kbI18n = {
  "knowledgeBase.title": "Knowledge base",
  "knowledgeBase.description":
    "Select documents for Ask Docs. Create or rebuild before enabling Ask Docs on links.",
  "knowledgeBase.status.none": "Not created — create a knowledge base before enabling Ask Docs",
  "knowledgeBase.status.building": "Building {{done}}/{{total}}",
  "knowledgeBase.status.ready": "Ready · {{count}} documents included",
  "knowledgeBase.status.stale": "Out of date — rebuild recommended (Ask Docs still works)",
  "knowledgeBase.status.failed": "Build failed — try creating again",
  "knowledgeBase.create": "Create knowledge base",
  "knowledgeBase.creating": "Creating...",
  "knowledgeBase.rebuild": "Rebuild knowledge base",
  "knowledgeBase.rebuilding": "Rebuilding...",
  "knowledgeBase.selectDocuments": "Documents to include",
  "knowledgeBase.selectFolders": "Folders to include",
  "knowledgeBase.confirmCreate": "Create",
  "knowledgeBase.confirmRebuild": "Rebuild",
  "knowledgeBase.confirmRebuildTitle": "Rebuild knowledge base?",
  "knowledgeBase.confirmRebuildBody":
    "Visitors keep using the previous index until rebuild finishes.",
  "knowledgeBase.cancel": "Cancel",
  "knowledgeBase.forbidden": "Only room owners and admins can manage the knowledge base.",
  "knowledgeBase.loadFailed": "Failed to load knowledge base",
  "knowledgeBase.createFailed": "Failed to create knowledge base",
  "knowledgeBase.rebuildFailed": "Failed to rebuild knowledge base",
  "knowledgeBase.embedFailed":
    "Knowledge base embedding failed. Check the embedding provider configuration and try again.",
  "knowledgeBase.rebuildHint":
    "Visitors keep using the previous index until rebuild finishes.",
};

function makeDoc(overrides: Partial<DealRoomDocumentItem> = {}): DealRoomDocumentItem {
  return {
    id: "rd-1",
    document_id: "doc-1",
    title: "Pitch Deck",
    folder_path: "/general",
    sort_order: 0,
    source_type: "pdf",
    status: "ready",
    created_at: "2026-07-18T10:00:00Z",
    ...overrides,
  };
}

function makeKb(overrides: Partial<DealRoomKnowledgeBase> = {}): DealRoomKnowledgeBase {
  return {
    room_id: "room-1",
    status: "none",
    folder_paths: [],
    document_ids: [],
    embedded_count: 0,
    folder_count: 0,
    ...overrides,
  };
}

async function renderPanel(props: {
  isAdmin?: boolean;
  documents?: DealRoomDocumentItem[];
  folders?: DealRoomFolder[];
}) {
  const i18n = await createTestI18n({ dealRooms: kbI18n });
  return render(
    <I18nextProvider i18n={i18n}>
      <KnowledgeBasePanel
        roomId="room-1"
        isAdmin={props.isAdmin ?? true}
        documents={props.documents ?? [makeDoc()]}
        folders={props.folders ?? []}
      />
    </I18nextProvider>,
  );
}

describe("KnowledgeBasePanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getDealRoomKnowledgeBaseMock.mockResolvedValue(makeKb());
  });

  it("creates a knowledge base with selected folder_paths", async () => {
    createDealRoomKnowledgeBaseMock.mockResolvedValue(
      makeKb({
        status: "ready",
        folder_paths: ["/legal"],
        folder_count: 1,
        embedded_count: 0,
      }),
    );

    await renderPanel({
      documents: [],
      folders: [{ path: "/legal", name: "Legal", sort_order: 0 }],
    });

    expect(
      await screen.findByText(/Not created — create a knowledge base/i),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Create knowledge base/i }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Legal/i }));
    fireEvent.click(screen.getByRole("button", { name: /^Create$/i }));

    await waitFor(() => {
      expect(createDealRoomKnowledgeBaseMock).toHaveBeenCalledWith("room-1", {
        folder_paths: ["/legal"],
        document_ids: [],
      });
    });

    expect(await screen.findByText(/Ready · 0 documents included/i)).toBeInTheDocument();
  });

  it("creates a knowledge base from selected documents and shows ready status", async () => {
    createDealRoomKnowledgeBaseMock.mockResolvedValue(
      makeKb({
        status: "ready",
        document_ids: ["doc-1"],
        embedded_count: 1,
        active_document_ids: ["doc-1"],
      }),
    );

    await renderPanel({});

    expect(
      await screen.findByText(/Not created — create a knowledge base/i),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Create knowledge base/i }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Pitch Deck/i }));
    fireEvent.click(screen.getByRole("button", { name: /^Create$/i }));

    await waitFor(() => {
      expect(createDealRoomKnowledgeBaseMock).toHaveBeenCalledWith("room-1", {
        folder_paths: [],
        document_ids: ["doc-1"],
      });
    });

    expect(await screen.findByText(/Ready · 1 documents included/i)).toBeInTheDocument();
  });

  it("rebuilds the knowledge base with an updated selection", async () => {
    getDealRoomKnowledgeBaseMock.mockResolvedValue(
      makeKb({
        status: "stale",
        document_ids: ["doc-1"],
        embedded_count: 1,
      }),
    );
    rebuildDealRoomKnowledgeBaseMock.mockResolvedValue(
      makeKb({
        status: "ready",
        document_ids: ["doc-1", "doc-2"],
        embedded_count: 2,
      }),
    );

    await renderPanel({
      documents: [
        makeDoc(),
        makeDoc({ id: "rd-2", document_id: "doc-2", title: "Financials" }),
      ],
    });

    expect(await screen.findByText(/Out of date/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Rebuild knowledge base/i }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Financials/i }));
    fireEvent.click(screen.getByRole("button", { name: /^Rebuild$/i }));
    // Confirm dialog before calling API.
    fireEvent.click(screen.getByRole("button", { name: /^Rebuild$/i }));

    await waitFor(() => {
      expect(rebuildDealRoomKnowledgeBaseMock).toHaveBeenCalledWith("room-1", {
        folder_paths: [],
        document_ids: expect.arrayContaining(["doc-1", "doc-2"]),
      });
    });
    expect(await screen.findByText(/Ready · 2 documents included/i)).toBeInTheDocument();
  });

  it("requires rebuild confirmation before calling the API", async () => {
    getDealRoomKnowledgeBaseMock.mockResolvedValue(
      makeKb({
        status: "ready",
        document_ids: ["doc-1"],
        embedded_count: 1,
      }),
    );
    rebuildDealRoomKnowledgeBaseMock.mockResolvedValue(
      makeKb({ status: "ready", embedded_count: 1 }),
    );

    await renderPanel({});
    await screen.findByText(/Ready · 1 documents included/i);

    fireEvent.click(screen.getByRole("button", { name: /Rebuild knowledge base/i }));
    fireEvent.click(screen.getByRole("button", { name: /^Rebuild$/i }));

    expect(rebuildDealRoomKnowledgeBaseMock).not.toHaveBeenCalled();
    expect(screen.getByText(/Rebuild knowledge base\?/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /^Rebuild$/i }));
    await waitFor(() => {
      expect(rebuildDealRoomKnowledgeBaseMock).toHaveBeenCalled();
    });
  });

  it("rebuilds with selected folder_paths", async () => {
    getDealRoomKnowledgeBaseMock.mockResolvedValue(
      makeKb({
        status: "stale",
        folder_paths: ["/general"],
        document_ids: [],
        embedded_count: 0,
        folder_count: 1,
      }),
    );
    rebuildDealRoomKnowledgeBaseMock.mockResolvedValue(
      makeKb({
        status: "ready",
        folder_paths: ["/general", "/legal"],
        folder_count: 2,
        embedded_count: 0,
      }),
    );

    await renderPanel({
      documents: [],
      folders: [
        { path: "/general", name: "General", sort_order: 0 },
        { path: "/legal", name: "Legal", sort_order: 1 },
      ],
    });

    await screen.findByText(/Out of date/i);
    fireEvent.click(screen.getByRole("button", { name: /Rebuild knowledge base/i }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Legal/i }));
    fireEvent.click(screen.getByRole("button", { name: /^Rebuild$/i }));
    fireEvent.click(screen.getByRole("button", { name: /^Rebuild$/i }));

    await waitFor(() => {
      expect(rebuildDealRoomKnowledgeBaseMock).toHaveBeenCalledWith("room-1", {
        folder_paths: expect.arrayContaining(["/general", "/legal"]),
        document_ids: [],
      });
    });
  });

  it("hides create and rebuild actions for non-admins", async () => {
    getDealRoomKnowledgeBaseMock.mockResolvedValue(makeKb({ status: "none" }));
    await renderPanel({ isAdmin: false });

    expect(
      await screen.findByText(/Not created — create a knowledge base/i),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Create knowledge base/i })).not.toBeInTheDocument();
    expect(screen.getByText(/Only room owners and admins/i)).toBeInTheDocument();
  });

  it("shows building progress on the status strip", async () => {
    getDealRoomKnowledgeBaseMock.mockResolvedValue(
      makeKb({
        status: "building",
        embedded_count: 2,
        building_document_ids: ["doc-1", "doc-2", "doc-3"],
      }),
    );
    await renderPanel({});

    expect(await screen.findByText(/Building 2\/3/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Create knowledge base/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Rebuild knowledge base/i })).not.toBeInTheDocument();
  });

  it("toasts a localized message when embedding fails", async () => {
    const { toast } = await import("sonner");
    const { ApiError } = await import("@/lib/apiClient");
    createDealRoomKnowledgeBaseMock.mockRejectedValue(
      new ApiError({
        status: 502,
        code: "knowledge_base_embed_failed",
        message: "knowledge base embedding failed: Invalid URL",
        requestId: "req-1",
      }),
    );

    await renderPanel({});

    fireEvent.click(await screen.findByRole("button", { name: /Create knowledge base/i }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Pitch Deck/i }));
    fireEvent.click(screen.getByRole("button", { name: /^Create$/i }));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(
        "Knowledge base embedding failed. Check the embedding provider configuration and try again.",
      );
    });
  });
});
