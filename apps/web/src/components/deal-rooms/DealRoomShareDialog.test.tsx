// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, within } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import type { Link } from "@/types";
import { DealRoomShareDialog } from "./DealRoomShareDialog";
import enDealRooms from "@/i18n/locales/en/dealRooms.json";
import enLinkShare from "@/i18n/locales/en/linkShare.json";

const i18nInstance = i18n.createInstance();
i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: {
    en: {
      dealRooms: enDealRooms,
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
    getDealRoomLinks: vi.fn(),
    getLinkAccessRules: vi.fn(),
    getLinkInvitations: vi.fn(),
    createDealRoomLink: vi.fn(),
    updateLinkFull: vi.fn(),
    setLinkAccessRules: vi.fn(),
    updateLink: vi.fn(),
    getAccessLogs: vi.fn(),
    inviteLinkViewers: vi.fn(),
    revokeLinkInvitation: vi.fn(),
  },
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/lib/clipboard", () => ({ copyToClipboard: vi.fn(() => Promise.resolve(true)) }));

describe("DealRoomShareDialog", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(api.getDealRoomLinks).mockResolvedValue({ data: [] });
    vi.mocked(api.getLinkAccessRules).mockResolvedValue({ data: [] });
    vi.mocked(api.getLinkInvitations).mockResolvedValue({ data: [] });
    vi.mocked(api.getAccessLogs).mockResolvedValue({ data: [] });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("opens in create mode when no links exist", async () => {
    render(
      <Wrapper>
        <DealRoomShareDialog roomId="room-1">
          <Button>Open</Button>
        </DealRoomShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));

    await waitFor(() => {
      expect(screen.getByText("Create share link")).toBeInTheDocument();
    });
    expect(screen.getByPlaceholderText("Recipient's Organization")).toBeInTheDocument();
  });

  it("renders all three tabs in the correct order", async () => {
    render(
      <Wrapper>
        <DealRoomShareDialog roomId="room-1">
          <Button>Open</Button>
        </DealRoomShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => screen.getByText("Create share link"));

    const tabs = ["Share", "Invite", "Access"];
    tabs.forEach((label) => {
      expect(screen.getByText(label)).toBeInTheDocument();
    });
  });

  it("switches to Access tab", async () => {
    render(
      <Wrapper>
        <DealRoomShareDialog roomId="room-1">
          <Button>Open</Button>
        </DealRoomShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => screen.getByText("Create share link"));

    fireEvent.click(screen.getByText("Access"));

    await waitFor(() => {
      expect(screen.getByText("Require email to view")).toBeInTheDocument();
    });
  });

  it("creates a link and persists access rules", async () => {
    vi.mocked(api.createDealRoomLink).mockResolvedValue({
      id: "link-1",
      name: "Acme DD",
      shortUrl: "http://localhost/l/abc123",
      requireEmail: true,
      requireEmailVerification: false,
      requirePassword: false,
      requireNda: false,
      downloadEnabled: false,
      watermarkEnabled: true,
      aiCopilotEnabled: false,
      folderPaths: [],
    } as unknown as Link);
    vi.mocked(api.setLinkAccessRules).mockResolvedValue(undefined);

    render(
      <Wrapper>
        <DealRoomShareDialog roomId="room-1">
          <Button>Open</Button>
        </DealRoomShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => screen.getByText("Create share link"));

    fireEvent.change(screen.getByPlaceholderText("Recipient's Organization"), {
      target: { value: "Acme DD" },
    });

    const dialog = screen.getByRole("dialog");
    const createButtons = within(dialog).getAllByRole("button", { name: "Create link" });
    fireEvent.click(createButtons[createButtons.length - 1]);

    await waitFor(() => {
      expect(vi.mocked(api.createDealRoomLink)).toHaveBeenCalledWith(
        "room-1",
        expect.objectContaining({ name: "Acme DD" })
      );
    });
    expect(vi.mocked(api.setLinkAccessRules)).toHaveBeenCalledWith("link-1", []);
  });

  it("lists invitations in edit mode", async () => {
    vi.mocked(api.getDealRoomLinks).mockResolvedValue({
      data: [
        {
          id: "link-2",
          name: "Existing link",
          shortUrl: "http://localhost/l/existing",
          requireEmail: true,
          requireEmailVerification: false,
          requirePassword: false,
          requireNda: false,
          downloadEnabled: false,
          watermarkEnabled: false,
          aiCopilotEnabled: false,
          folderPaths: [],
          accessCount: 3,
          heatLevel: "warm",
          isBundle: false,
          documents: [],
          status: "active",
          isActive: true,
          createdAt: new Date().toISOString(),
        } as unknown as Link,
      ],
    });
    vi.mocked(api.getLinkInvitations).mockResolvedValue({
      data: [
        {
          id: "inv-1",
          linkId: "link-2",
          email: "alice@vc.com",
          token: "token-1",
          status: "pending",
          createdAt: new Date().toISOString(),
        },
      ],
    });

    render(
      <Wrapper>
        <DealRoomShareDialog roomId="room-1" linkId="link-2">
          <Button>Open</Button>
        </DealRoomShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => screen.getByText("Existing link"));

    fireEvent.click(screen.getByText("Invite"));

    await waitFor(() => {
      expect(screen.getByText("alice@vc.com")).toBeInTheDocument();
    });
  });
});
