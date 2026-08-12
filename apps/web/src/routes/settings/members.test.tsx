// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { SettingsMembersPage } from "./members";
import { toast } from "sonner";
import { ApiError } from "@/lib/apiClient";
import { getCachedAccountEmail } from "@/lib/authAccount";

const {
  getWorkspaceMembersMock,
  inviteWorkspaceMemberMock,
  updateWorkspaceInvitationRoleMock,
  revokeWorkspaceInvitationMock,
  updateWorkspaceMemberRoleMock,
  removeWorkspaceMemberMock,
} = vi.hoisted(() => ({
  getWorkspaceMembersMock: vi.fn(),
  inviteWorkspaceMemberMock: vi.fn(),
  updateWorkspaceInvitationRoleMock: vi.fn(),
  revokeWorkspaceInvitationMock: vi.fn(),
  updateWorkspaceMemberRoleMock: vi.fn(),
  removeWorkspaceMemberMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getWorkspaceMembers: getWorkspaceMembersMock,
    inviteWorkspaceMember: inviteWorkspaceMemberMock,
    updateWorkspaceInvitationRole: updateWorkspaceInvitationRoleMock,
    revokeWorkspaceInvitation: revokeWorkspaceInvitationMock,
    updateWorkspaceMemberRole: updateWorkspaceMemberRoleMock,
    removeWorkspaceMember: removeWorkspaceMemberMock,
  },
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

vi.mock("@/lib/authAccount", () => ({
  getCachedAccountEmail: vi.fn(() => undefined),
}));

const settingsResources = {
  en: {
    settings: {
      members: {
        title: "Members",
        emailPlaceholder: "Email address",
        roleLabel: "Role",
        invite: "Invite",
        inviting: "Inviting...",
        invited: "Invitation sent to {{email}}",
        inviteFailed: "Could not send invitation. Please try again.",
        invalidEmail: "Please enter a valid email address",
        loadFailed: "Failed to load members",
        retry: "Retry",
        editRole: "Edit role",
        editRoleTitle: "Edit role",
        editRoleDescription: "Choose a new role for {{email}}.",
        saveRole: "Save role",
        savingRole: "Saving...",
        roleUpdated: "Role updated for {{email}}",
        roleUpdateFailed: "Could not update role. Please try again.",
        remove: "Remove",
        removeTitle: "Remove member",
        removeDescription: "Remove {{email}} from this workspace?",
        removePendingDescription: "Revoke the invitation sent to {{email}}?",
        removing: "Removing...",
        removed: "{{email}} was removed",
        inviteRevoked: "Invitation to {{email}} was revoked",
        removeFailed: "Could not remove this person. Please try again.",
        cannotModifyOwner: "The workspace owner cannot be changed here",
        cannotModifySelf: "You cannot change your own membership here",
        cannotManageMember: "You do not have permission to manage this member",
        status: { pending: "Pending", active: "Active", suspended: "Suspended" },
        roles: { owner: "Owner", admin: "Admin", member: "Member", guest: "Guest" },
      },
    },
    common: {
      cancel: "Cancel",
      save: "Save",
      delete: "Delete",
      moreActions: "More actions",
      error: {
        saveFailed: "Failed to save",
        deleteFailed: "Failed to delete",
        codes: {
          already_member: "This person is already a member.",
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
    ns: ["settings", "common"],
    defaultNS: "settings",
    resources: settingsResources,
    interpolation: { escapeValue: false },
  });
  return instance;
}

async function renderPage() {
  const i18nInstance = await initI18n();
  let result!: ReturnType<typeof render>;
  await act(async () => {
    result = render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter initialEntries={["/acme/settings/members"]}>
          <Routes>
            <Route path="/:workspaceSlug/settings/members" element={<SettingsMembersPage />} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>,
    );
  });
  return result;
}

describe("SettingsMembersPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(getCachedAccountEmail).mockReturnValue(undefined);
    getWorkspaceMembersMock.mockResolvedValue({
      data: [
        {
          id: "m_1",
          userId: "u_1",
          email: "owner@acme.com",
          name: "Owner",
          role: "owner",
          joinedAt: "2026-01-01T00:00:00Z",
          status: "active",
        },
      ],
    });
    inviteWorkspaceMemberMock.mockResolvedValue({
      data: {
        id: "inv_1",
        email: "new@example.com",
        role: "member",
        status: "pending",
        expiresAt: "2026-01-08T00:00:00Z",
        createdAt: "2026-01-01T00:00:00Z",
      },
    });
    updateWorkspaceInvitationRoleMock.mockResolvedValue({ data: { token: "inv_pending", role: "guest" } });
    revokeWorkspaceInvitationMock.mockResolvedValue(undefined);
    updateWorkspaceMemberRoleMock.mockResolvedValue({ data: { userId: "u_2", role: "admin" } });
    removeWorkspaceMemberMock.mockResolvedValue(undefined);
  });

  it("lists members and invites with role + toast", async () => {
    getWorkspaceMembersMock
      .mockResolvedValueOnce({
        data: [
          {
            id: "m_1",
            userId: "u_1",
            email: "owner@acme.com",
            name: "Owner",
            role: "owner",
            joinedAt: "2026-01-01T00:00:00Z",
            status: "active",
          },
        ],
      })
      .mockResolvedValueOnce({
        data: [
          {
            id: "m_1",
            userId: "u_1",
            email: "owner@acme.com",
            name: "Owner",
            role: "owner",
            joinedAt: "2026-01-01T00:00:00Z",
            status: "active",
          },
          {
            id: "inv_1",
            userId: "",
            email: "new@example.com",
            name: "new@example.com",
            role: "member",
            joinedAt: "2026-01-01T00:00:00Z",
            status: "pending",
          },
        ],
      });

    await renderPage();
    expect(await screen.findByText("owner@acme.com")).toBeTruthy();
    expect(screen.getAllByText("Owner").length).toBeGreaterThan(0);

    const emailInput = screen.getByPlaceholderText("Email address");
    fireEvent.change(emailInput, { target: { value: "New@Example.com" } });
    fireEvent.click(screen.getByRole("button", { name: /^Invite$/i }));

    await waitFor(() => {
      expect(inviteWorkspaceMemberMock).toHaveBeenCalledWith("new@example.com", "member");
      expect(toast.success).toHaveBeenCalledWith("Invitation sent to new@example.com");
    });
    expect(await screen.findByText("Pending")).toBeTruthy();
    expect(screen.getByText("new@example.com")).toBeTruthy();
  });

  it("renders pending invite rows from the members list", async () => {
    getWorkspaceMembersMock.mockResolvedValue({
      data: [
        {
          id: "inv_pending",
          userId: "",
          email: "pending@acme.com",
          name: "pending@acme.com",
          role: "admin",
          joinedAt: "2026-01-02T00:00:00Z",
          status: "pending",
        },
      ],
    });

    await renderPage();
    expect(await screen.findByText("pending@acme.com")).toBeTruthy();
    expect(screen.getByText("Pending")).toBeTruthy();
    expect(screen.getByText("Admin")).toBeTruthy();
  });

  it("exposes invite role options", async () => {
    await renderPage();
    await screen.findByText("owner@acme.com");

    fireEvent.click(screen.getByLabelText("Role"));
    expect(await screen.findByRole("option", { name: "Admin" })).toBeTruthy();
    expect(screen.getByRole("option", { name: "Member" })).toBeTruthy();
    expect(screen.getByRole("option", { name: "Guest" })).toBeTruthy();
  });

  it("hides admin invite option for admin actors", async () => {
    vi.mocked(getCachedAccountEmail).mockReturnValue("admin@acme.com");
    getWorkspaceMembersMock.mockResolvedValue({
      data: [
        {
          id: "m_admin",
          userId: "u_admin",
          email: "admin@acme.com",
          name: "Admin User",
          role: "admin",
          joinedAt: "2026-01-01T00:00:00Z",
          status: "active",
        },
      ],
    });

    await renderPage();
    await screen.findByText("admin@acme.com");

    fireEvent.click(screen.getByLabelText("Role"));
    expect(await screen.findByRole("option", { name: "Member" })).toBeTruthy();
    expect(screen.getByRole("option", { name: "Guest" })).toBeTruthy();
    expect(screen.queryByRole("option", { name: "Admin" })).toBeNull();
  });

  it("rejects invalid email before calling the API", async () => {
    await renderPage();
    await screen.findByText("owner@acme.com");

    fireEvent.change(screen.getByPlaceholderText("Email address"), {
      target: { value: "not-an-email" },
    });
    expect(screen.getByRole("button", { name: /^Invite$/i })).toBeDisabled();
  });

  it("surfaces already_member conflicts via toast", async () => {
    inviteWorkspaceMemberMock.mockRejectedValueOnce(
      new ApiError({
        status: 409,
        code: "already_member",
        message: "already a member",
        requestId: "req_1",
      }),
    );
    await renderPage();
    await screen.findByText("owner@acme.com");

    fireEvent.change(screen.getByPlaceholderText("Email address"), {
      target: { value: "owner@acme.com" },
    });
    fireEvent.click(screen.getByRole("button", { name: /^Invite$/i }));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("This person is already a member.");
    });
  });

  it("updates a pending invitation role from row actions", async () => {
    getWorkspaceMembersMock.mockResolvedValue({
      data: [
        {
          id: "inv_pending",
          userId: "",
          email: "pending@acme.com",
          name: "pending@acme.com",
          role: "member",
          joinedAt: "2026-01-02T00:00:00Z",
          status: "pending",
        },
      ],
    });

    await renderPage();
    await screen.findByText("pending@acme.com");
    fireEvent.click(screen.getByLabelText("More actions"));
    fireEvent.click(await screen.findByText("Edit role"));
    expect(await screen.findByText("Choose a new role for pending@acme.com.")).toBeTruthy();
    expect(screen.getByRole("button", { name: /Save role/i })).toBeTruthy();
  });

  it("revokes a pending invitation from row actions", async () => {
    getWorkspaceMembersMock.mockResolvedValue({
      data: [
        {
          id: "inv_pending",
          userId: "",
          email: "pending@acme.com",
          name: "pending@acme.com",
          role: "member",
          joinedAt: "2026-01-02T00:00:00Z",
          status: "pending",
        },
      ],
    });

    await renderPage();
    await screen.findByText("pending@acme.com");
    fireEvent.click(screen.getByLabelText("More actions"));
    fireEvent.click(await screen.findByRole("menuitem", { name: "Remove" }));
    fireEvent.click(await screen.findByRole("button", { name: /^Remove$/i }));

    await waitFor(() => {
      expect(revokeWorkspaceInvitationMock).toHaveBeenCalledWith("inv_pending");
      expect(toast.success).toHaveBeenCalledWith("Invitation to pending@acme.com was revoked");
    });
  });
});
