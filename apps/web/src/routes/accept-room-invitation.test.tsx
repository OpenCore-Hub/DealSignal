// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, act } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { AcceptRoomInvitationPage } from "./accept-room-invitation";

const {
  acceptDealRoomInvitationMock,
  previewDealRoomInvitationMock,
  getMeMock,
  logoutMock,
} = vi.hoisted(() => ({
  acceptDealRoomInvitationMock: vi.fn(),
  previewDealRoomInvitationMock: vi.fn(),
  getMeMock: vi.fn(),
  logoutMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    acceptDealRoomInvitation: acceptDealRoomInvitationMock,
    previewDealRoomInvitation: previewDealRoomInvitationMock,
    getMe: getMeMock,
    logout: logoutMock,
  },
}));

const resources = {
  en: {
    auth: {
      acceptRoomInvitation: {
        title: "Join data room",
        loading: "Loading invitation…",
        accepting: "Accepting your invitation…",
        readyUnauthenticated: "Create an account or sign in with {{email}} to join {{room}} in {{workspace}}.",
        workspaceLabel: "Workspace: {{workspace}}",
        roomLabel: "Data room: {{room}}",
        emailLabel: "Invited email: {{email}}",
        signedInLabel: "Signed in as: {{email}}",
        successTitle: "You're in",
        success: "Joined {{room}}. Redirecting…",
        errorTitle: "Could not join",
        error: "This invitation could not be accepted.",
        used: "This invitation has already been used.",
        emailMismatch:
          "You're signed in with a different email than this invitation. Sign out and continue with the invited email.",
        emailMismatchDetail: "You're signed in as {{signedIn}}, but this invitation is for {{invited}}.",
        createAccount: "Create account",
        signIn: "Sign in",
        openRoom: "Open data room",
        backHome: "Back to home",
        switchAccount: "Sign in with invited email",
      },
    },
    common: {
      error: {
        loadFailed: "Failed to load",
        codes: {
          invitation_not_found: "Invitation not found.",
          invitation_email_mismatch:
            "You're signed in with a different email than this invitation.",
        },
      },
    },
  },
};

async function initI18n() {
  const instance = i18n.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["auth", "common"],
    defaultNS: "auth",
    resources,
    interpolation: { escapeValue: false },
  });
  return instance;
}

async function renderPage(path: string) {
  const i18nInstance = await initI18n();
  let result!: ReturnType<typeof render>;
  await act(async () => {
    result = render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter initialEntries={[path]}>
          <Routes>
            <Route path="/room-invitations/:token/accept" element={<AcceptRoomInvitationPage />} />
            <Route path="/login" element={<div>login-page</div>} />
            <Route path="/register" element={<div>register-page</div>} />
            <Route path="/:workspaceSlug/deal-rooms/:roomId" element={<div>room-page</div>} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>,
    );
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  return result;
}

const pendingPreview = {
  email: "guest@example.test",
  role: "guest",
  status: "pending" as const,
  workspaceId: "ws1",
  workspaceSlug: "kendiyang",
  workspaceName: "kendi yang",
  roomId: "room-1",
  roomName: "Series A",
};

describe("AcceptRoomInvitationPage", () => {
  beforeEach(() => {
    acceptDealRoomInvitationMock.mockReset();
    previewDealRoomInvitationMock.mockReset();
    getMeMock.mockReset();
    logoutMock.mockReset();
    document.cookie = "auth_session=; Max-Age=0; path=/";
  });

  it("shows create/sign-in actions when unauthenticated", async () => {
    previewDealRoomInvitationMock.mockResolvedValue(pendingPreview);
    await renderPage("/room-invitations/dsr1.tok/accept");
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Create account" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Sign in" })).toBeInTheDocument();
    });
    expect(acceptDealRoomInvitationMock).not.toHaveBeenCalled();
    expect(screen.getByText(/Invited email: guest@example.test/)).toBeInTheDocument();
    expect(screen.getByText(/Data room: Series A/)).toBeInTheDocument();
  });

  it("accepts when signed-in email matches invitation", async () => {
    document.cookie = "auth_session=1; path=/";
    previewDealRoomInvitationMock.mockResolvedValue(pendingPreview);
    getMeMock.mockResolvedValue({ id: "u1", email: "guest@example.test" });
    acceptDealRoomInvitationMock.mockResolvedValue({
      userId: "u1",
      role: "guest",
      workspaceId: "ws1",
      workspaceSlug: "kendiyang",
      workspaceName: "kendi yang",
      roomId: "room-1",
      roomName: "Series A",
      memberStatus: "active",
    });

    await renderPage("/room-invitations/dsr1.tok/accept");
    await waitFor(() => {
      expect(acceptDealRoomInvitationMock).toHaveBeenCalledWith("dsr1.tok");
    });
    await waitFor(() => {
      expect(screen.getByText(/Joined Series A/)).toBeInTheDocument();
    });
  });

  it("blocks accept and offers switch account on email mismatch", async () => {
    document.cookie = "auth_session=1; path=/";
    previewDealRoomInvitationMock.mockResolvedValue(pendingPreview);
    getMeMock.mockResolvedValue({ id: "u-owner", email: "owner@example.test" });
    logoutMock.mockResolvedValue(undefined);

    await renderPage("/room-invitations/dsr1.tok/accept");
    await waitFor(() => {
      expect(screen.getByText("Could not join")).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Sign in with invited email" })).toBeInTheDocument();
    });
    expect(acceptDealRoomInvitationMock).not.toHaveBeenCalled();
    expect(screen.getByText(/signed in as owner@example.test/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Try again" })).not.toBeInTheDocument();

    await act(async () => {
      screen.getByRole("button", { name: "Sign in with invited email" }).click();
    });
    await waitFor(() => {
      expect(logoutMock).toHaveBeenCalled();
      expect(screen.getByText("login-page")).toBeInTheDocument();
    });
  });

  it("surfaces used invitations without opening the room", async () => {
    previewDealRoomInvitationMock.mockResolvedValue({ ...pendingPreview, status: "used" });
    await renderPage("/room-invitations/dsr1.tok/accept");
    await waitFor(() => {
      expect(screen.getByText("This invitation has already been used.")).toBeInTheDocument();
    });
    expect(acceptDealRoomInvitationMock).not.toHaveBeenCalled();
  });
});
