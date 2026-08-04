// @vitest-environment jsdom
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { createTestI18n } from "@/i18n/test-utils";
import { DealRoomDocumentsDialog } from "./DealRoomDocumentsDialog";

vi.mock("@/lib/api", () => ({
  api: {
    createDealRoomFolder: vi.fn(),
    renameDealRoomFolder: vi.fn(),
    deleteDealRoomFolder: vi.fn(),
    updateDealRoomDocument: vi.fn(),
    removeDealRoomDocument: vi.fn(),
    addDealRoomDocument: vi.fn(),
  },
}));

vi.mock("./DocumentPicker", () => ({
  DocumentPicker: () => null,
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("@/stores/uiStore", () => ({
  useUIStore: (selector: (s: { currentWorkspace: { slug: string } }) => unknown) =>
    selector({ currentWorkspace: { slug: "acme-capital" } }),
}));

describe("DealRoomDocumentsDialog open viewer", () => {
  const openSpy = vi.fn();

  beforeEach(() => {
    openSpy.mockReset();
    vi.stubGlobal("open", openSpy);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("opens viewer with roomId for knowledge rail continuity", async () => {
    const i18n = await createTestI18n({
      dealRooms: {
        "detail.manageDocs": "Manage documents",
        "documents.dialogDescription": "Room docs",
        "documents.clickToOpen": "Open",
        "folders.root": "Root",
        "folders.empty": "No folders",
        "folders.lockedBadge": "Locked",
        "folders.searchPlaceholder": "Search",
        "folders.lockFilterAll": "All",
        "folders.lockFilterLocked": "Locked",
        "folders.lockFilterUnlocked": "Unlocked",
      },
      common: { loading: "Loading…" },
    });
    render(
      <I18nextProvider i18n={i18n}>
        <DealRoomDocumentsDialog
          roomId="room-1"
          folders={[{ path: "/legal", name: "Legal", sort_order: 0 }]}
          folderDocs={[
            {
              folder: "/legal",
              permission: "admin",
              documents: [
                {
                  id: "rd-1",
                  document_id: "doc-1",
                  title: "SPA.pdf",
                  folder_path: "/legal",
                  sort_order: 0,
                  source_type: "pdf",
                  status: "ready",
                  created_at: "2026-08-04T00:00:00Z",
                },
              ],
            },
          ]}
          workspaceDocuments={[]}
          onChanged={vi.fn()}
          open
        />
      </I18nextProvider>,
    );

    const docBtn = await screen.findByRole("button", { name: /SPA\.pdf/i });
    fireEvent.click(docBtn);
    expect(openSpy).toHaveBeenCalledWith(
      "/viewer/doc-1?roomId=room-1&ws=acme-capital",
      "_blank",
      "noopener,noreferrer",
    );
  });
});
