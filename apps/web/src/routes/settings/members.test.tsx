// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act, within } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { SettingsMembersPage, internalSeatsAtCap, inviteRoleBlockedBySeats } from "./members";
import { toast } from "sonner";
import { ApiError } from "@/lib/apiClient";
import { getCachedAccountEmail } from "@/lib/authAccount";

const {
  getWorkspaceMembersMock,
  getBillingInfoMock,
  inviteWorkspaceMemberMock,
  updateWorkspaceInvitationRoleMock,
  revokeWorkspaceInvitationMock,
  updateWorkspaceMemberRoleMock,
  removeWorkspaceMemberMock,
} = vi.hoisted(() => ({
  getWorkspaceMembersMock: vi.fn(),
  getBillingInfoMock: vi.fn(),
  inviteWorkspaceMemberMock: vi.fn(),
  updateWorkspaceInvitationRoleMock: vi.fn(),
  revokeWorkspaceInvitationMock: vi.fn(),
  updateWorkspaceMemberRoleMock: vi.fn(),
  removeWorkspaceMemberMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getWorkspaceMembers: getWorkspaceMembersMock,
    getBillingInfo: getBillingInfoMock,
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
        title: "Workspace members",
        description:
          "These are workspace roles, not data-room roles. Invite people into a specific data room from that room’s Members tab.",
        emailPlaceholder: "Email address",
        roleLabel: "Workspace role",
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
        removeDescription:
          "Remove {{email}} from this workspace? They will lose workspace access. If they are the only operator of a data room, removal is blocked.",
        removePendingDescription: "Revoke the invitation sent to {{email}}?",
        removing: "Removing...",
        removed: "{{email}} was removed",
        inviteRevoked: "Invitation to {{email}} was revoked",
        removeFailed: "Could not remove this person. Please try again.",
        cannotModifyOwner: "The workspace owner cannot be changed here",
        cannotModifySelf: "You cannot change your own membership here",
        cannotManageMember: "You do not have permission to manage this member",
        seatsUsage: "Internal seats",
        seatLimitReached:
          "You've reached the team seat limit. Invite workspace guests for free, or upgrade to add more members.",
        roleHints: {
          admin: "Can manage workspace settings and view every data room.",
          member: "Can create data rooms and only sees rooms they belong to.",
          guest: "Free workspace access. Does not use a seat. Not a share-link visitor.",
        },
        status: { pending: "Pending", active: "Active", suspended: "Suspended" },
        roles: {
          owner: "Workspace owner",
          admin: "Workspace admin",
          member: "Workspace member",
          guest: "Workspace guest",
        },
      },
    },
    common: {
      cancel: "Cancel",
      save: "Save",
      delete: "Delete",
      moreActions: "More actions",
      unlimited: "Unlimited",
      usageAtLimit: "At limit",
      usageNearLimit: "Near limit",
      error: {
        saveFailed: "Failed to save",
        deleteFailed: "Failed to delete",
        codes: {
          already_member: "This person is already a member.",
          plan_limit_seats: "You've reached the team seat limit for your plan. Upgrade to invite more members.",
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
    vi.mocked(getCachedAccountEmail).mockReturnValue("owner@acme.com");
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
    getBillingInfoMock.mockResolvedValue({
      plan: "trial",
      period: "monthly",
      trialExpired: false,
      storageUsed: 0,
      storageLimit: 0,
      linksUsed: 0,
      linksLimit: 0,
      roomsUsed: 0,
      roomsLimit: 0,
      seatsUsed: 1,
      seatsLimit: 10,
      customDomainEnabled: true,
      watermarkEnabled: true,
      ndaEnabled: true,
      visitorAskAiEnabled: true,
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

  it("computes internal seat cap correctly", () => {
    expect(internalSeatsAtCap(1, 1)).toBe(true);
    expect(internalSeatsAtCap(0, 1)).toBe(false);
    expect(internalSeatsAtCap(5, 0)).toBe(false);
    expect(inviteRoleBlockedBySeats("member", 1, 1)).toBe(true);
    expect(inviteRoleBlockedBySeats("admin", 1, 1)).toBe(true);
    expect(inviteRoleBlockedBySeats("guest", 1, 1)).toBe(false);
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
    expect(screen.getAllByText("Workspace owner").length).toBeGreaterThan(0);

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
    expect(screen.getByText("Workspace admin")).toBeTruthy();
  });

  it("exposes invite role options", async () => {
    await renderPage();
    await screen.findByText("owner@acme.com");

    fireEvent.click(screen.getByLabelText("Workspace role"));
    expect(await screen.findByRole("option", { name: "Workspace admin" })).toBeTruthy();
    expect(screen.getByRole("option", { name: "Workspace member" })).toBeTruthy();
    expect(screen.getByRole("option", { name: "Workspace guest" })).toBeTruthy();
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

    fireEvent.click(screen.getByLabelText("Workspace role"));
    expect(await screen.findByRole("option", { name: "Workspace member" })).toBeTruthy();
    expect(screen.getByRole("option", { name: "Workspace guest" })).toBeTruthy();
    expect(screen.queryByRole("option", { name: "Workspace admin" })).toBeNull();
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
          id: "m_1",
          userId: "u_1",
          email: "owner@acme.com",
          name: "Owner",
          role: "owner",
          joinedAt: "2026-01-01T00:00:00Z",
          status: "active",
        },
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
    const pendingRow = screen.getByText("pending@acme.com").closest("li");
    expect(pendingRow).toBeTruthy();
    fireEvent.click(within(pendingRow as HTMLElement).getByLabelText("More actions"));
    fireEvent.click(await screen.findByText("Edit role"));
    expect(await screen.findByText("Choose a new role for pending@acme.com.")).toBeTruthy();
    expect(screen.getByRole("button", { name: /Save role/i })).toBeTruthy();
  });

  it("revokes a pending invitation from row actions", async () => {
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
    const pendingRow = screen.getByText("pending@acme.com").closest("li");
    expect(pendingRow).toBeTruthy();
    fireEvent.click(within(pendingRow as HTMLElement).getByLabelText("More actions"));
    fireEvent.click(await screen.findByRole("menuitem", { name: "Remove" }));
    fireEvent.click(await screen.findByRole("button", { name: /^Remove$/i }));

    await waitFor(() => {
      expect(revokeWorkspaceInvitationMock).toHaveBeenCalledWith("inv_pending");
      expect(toast.success).toHaveBeenCalledWith("Invitation to pending@acme.com was revoked");
    });
  });

  it("blocks member invites at seat cap and shows usage", async () => {
    getBillingInfoMock.mockResolvedValue({
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
      watermarkEnabled: false,
      ndaEnabled: false,
      visitorAskAiEnabled: false,
    });

    await renderPage();
    await screen.findByTestId("members-seat-usage");
    expect(screen.getByText(/Internal seats/i)).toBeTruthy();

    fireEvent.change(screen.getByPlaceholderText("Email address"), {
      target: { value: "member@example.com" },
    });
    expect(screen.getByRole("button", { name: /^Invite$/i })).toBeDisabled();
    expect(screen.getByTestId("members-seat-limit-hint")).toBeTruthy();
    expect(inviteWorkspaceMemberMock).not.toHaveBeenCalled();
  });

  it("surfaces plan_limit_seats from the API via toast", async () => {
    inviteWorkspaceMemberMock.mockRejectedValueOnce(
      new ApiError({
        status: 403,
        code: "plan_limit_seats",
        message: "internal seat limit reached for this plan",
        requestId: "req_seats",
      }),
    );
    await renderPage();
    await screen.findByText("owner@acme.com");

    fireEvent.change(screen.getByPlaceholderText("Email address"), {
      target: { value: "extra@example.com" },
    });
    fireEvent.click(screen.getByRole("button", { name: /^Invite$/i }));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(
        "You've reached the team seat limit for your plan. Upgrade to invite more members.",
      );
    });
  });

  it("does not treat a missing account email as workspace owner", async () => {
    vi.mocked(getCachedAccountEmail).mockReturnValue(undefined);
    await renderPage();
    expect(await screen.findByText("owner@acme.com")).toBeTruthy();
    expect(screen.queryByTestId("workspace-members-invite")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^Invite$/i })).not.toBeInTheDocument();
  });

  it("explains workspace roles separately from data-room roles", async () => {
    await renderPage();
    expect(await screen.findByTestId("workspace-members-plane-hint")).toHaveTextContent(
      "These are workspace roles, not data-room roles",
    );
    expect(await screen.findByTestId("members-seat-usage")).not.toHaveTextContent(
      "Data-room members and share-link visitors are not counted here",
    );
    expect(screen.getByTestId("workspace-members-invite")).toHaveTextContent(
      "Can create data rooms and only sees rooms they belong to.",
    );
  });
});
