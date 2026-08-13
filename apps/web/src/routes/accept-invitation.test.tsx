// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, act } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { AcceptInvitationPage } from "./accept-invitation";
import { ApiError } from "@/lib/apiClient";

const {
  acceptWorkspaceInvitationMock,
  previewWorkspaceInvitationMock,
  getMeMock,
  logoutMock,
} = vi.hoisted(() => ({
  acceptWorkspaceInvitationMock: vi.fn(),
  previewWorkspaceInvitationMock: vi.fn(),
  getMeMock: vi.fn(),
  logoutMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    acceptWorkspaceInvitation: acceptWorkspaceInvitationMock,
    previewWorkspaceInvitation: previewWorkspaceInvitationMock,
    getMe: getMeMock,
    logout: logoutMock,
  },
}));

const resources = {
  en: {
    auth: {
      acceptInvitation: {
        title: "Join workspace",
        loading: "Loading invitation…",
        accepting: "Accepting your invitation…",
        readyUnauthenticated: "Create an account or sign in with {{email}} to join {{workspace}}.",
        workspaceLabel: "Workspace: {{workspace}}",
        emailLabel: "Invited email: {{email}}",
        signedInLabel: "Signed in as: {{email}}",
        successTitle: "You're in",
        success: "Joined {{workspace}}. Redirecting…",
        errorTitle: "Could not join",
        error: "This invitation could not be accepted.",
        expired: "This invitation has expired.",
        used: "This invitation has already been used.",
        emailMismatch:
          "You're signed in with a different email than this invitation. Sign out and continue with the invited email.",
        emailMismatchDetail: "You're signed in as {{signedIn}}, but this invitation is for {{invited}}.",
        planLimitSeats:
          "This workspace has no available team seats. Ask an admin to free a seat or upgrade the plan, then try again.",
        createAccount: "Create account",
        signIn: "Sign in",
        openWorkspace: "Open workspace",
        backHome: "Back to home",
        retry: "Try again",
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
            <Route path="/invitations/:token/accept" element={<AcceptInvitationPage />} />
            <Route path="/login" element={<div>login-page</div>} />
            <Route path="/register" element={<div>register-page</div>} />
            <Route path="/:workspaceSlug/dashboard" element={<div>dashboard-page</div>} />
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
  role: "member",
  status: "pending" as const,
  expiresAt: "2026-12-01T00:00:00Z",
  workspaceId: "ws1",
  workspaceSlug: "kendiyang",
  workspaceName: "kendi yang",
};

describe("AcceptInvitationPage", () => {
  beforeEach(() => {
    acceptWorkspaceInvitationMock.mockReset();
    previewWorkspaceInvitationMock.mockReset();
    getMeMock.mockReset();
    logoutMock.mockReset();
    document.cookie = "auth_session=; Max-Age=0; path=/";
  });

  it("shows create/sign-in actions when unauthenticated", async () => {
    previewWorkspaceInvitationMock.mockResolvedValue(pendingPreview);
    await renderPage("/invitations/tok-1/accept");
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Create account" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Sign in" })).toBeInTheDocument();
    });
    expect(acceptWorkspaceInvitationMock).not.toHaveBeenCalled();
    expect(screen.getByText(/Invited email: guest@example.test/)).toBeInTheDocument();
  });

  it("accepts when signed-in email matches invitation", async () => {
    document.cookie = "auth_session=1; path=/";
    previewWorkspaceInvitationMock.mockResolvedValue(pendingPreview);
    getMeMock.mockResolvedValue({ id: "u1", email: "guest@example.test" });
    acceptWorkspaceInvitationMock.mockResolvedValue({
      userId: "u1",
      role: "member",
      joinedAt: "2026-01-01T00:00:00Z",
      workspaceId: "ws1",
      workspaceSlug: "kendiyang",
      workspaceName: "kendi yang",
    });

    await renderPage("/invitations/tok-1/accept");
    await waitFor(() => {
      expect(acceptWorkspaceInvitationMock).toHaveBeenCalledWith("tok-1");
    });
    await waitFor(() => {
      expect(screen.getByText(/Joined kendi yang/)).toBeInTheDocument();
    });
  });

  it("blocks accept and offers switch account on email mismatch", async () => {
    document.cookie = "auth_session=1; path=/";
    previewWorkspaceInvitationMock.mockResolvedValue(pendingPreview);
    getMeMock.mockResolvedValue({ id: "u-owner", email: "owner@example.test" });
    logoutMock.mockResolvedValue(undefined);

    await renderPage("/invitations/tok-1/accept");
    await waitFor(() => {
      expect(screen.getByText("Could not join")).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Sign in with invited email" })).toBeInTheDocument();
    });
    expect(acceptWorkspaceInvitationMock).not.toHaveBeenCalled();
    expect(screen.getByText(/signed in as owner@example.test/i)).toBeInTheDocument();

    await act(async () => {
      screen.getByRole("button", { name: "Sign in with invited email" }).click();
    });
    await waitFor(() => {
      expect(logoutMock).toHaveBeenCalled();
      expect(screen.getByText("login-page")).toBeInTheDocument();
    });
  });

  it("shows expired state without calling accept", async () => {
    previewWorkspaceInvitationMock.mockResolvedValue({ ...pendingPreview, status: "expired" });
    await renderPage("/invitations/tok-1/accept");
    await waitFor(() => {
      expect(screen.getByText("This invitation has expired.")).toBeInTheDocument();
    });
    expect(acceptWorkspaceInvitationMock).not.toHaveBeenCalled();
  });

  it("surfaces API accept failure without viewer delivery-email copy", async () => {
    document.cookie = "auth_session=1; path=/";
    previewWorkspaceInvitationMock.mockResolvedValue(pendingPreview);
    getMeMock.mockResolvedValue({ id: "u1", email: "guest@example.test" });
    acceptWorkspaceInvitationMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "invitation_email_mismatch",
        message: "signed-in email does not match this workspace invitation",
        requestId: "r1",
      }),
    );

    await renderPage("/invitations/tok-1/accept");
    await waitFor(() => {
      expect(screen.getByText("Could not join")).toBeInTheDocument();
    });
    expect(screen.queryByText(/reserved delivery email/i)).not.toBeInTheDocument();
  });

  it("surfaces plan_limit_seats with invitee-facing copy", async () => {
    document.cookie = "auth_session=1; path=/";
    previewWorkspaceInvitationMock.mockResolvedValue(pendingPreview);
    getMeMock.mockResolvedValue({ id: "u1", email: "guest@example.test" });
    acceptWorkspaceInvitationMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "plan_limit_seats",
        message: "internal seat limit reached for this plan",
        requestId: "r1",
      }),
    );

    await renderPage("/invitations/tok-1/accept");
    await waitFor(() => {
      expect(
        screen.getByText(
          "This workspace has no available team seats. Ask an admin to free a seat or upgrade the plan, then try again.",
        ),
      ).toBeInTheDocument();
    });
    expect(screen.queryByText(/invite more members/i)).not.toBeInTheDocument();
  });
});
