// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { createTestI18n } from "@/i18n/test-utils";
import { DealRoomMembersPanel } from "./DealRoomMembersPanel";
import type { DealRoomMember } from "@/types";

const {
  getDealRoomMembersMock,
  updateDealRoomMemberRoleMock,
  removeDealRoomMemberMock,
} = vi.hoisted(() => ({
  getDealRoomMembersMock: vi.fn(),
  updateDealRoomMemberRoleMock: vi.fn(),
  removeDealRoomMemberMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getDealRoomMembers: getDealRoomMembersMock,
    updateDealRoomMemberRole: updateDealRoomMemberRoleMock,
    removeDealRoomMember: removeDealRoomMemberMock,
  },
}));

vi.mock("@/lib/authAccount", () => ({
  getCachedAccountEmail: () => "owner@example.com",
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

const members: DealRoomMember[] = [
  {
    id: "m-owner",
    email: "owner@example.com",
    role: "owner",
    nda_status: "none",
    status: "active",
    name: "Owner",
  },
  {
    id: "m-guest",
    email: "lp@example.com",
    role: "guest",
    nda_status: "none",
    status: "active",
  },
];

async function renderPanel(opts?: { canManage?: boolean; actorRoomRole?: "owner" | "admin" | "" }) {
  const i18nInstance = await createTestI18n({
    dealRooms: {
      members: {
        listTitle: "Room members",
        listDescription: "Change grantable roles.",
        oversightHint: "View only",
        empty: "No members yet.",
        loadFailed: "Could not load members.",
        retry: "Retry",
        role: "Role",
        roleUpdated: "Updated role for {{email}}",
        remove: "Remove {{email}}",
        removeConfirm: "Remove {{email}} from this data room?",
        removed: "Removed {{email}}",
        removing: "Removing...",
        cannotModifyOwner: "The room owner cannot be changed here.",
        cannotModifySelf: "You cannot change your own room role here.",
        cannotManageMember: "You do not have permission to manage this member.",
        roles: { owner: "Owner", admin: "Admin", member: "Member", guest: "Visitor" },
        status: { pending: "Pending" },
      },
    },
    common: { cancel: "Cancel" },
  });
  const view = render(
    <I18nextProvider i18n={i18nInstance}>
      <DealRoomMembersPanel
        roomId="room-1"
        canManage={opts?.canManage ?? true}
        actorRoomRole={opts?.actorRoomRole ?? "owner"}
      />
    </I18nextProvider>,
  );
  await act(async () => {
    await Promise.resolve();
  });
  return view;
}

describe("DealRoomMembersPanel", () => {
  beforeEach(() => {
    getDealRoomMembersMock.mockReset();
    updateDealRoomMemberRoleMock.mockReset();
    removeDealRoomMemberMock.mockReset();
    getDealRoomMembersMock.mockResolvedValue({ data: members });
    updateDealRoomMemberRoleMock.mockResolvedValue(members[1]);
    removeDealRoomMemberMock.mockResolvedValue(undefined);
  });

  it("shows a read-only roster for oversight", async () => {
    await renderPanel({ canManage: false, actorRoomRole: "" });
    await waitFor(() => {
      expect(screen.getByText("View only")).toBeInTheDocument();
    });
    expect(screen.getByTestId("deal-room-member-m-owner")).toBeInTheDocument();
    expect(screen.getByText("lp@example.com")).toBeInTheDocument();
    expect(screen.queryByTestId("deal-room-member-role-m-guest")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("moreActions")).not.toBeInTheDocument();
  });

  it("lets a room owner edit grantable members but not the owner row", async () => {
    await renderPanel({ canManage: true, actorRoomRole: "owner" });
    await waitFor(() => {
      expect(screen.getByTestId("deal-room-member-role-m-guest")).toBeInTheDocument();
    });
    expect(screen.queryByTestId("deal-room-member-role-m-owner")).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId("deal-room-member-role-m-guest"));
    expect(await screen.findByRole("option", { name: "Admin" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Member" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Visitor" })).toBeInTheDocument();
  });
});
