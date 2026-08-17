// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { createTestI18n } from "@/i18n/test-utils";
import { DealRoomMembersTab } from "./DealRoomMembersTab";
import type { DealRoomMember } from "@/types";

const { getDealRoomMembersMock } = vi.hoisted(() => ({
  getDealRoomMembersMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getDealRoomMembers: getDealRoomMembersMock,
    getDealRoomById: vi.fn().mockResolvedValue({
      id: "room-1",
      ndaEnabled: false,
    }),
    inviteDealRoomMember: vi.fn(),
    updateDealRoomMemberRole: vi.fn(),
    removeDealRoomMember: vi.fn(),
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
];

async function renderTab(canManage: boolean) {
  const i18n = await createTestI18n({
    dealRooms: {
      pageTabs: { members: "Members" },
      members: {
        inviteTitle: "Invite members",
        pageDescription: "Invite collaborators.",
        oversightHint: "View only",
        listTitle: "Room members",
        empty: "No members yet.",
        roles: { owner: "Owner" },
      },
    },
  });
  return render(
    <I18nextProvider i18n={i18n}>
      <DealRoomMembersTab
        roomId="room-1"
        room={{ roomRole: canManage ? "owner" : "", ndaEnabled: false }}
        canManage={canManage}
      />
    </I18nextProvider>,
  );
}

describe("DealRoomMembersTab", () => {
  beforeEach(() => {
    getDealRoomMembersMock.mockReset();
    getDealRoomMembersMock.mockResolvedValue({ data: members });
  });

  it("shows invite for room managers", async () => {
    await renderTab(true);
    expect(screen.getByTestId("deal-room-members-tab")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /invite members/i })).toBeInTheDocument();
    expect(screen.getByText("Invite collaborators.")).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByTestId("deal-room-member-m-owner")).toBeInTheDocument();
    });
  });

  it("hides invite for oversight and room viewers", async () => {
    await renderTab(false);
    expect(screen.queryByRole("button", { name: /invite members/i })).not.toBeInTheDocument();
    expect(screen.getByText("View only")).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByTestId("deal-room-member-m-owner")).toBeInTheDocument();
    });
  });

  it("hides invite when canManage is set without a grantable room role", async () => {
    const i18n = await createTestI18n({
      dealRooms: {
        pageTabs: { members: "Members" },
        members: {
          inviteTitle: "Invite members",
          pageDescription: "Invite collaborators.",
          oversightHint: "View only",
          listTitle: "Room members",
          empty: "No members yet.",
          roles: { owner: "Owner" },
        },
      },
    });
    render(
      <I18nextProvider i18n={i18n}>
        <DealRoomMembersTab
          roomId="room-1"
          room={{ roomRole: "", ndaEnabled: false }}
          canManage
        />
      </I18nextProvider>,
    );
    expect(screen.queryByRole("button", { name: /invite members/i })).not.toBeInTheDocument();
  });
});
