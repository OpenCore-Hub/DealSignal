// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { DealRoomShareDialog } from "./DealRoomShareDialog";
import { createTestI18n } from "@/i18n/test-utils";
import enLinkShare from "@/i18n/locales/en/linkShare.json";
import * as shareModule from "@/components/links/share";
import type { Link } from "@/types";

function expandAdvancedAndSelectExperience(
  experience: "host_only" | "ai_supervised" | "ai_self_serve" | "formal",
) {
  fireEvent.click(screen.getByText(/advanced/i));
  fireEvent.click(screen.getByTestId(`visitor-ask-experience-${experience}`));
}

function expandAdvancedAndToggleAiAssistant(enabled: boolean) {
  expandAdvancedAndSelectExperience(enabled ? "ai_supervised" : "host_only");
}

vi.mock("@/lib/api", () => ({
  api: {
    getDealRoomLinks: vi.fn(),
    getDealRoomDocuments: vi.fn(),
    getDealRoomFolders: vi.fn(),
    getDealRoomAccessPolicy: vi.fn(),
    getLinkAccessRules: vi.fn(),
    getLinkById: vi.fn(),
    createDealRoomLink: vi.fn(),
    updateLinkFull: vi.fn(),
    setLinkAccessRules: vi.fn(),
    updateLink: vi.fn(),
    listNDATemplates: vi.fn(),
    getDocuments: vi.fn(),
    getLinkAskPolicy: vi.fn(),
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
    linkShare: enLinkShare,
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
    vi.mocked(api.getDealRoomAccessPolicy).mockResolvedValue({
      data: {
        dealRoomId: "room-1",
        configured: true,
        requireEmail: false,
        requireEmailVerification: true,
        requirePassword: false,
        hasPassword: false,
        requireNda: false,
        watermarkEnabled: true,
        downloadEnabled: false,
        screenshotProtectionEnabled: false,
        fileRequestsEnabled: false,
        indexFileEnabled: false,
        qaEnabled: false,
        allowedEmails: [],
        blockedEmails: [],
      },
    });
    vi.mocked(api.listNDATemplates).mockResolvedValue({ data: [] });
    vi.mocked(api.getDocuments).mockResolvedValue({ data: [] });
    vi.mocked(api.getLinkAccessRules).mockResolvedValue({ data: [] });
    vi.mocked(api.getLinkAskPolicy).mockResolvedValue({
      data: {
        id: "link-1",
        askMode: "supervised",
        askAiEnabled: true,
        askAiMonthlyQuota: null,
        askAiMonthlyUsed: 0,
        askAiMonthlyLimit: 500,
        askAiQuotaExceeded: false,
        askAiEntitled: true,
        formalEntitled: false,
      },
    });
    vi.mocked(api.listNDATemplates).mockResolvedValue({ data: [] });
    vi.mocked(api.getDocuments).mockResolvedValue({ data: [] });
  });

  it("opens in create mode with share settings and document scope", async () => {
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
    expect(await screen.findByTestId("documents-tab")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Scope" })).toBeInTheDocument();
  });

  it("closes create dialog before refetch after successful create", async () => {
    const onOpenChange = vi.fn();
    const validateSpy = vi.spyOn(shareModule, "validateDraft").mockReturnValue({});
    const createdLink = {
      id: "link-new",
      name: "Investor pack",
      shortUrl: "http://localhost/l/abc",
      dealRoomId: "room-1",
      isActive: true,
    } as Link;
    vi.mocked(api.createDealRoomLink).mockResolvedValue(createdLink);

    await renderDialog(
      <DealRoomShareDialog roomId="room-1" open onOpenChange={onOpenChange} />,
    );

    await waitFor(() => {
      expect(screen.getByText("Create share link")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "Create link" }));

    await waitFor(() => {
      expect(api.createDealRoomLink).toHaveBeenCalled();
      expect(onOpenChange).toHaveBeenCalledWith(false);
    });

    validateSpy.mockRestore();
  });

  it("sends ask_ai_enabled false on create when AI assistant is disabled", async () => {
    const validateSpy = vi.spyOn(shareModule, "validateDraft").mockReturnValue({});
    vi.mocked(api.createDealRoomLink).mockResolvedValue({
      id: "link-new",
      name: "No AI",
      shortUrl: "http://localhost/l/abc",
      dealRoomId: "room-1",
      isActive: true,
      qaEnabled: true,
      askAiEnabled: false,
    } as Link);

    await renderDialog(<DealRoomShareDialog roomId="room-1" open />);

    await waitFor(() => {
      expect(screen.getByText("Create share link")).toBeInTheDocument();
    });

    await expandAdvancedAndToggleAiAssistant(false);
    fireEvent.click(screen.getByRole("button", { name: "Create link" }));

    await waitFor(() => {
      expect(api.createDealRoomLink).toHaveBeenCalledWith(
        "room-1",
        expect.objectContaining({ ask_ai_enabled: false }),
      );
    });

    validateSpy.mockRestore();
  });

  it("sends ask_ai_enabled true on create by default for deal-room links", async () => {
    const validateSpy = vi.spyOn(shareModule, "validateDraft").mockReturnValue({});
    vi.mocked(api.createDealRoomLink).mockResolvedValue({
      id: "link-new",
      name: "With AI",
      shortUrl: "http://localhost/l/abc",
      dealRoomId: "room-1",
      isActive: true,
      qaEnabled: true,
      askAiEnabled: true,
    } as Link);

    await renderDialog(<DealRoomShareDialog roomId="room-1" open />);

    await waitFor(() => {
      expect(screen.getByText("Create share link")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "Create link" }));

    await waitFor(() => {
      expect(api.createDealRoomLink).toHaveBeenCalledWith(
        "room-1",
        expect.objectContaining({ ask_ai_enabled: true }),
      );
    });

    validateSpy.mockRestore();
  });

  it("sends ask_ai_enabled false and qa_enabled true on update when AI assistant is disabled", async () => {
    const existingLink = {
      id: "link-1",
      name: "Investor pack",
      shortUrl: "http://localhost/l/abc",
      dealRoomId: "room-1",
      isActive: true,
      qaEnabled: true,
      askAiEnabled: true,
      requireEmailVerification: true,
      watermarkEnabled: true,
    } as Link;
    vi.mocked(api.getDealRoomLinks).mockResolvedValue({ data: [existingLink] });
    vi.mocked(api.getLinkAccessRules).mockResolvedValue({
      data: [{ ruleType: "email", value: "alice@vc.com", action: "allow" }],
    });
    vi.mocked(api.updateLinkFull).mockResolvedValue({ ...existingLink, askAiEnabled: false });
    vi.mocked(api.setLinkAccessRules).mockResolvedValue(undefined);

    await renderDialog(<DealRoomShareDialog roomId="room-1" linkId="link-1" open />);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Save link settings" })).toBeInTheDocument();
    });

    await expandAdvancedAndToggleAiAssistant(false);
    fireEvent.click(screen.getByRole("button", { name: "Save link settings" }));

    await waitFor(() => {
      expect(api.updateLinkFull).toHaveBeenCalledWith(
        "link-1",
        expect.objectContaining({ qa_enabled: true, ask_ai_enabled: false }),
      );
    });
  });
});
