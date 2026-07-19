// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, within } from "@testing-library/react";
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
        error: { loadFailed: "Failed to load", saveFailed: "Failed to save" },
      },
    },
  },
  interpolation: { escapeValue: false },
});

function Wrapper({ children }: { children: React.ReactNode }) {
  return <I18nextProvider i18n={i18nInstance}>{children}</I18nextProvider>;
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
    listLinkQuestions: vi.fn(),
    listLinkFileRequests: vi.fn(),
    answerQuestion: vi.fn(),
    updateFileRequestStatus: vi.fn(),
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
  aiCopilotEnabled: false,
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
    vi.mocked(api.listLinkQuestions).mockResolvedValue({ data: [] });
    vi.mocked(api.listLinkFileRequests).mockResolvedValue({ data: [] });
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

    fireEvent.click(screen.getByText("Access"));

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

    // Share tab: custom domain should be reflected in the public URL.
    expect(screen.getByText(/share\.example\.com/)).toBeInTheDocument();
    expect(screen.getByRole("switch", { name: /Notify on access/i })).toBeChecked();

    fireEvent.click(screen.getByText("Access"));
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

    // The top alert should use the updated email-only copy.
    expect(
      screen.getByText("This link is restricted. Only allowed emails can access.")
    ).toBeInTheDocument();

    // Expand the Share tab access summary to verify loaded rules.
    fireEvent.click(screen.getByText("Access summary"));
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

  it("selects custom preset from the dropdown", async () => {
    const publicLink: Link = {
      ...baseLink,
      name: "Public Link",
      requireEmail: false,
      requireEmailVerification: false,
      requirePassword: false,
      watermarkEnabled: false,
      requireNda: false,
      downloadEnabled: false,
      screenshotProtectionEnabled: false,
      aiCopilotEnabled: false,
      fileRequestsEnabled: false,
      indexFileEnabled: false,
      qaEnabled: false,
    } as unknown as Link;

    vi.mocked(api.getLinkById).mockResolvedValue(publicLink);

    render(
      <Wrapper>
        <LinkShareDialog linkId="link-1">
          <Button>Open</Button>
        </LinkShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => screen.getByText("Public Link"));

    const trigger = screen.getByRole("combobox", { name: /link preset/i });
    expect(trigger).toHaveTextContent("Public");

    fireEvent.pointerDown(trigger);
    fireEvent.click(trigger);
    const option = await waitFor(() => screen.getByRole("option", { name: /Custom/i }));
    fireEvent.pointerDown(option);
    fireEvent.click(option);

    expect(trigger).toHaveTextContent("Custom");
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
});
