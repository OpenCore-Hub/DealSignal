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
    getLinkById: vi.fn(),
    getLinkAccessRules: vi.fn(),
    getLinkInvitations: vi.fn(),
    updateLinkFull: vi.fn(),
    setLinkAccessRules: vi.fn(),
    updateLink: vi.fn(),
    getAccessLogs: vi.fn(),
    listLinkQuestions: vi.fn(),
    listLinkFileRequests: vi.fn(),
    answerQuestion: vi.fn(),
    updateFileRequestStatus: vi.fn(),
    inviteLinkViewers: vi.fn(),
    revokeLinkInvitation: vi.fn(),
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
  requireEmail: true,
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
    vi.mocked(api.getLinkAccessRules).mockResolvedValue({ data: [] });
    vi.mocked(api.getLinkInvitations).mockResolvedValue({ data: [] });
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
    expect(vi.mocked(api.setLinkAccessRules)).toHaveBeenCalledWith("link-1", []);
  });
});
