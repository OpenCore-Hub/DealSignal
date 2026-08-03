// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { DealRoomShareDialog } from "./DealRoomShareDialog";
import { createTestI18n } from "@/i18n/test-utils";

vi.mock("@/lib/api", () => ({
  api: {
    getDealRoomLinks: vi.fn(),
    getDealRoomDocuments: vi.fn(),
    getDealRoomFolders: vi.fn(),
    getLinkAccessRules: vi.fn(),
    getLinkById: vi.fn(),
    createDealRoomLink: vi.fn(),
    updateLinkFull: vi.fn(),
    setLinkAccessRules: vi.fn(),
    updateLink: vi.fn(),
    listNDATemplates: vi.fn(),
  },
}));

vi.mock("@/components/links/share", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/components/links/share")>();
  return {
    ...actual,
    ShareTab: () => <div data-testid="share-tab">Share settings</div>,
    DocumentsTab: () => <div data-testid="documents-tab">Scope</div>,
  };
});

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), message: vi.fn() },
}));

async function renderDialog(ui: React.ReactNode) {
  const i18n = await createTestI18n({
    dealRooms: {
      "share.createTitle": "Create share link",
      "share.createLink": "Create link",
      "share.saveLinkSettings": "Save link settings",
      "share.createSuccess": "Created",
      "share.saveSuccess": "Saved",
      "share.active": "Active",
      "share.inactive": "Inactive",
    },
    linkShare: {
      "share.title": "Basic configuration",
      "share.savedButtonLabel": "Saved",
      "documents.title": "Scope",
    },
    common: {
      loading: "Loading...",
      cancel: "Cancel",
      saving: "Saving...",
      "error.saveFailed": "Save failed",
      unsavedChangesTitle: "Unsaved",
      unsavedChangesDescription: "Discard?",
      unsavedChangesConfirm: "Discard",
      confirm: "Confirm",
      disable: "Disable",
    },
  });
  return render(<I18nextProvider i18n={i18n}>{ui}</I18nextProvider>);
}

describe("DealRoomShareDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.getDealRoomLinks).mockResolvedValue({ data: [] });
    vi.mocked(api.getDealRoomDocuments).mockResolvedValue({ data: [] });
    vi.mocked(api.getDealRoomFolders).mockResolvedValue({ data: [] });
    vi.mocked(api.getLinkAccessRules).mockResolvedValue({ data: [] });
    vi.mocked(api.listNDATemplates).mockResolvedValue({ data: [] });
  });

  it("opens in create mode with share settings only (no basic heading or scope section)", async () => {
    await renderDialog(
      <DealRoomShareDialog roomId="room-1">
        <Button>Open</Button>
      </DealRoomShareDialog>
    );

    fireEvent.click(screen.getByText("Open"));

    await waitFor(() => {
      expect(screen.getByText("Create share link")).toBeInTheDocument();
    });
    expect(await screen.findByTestId("share-tab")).toBeInTheDocument();
    expect(screen.queryByText("Basic configuration")).not.toBeInTheDocument();
    expect(screen.queryByTestId("documents-tab")).not.toBeInTheDocument();
    expect(screen.queryByText("Scope")).not.toBeInTheDocument();
  });
});
