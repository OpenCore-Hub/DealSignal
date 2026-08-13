// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { MemoryRouter } from "react-router";
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
    getWorkspaceViewerDomain: vi.fn(),
    getBillingInfo: vi.fn(),
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
      "share.linkLimitReached":
        "You've reached the share link limit for your plan. Upgrade to create more.",
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
      "error.codes.plan_limit_links":
        "You've reached the share link limit for your plan. Upgrade to create more.",
      "error.codes.plan_feature_watermark":
        "Watermark and viewer protection features are not available on your plan. Upgrade to Pro or higher.",
      "error.codes.plan_feature_nda":
        "NDA requirements are not available on your plan. Upgrade to Business or higher.",
      unsavedChangesTitle: "Unsaved",
      unsavedChangesDescription: "Discard?",
      unsavedChangesConfirm: "Discard",
      confirm: "Confirm",
      disable: "Disable",
    },
  });
  return render(
    <MemoryRouter initialEntries={["/acme/deal-rooms/room-1"]}>
      <I18nextProvider i18n={i18n}>{ui}</I18nextProvider>
    </MemoryRouter>,
  );
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
    vi.mocked(api.getWorkspaceViewerDomain).mockResolvedValue({
      hostname: "",
      status: "",
      cnameHost: "",
      cnameTarget: "",
    });
    vi.mocked(api.getBillingInfo).mockResolvedValue({
      plan: "trial",
      period: "monthly",
      trialExpired: false,
      storageUsed: 0,
      storageLimit: 10,
      linksUsed: 0,
      linksLimit: 50,
      roomsUsed: 0,
      roomsLimit: 10,
      seatsUsed: 1,
      seatsLimit: 5,
      customDomainEnabled: true,
      watermarkEnabled: true,
      ndaEnabled: true,
      visitorAskAiEnabled: true,
    });
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

  it("sends ask_ai_enabled false on create when plan lacks visitor Ask AI", async () => {
    const validateSpy = vi.spyOn(shareModule, "validateDraft").mockReturnValue({});
    vi.mocked(api.getBillingInfo).mockResolvedValue({
      plan: "free",
      period: "monthly",
      trialExpired: false,
      storageUsed: 0,
      storageLimit: 1,
      linksUsed: 0,
      linksLimit: 3,
      roomsUsed: 0,
      roomsLimit: 1,
      seatsUsed: 1,
      seatsLimit: 1,
      customDomainEnabled: false,
      watermarkEnabled: false,
      ndaEnabled: false,
      visitorAskAiEnabled: false,
    });
    vi.mocked(api.createDealRoomLink).mockResolvedValue({
      id: "link-new",
      name: "Free plan",
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

    fireEvent.click(screen.getByRole("button", { name: "Create link" }));

    await waitFor(() => {
      expect(api.createDealRoomLink).toHaveBeenCalledWith(
        "room-1",
        expect.objectContaining({
          ask_ai_enabled: false,
          watermark_enabled: false,
          screenshot_protection_enabled: false,
          require_nda: false,
        }),
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

  it("blocks create when share link quota is exhausted", async () => {
    vi.mocked(api.getBillingInfo).mockResolvedValue({
      plan: "free",
      period: "monthly",
      trialExpired: false,
      storageUsed: 0,
      storageLimit: 1,
      linksUsed: 20,
      linksLimit: 20,
      roomsUsed: 1,
      roomsLimit: 1,
      seatsUsed: 1,
      seatsLimit: 1,
      customDomainEnabled: false,
      watermarkEnabled: false,
      ndaEnabled: false,
      visitorAskAiEnabled: false,
    });

    await renderDialog(<DealRoomShareDialog roomId="room-1" open />);

    await waitFor(() => {
      expect(screen.getByTestId("share-link-limit-hint")).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: "Create link" })).toBeDisabled();
  });

  it("surfaces plan_limit_links when create races past a stale client quota gate", async () => {
    const { toast } = await import("sonner");
    const { ApiError } = await import("@/lib/apiClient");
    const validateSpy = vi.spyOn(shareModule, "validateDraft").mockReturnValue({});
    vi.mocked(api.getBillingInfo).mockResolvedValue({
      plan: "free",
      period: "monthly",
      trialExpired: false,
      storageUsed: 0,
      storageLimit: 1,
      linksUsed: 19,
      linksLimit: 20,
      roomsUsed: 1,
      roomsLimit: 1,
      seatsUsed: 1,
      seatsLimit: 1,
      customDomainEnabled: false,
      watermarkEnabled: false,
      ndaEnabled: false,
      visitorAskAiEnabled: false,
    });
    vi.mocked(api.createDealRoomLink).mockRejectedValue(
      new ApiError({
        status: 403,
        code: "plan_limit_links",
        message: "share link limit reached for this plan",
        requestId: "r-race",
      }),
    );

    await renderDialog(<DealRoomShareDialog roomId="room-1" open />);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Create link" })).toBeEnabled();
    });
    fireEvent.click(screen.getByRole("button", { name: "Create link" }));

    await waitFor(() => {
      expect(api.createDealRoomLink).toHaveBeenCalled();
      expect(toast.error).toHaveBeenCalled();
    });
    const [[message]] = vi.mocked(toast.error).mock.calls;
    expect(String(message)).toMatch(/share link limit/i);

    validateSpy.mockRestore();
  });

  it("surfaces plan_feature_watermark when create races past a stale client feature gate", async () => {
    const { toast } = await import("sonner");
    const { ApiError } = await import("@/lib/apiClient");
    const validateSpy = vi.spyOn(shareModule, "validateDraft").mockReturnValue({});
    // Stale client billing still thinks watermark is allowed.
    vi.mocked(api.getBillingInfo).mockResolvedValue({
      plan: "free",
      period: "monthly",
      trialExpired: false,
      storageUsed: 0,
      storageLimit: 1,
      linksUsed: 0,
      linksLimit: 20,
      roomsUsed: 1,
      roomsLimit: 1,
      seatsUsed: 1,
      seatsLimit: 1,
      customDomainEnabled: false,
      watermarkEnabled: true,
      ndaEnabled: false,
      visitorAskAiEnabled: false,
    });
    vi.mocked(api.createDealRoomLink).mockRejectedValue(
      new ApiError({
        status: 403,
        code: "plan_feature_watermark",
        message: "watermark not available on this plan",
        requestId: "r-wm-race",
      }),
    );

    await renderDialog(<DealRoomShareDialog roomId="room-1" open />);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Create link" })).toBeEnabled();
    });
    fireEvent.click(screen.getByRole("button", { name: "Create link" }));

    await waitFor(() => {
      expect(api.createDealRoomLink).toHaveBeenCalled();
      expect(toast.error).toHaveBeenCalled();
    });
    const [[message]] = vi.mocked(toast.error).mock.calls;
    expect(String(message)).toMatch(/Watermark and viewer protection|Save failed|Failed to save/i);

    validateSpy.mockRestore();
  });
});
