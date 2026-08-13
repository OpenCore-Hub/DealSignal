// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import type { Link } from "@/types";
import { LinkShareDialog } from "./LinkShareDialog";
import enLinkShare from "@/i18n/locales/en/linkShare.json";

const i18nInstance = i18n.createInstance();
i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: {
    en: {
      linkShare: enLinkShare,
      common: {
        cancel: "Cancel",
        saving: "Saving...",
        loading: "Loading...",
        close: "Close",
        error: {
          loadFailed: "Failed to load",
          saveFailed: "Failed to save",
          codes: {
            plan_limit_links:
              "You've reached the share link limit for your plan. Upgrade to create more.",
            plan_feature_watermark:
              "Watermark and viewer protection features are not available on your plan. Upgrade to Pro or higher.",
          },
        },
      },
    },
  },
  interpolation: { escapeValue: false },
});

function Wrapper({ children }: { children: React.ReactNode }) {
  return (
    <MemoryRouter initialEntries={["/acme/links"]}>
      <I18nextProvider i18n={i18nInstance}>{children}</I18nextProvider>
    </MemoryRouter>
  );
}

vi.mock("@/lib/api", () => ({
  api: {
    getContacts: vi.fn(() => Promise.resolve({ data: [] })),
    getLinkById: vi.fn(),
    getLinkAccessRules: vi.fn(),
    updateLinkFull: vi.fn(),
    setLinkAccessRules: vi.fn(),
    updateLink: vi.fn(),
    getAccessLogs: vi.fn(),
    listLinkAsk: vi.fn(),
    listLinkFileRequests: vi.fn(),
    updateFileRequestStatus: vi.fn(),
    listNDATemplates: vi.fn(() => Promise.resolve({ data: [] })),
    getDocuments: vi.fn(() => Promise.resolve({ data: [] })),
    getWorkspaceViewerDomain: vi.fn(),
    getBillingInfo: vi.fn(),
    getDealRoomAccessPolicy: vi.fn(),
    getLinkAskPolicy: vi.fn(),
  },
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/lib/clipboard", () => ({ copyToClipboard: vi.fn(() => Promise.resolve(true)) }));

const baseLink: Link = {
  id: "link-1",
  name: "Acme Corp",
  shortUrl: "http://localhost/l/abc123",
  documentId: "doc-1",
  documentTitle: "Acme Pitch",
  requireEmail: false,
  requireEmailVerification: false,
  requirePassword: false,
  requireNda: false,
  downloadEnabled: false,
  watermarkEnabled: true,
  folderPaths: [],
  accessCount: 5,
  heatLevel: "warm",
  isBundle: false,
  documents: [],
  status: "active",
  isActive: true,
  createdAt: new Date().toISOString(),
} as unknown as Link;

describe("LinkShareDialog", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(api.getLinkById).mockResolvedValue(baseLink);
    vi.mocked(api.getContacts).mockResolvedValue({ data: [] });
    vi.mocked(api.getLinkAccessRules).mockResolvedValue({ data: [] });
    vi.mocked(api.getAccessLogs).mockResolvedValue({ data: [] });
    vi.mocked(api.listLinkAsk).mockResolvedValue({ data: [] });
    vi.mocked(api.listLinkFileRequests).mockResolvedValue({ data: [] });
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

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("opens in edit mode and displays link name", async () => {
    render(
      <Wrapper>
        <LinkShareDialog linkId="link-1">
          <Button>Open</Button>
        </LinkShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));

    await waitFor(() => {
      expect(screen.getByText("Acme Corp")).toBeInTheDocument();
    });
    expect(screen.getByPlaceholderText("Recipient's Organization")).toBeInTheDocument();
  });

  it("switches to Access tab", async () => {
    render(
      <Wrapper>
        <LinkShareDialog linkId="link-1">
          <Button>Open</Button>
        </LinkShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => screen.getByText("Acme Corp"));

    fireEvent.click(screen.getByText("Access control"));

    await waitFor(() => {
      expect(screen.getByText("Require email to view")).toBeInTheDocument();
    });
  });

  it("saves link settings", async () => {
    vi.mocked(api.updateLinkFull).mockResolvedValue(baseLink);
    vi.mocked(api.setLinkAccessRules).mockResolvedValue(undefined);

    render(
      <Wrapper>
        <LinkShareDialog linkId="link-1">
          <Button>Open</Button>
        </LinkShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => screen.getByText("Acme Corp"));

    fireEvent.change(screen.getByPlaceholderText("Recipient's Organization"), {
      target: { value: "Acme Updated" },
    });

    const dialog = screen.getByRole("dialog");
    fireEvent.click(within(dialog).getByText("Save link settings"));

    await waitFor(() => {
      expect(vi.mocked(api.updateLinkFull)).toHaveBeenCalledWith(
        "link-1",
        expect.objectContaining({ name: "Acme Updated" })
      );
    });
  });

  it("echoes existing link settings in Share and Access tabs", async () => {
    const editLink: Link = {
      ...baseLink,
      id: "link-edit",
      name: "Acme Edit",
      requireEmail: true,
      requireEmailVerification: false,
      requirePassword: false,
      requireNda: true,
      ndaDocumentId: "doc-nda",
      watermarkEnabled: true,
      customDomain: "share.example.com",
      notifyOnAccess: true,
      dealRoomId: undefined,
    } as unknown as Link;

    vi.mocked(api.getLinkById).mockResolvedValue(editLink);
    vi.mocked(api.getLinkAccessRules).mockResolvedValue({
      data: [
        { ruleType: "email", value: "alice@vc.com", action: "allow" },
        { ruleType: "email", value: "leaker@bad.com", action: "block" },
      ],
    });

    render(
      <Wrapper>
        <LinkShareDialog linkId="link-edit">
          <Button>Open</Button>
        </LinkShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => {
      expect(screen.getByDisplayValue("Acme Edit")).toBeInTheDocument();
    });

    // Share tab: custom domain should be reflected in the public URL input.
    expect((screen.getByLabelText(/Public link/i) as HTMLInputElement).value).toMatch(
      /share\.example\.com/
    );
    expect(screen.getByRole("switch", { name: /Notify on access/i })).toBeChecked();

    fireEvent.click(screen.getByText("Access control"));
    await waitFor(() => {
      expect(screen.getByText("Require NDA to view")).toBeInTheDocument();
    });

    expect(screen.getByLabelText(/Require email to view/i)).toBeChecked();
    expect(screen.getByLabelText(/Require NDA to view/i)).toBeChecked();
    expect(screen.getByText("alice@vc.com")).toBeInTheDocument();
    expect(screen.getByText("leaker@bad.com")).toBeInTheDocument();
  });

  it("loads existing access rules, shows the restricted email alert, and keeps them after save", async () => {
    const editLink: Link = {
      ...baseLink,
      id: "link-edit",
      name: "Acme Edit",
      requireEmail: true,
      requireEmailVerification: false,
      requirePassword: false,
      requireNda: false,
    } as unknown as Link;

    vi.mocked(api.updateLinkFull).mockResolvedValue(editLink);
    vi.mocked(api.setLinkAccessRules).mockResolvedValue(undefined);
    vi.mocked(api.getLinkById).mockResolvedValue(editLink);
    vi.mocked(api.getLinkAccessRules).mockResolvedValue({
      data: [
        { ruleType: "email", value: "alice@vc.com", action: "allow" },
        { ruleType: "email", value: "leaker@bad.com", action: "block" },
      ],
    });

    render(
      <Wrapper>
        <LinkShareDialog linkId="link-edit">
          <Button>Open</Button>
        </LinkShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => {
      expect(screen.getByDisplayValue("Acme Edit")).toBeInTheDocument();
    });

    // Share tab access summary is expanded by default; verify loaded rules.
    await waitFor(() => {
      expect(screen.getByText("alice@vc.com")).toBeInTheDocument();
    });
    expect(screen.getByText("leaker@bad.com")).toBeInTheDocument();

    // Save should call the API with the existing rules still present.
    const dialog = screen.getByRole("dialog");
    fireEvent.click(within(dialog).getByText("Save link settings"));

    await waitFor(() => {
      expect(vi.mocked(api.setLinkAccessRules)).toHaveBeenCalledWith(
        "link-edit",
        expect.arrayContaining([
          expect.objectContaining({ value: "alice@vc.com", action: "allow" }),
          expect.objectContaining({ value: "leaker@bad.com", action: "block" }),
        ])
      );
    });

    // After save/refetch the rules should still be echoed in the summary.
    expect(screen.getByText("alice@vc.com")).toBeInTheDocument();
    expect(screen.getByText("leaker@bad.com")).toBeInTheDocument();
  });
  it("disables save link settings button when required fields become invalid", async () => {
    render(
      <Wrapper>
        <LinkShareDialog linkId="link-1">
          <Button>Open</Button>
        </LinkShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => screen.getByText("Acme Corp"));

    const saveButton = screen.getByRole("button", { name: "Save link settings" });
    expect(saveButton).toBeEnabled();

    fireEvent.change(screen.getByPlaceholderText("Recipient's Organization"), {
      target: { value: "" },
    });

    expect(saveButton).toBeDisabled();
  });

  it("surfaces plan_limit_links when reactivating a link at quota", async () => {
    const { toast } = await import("sonner");
    const { ApiError } = await import("@/lib/apiClient");
    const inactive: Link = { ...baseLink, isActive: false, status: "revoked" };
    vi.mocked(api.getLinkById).mockResolvedValue(inactive);
    vi.mocked(api.updateLink).mockRejectedValue(
      new ApiError({
        status: 403,
        code: "plan_limit_links",
        message: "share link limit reached for this plan",
        requestId: "r1",
      }),
    );

    render(
      <Wrapper>
        <LinkShareDialog linkId="link-1">
          <Button>Open</Button>
        </LinkShareDialog>
      </Wrapper>,
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => screen.getByText("Acme Corp"));

    const dialog = screen.getByRole("dialog");
    const inactiveLabel = within(dialog).getByText("Inactive");
    const toggle = within(inactiveLabel.parentElement as HTMLElement).getByRole("switch");
    fireEvent.click(toggle);

    await waitFor(() => {
      expect(api.updateLink).toHaveBeenCalledWith("link-1", { status: "active" });
      expect(toast.error).toHaveBeenCalled();
    });
    const [[message]] = vi.mocked(toast.error).mock.calls;
    expect(String(message)).toMatch(/share link limit|Failed to save/i);
  });

  it("surfaces plan_feature_watermark when updateLinkFull races past a stale client gate", async () => {
    const { toast } = await import("sonner");
    const { ApiError } = await import("@/lib/apiClient");
    // Client still thinks watermark is allowed (billing lag); server rejects re-enable.
    vi.mocked(api.getBillingInfo).mockResolvedValue({
      plan: "free",
      period: "monthly",
      trialExpired: false,
      storageUsed: 0,
      storageLimit: 1 << 30,
      linksUsed: 0,
      linksLimit: 20,
      roomsUsed: 0,
      roomsLimit: 1,
      seatsUsed: 1,
      seatsLimit: 1,
      customDomainEnabled: false,
      watermarkEnabled: true,
      ndaEnabled: false,
      visitorAskAiEnabled: false,
    });
    vi.mocked(api.updateLinkFull).mockRejectedValue(
      new ApiError({
        status: 403,
        code: "plan_feature_watermark",
        message: "watermark not available on this plan",
        requestId: "r-wm-race",
      }),
    );

    render(
      <Wrapper>
        <LinkShareDialog linkId="link-1">
          <Button>Open</Button>
        </LinkShareDialog>
      </Wrapper>,
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => screen.getByText("Acme Corp"));

    fireEvent.click(screen.getByRole("button", { name: "Save link settings" }));

    await waitFor(() => {
      expect(api.updateLinkFull).toHaveBeenCalled();
      expect(toast.error).toHaveBeenCalled();
    });
    const [[message]] = vi.mocked(toast.error).mock.calls;
    expect(String(message)).toMatch(/Watermark and viewer protection|Failed to save/i);
  });
});
