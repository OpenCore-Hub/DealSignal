// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { DealRoomDocumentsHome } from "./DealRoomDocumentsHome";
import { DealRoomActivityTab } from "./DealRoomActivityTab";
import { DealRoomSettingsTab } from "./DealRoomSettingsTab";
import { badgeCountForTab } from "@/stores/dealRoomNavStore";
import { deriveRoomStage } from "@/lib/dealRoomNav";
import { findMissingRecommendedFiles } from "@/lib/dealRoomReadiness";

async function withI18n(ui: React.ReactElement) {
  const instance = i18n.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    resources: {
      en: {
        dealRooms: {
          documentsHome: {
            attention: {
              noActiveLinks: "No active share links yet — create one in Share",
              failedDeliveries_one: "{{count}} access code failed to send — fix in Share",
              failedDeliveries_other: "{{count}} access codes failed to send — fix in Share",
              unreadQuestions_one: "{{count}} unanswered question — open Q&A",
              unreadQuestions_other: "{{count}} unanswered questions — open Q&A",
            },
          },
          activity: {
            title: "Activity",
            description: "Recent access",
            emptyTitle: "No activity yet",
            emptyDescription: "Empty",
            goShare: "Manage share links",
            goAnalytics: "View analytics",
            linkFallback: "Share link",
            linkViews_one: "{{count}} view",
            linkViews_other: "{{count}} views",
          },
          settings: {
            title: "Room Settings",
            description: "Policy",
            moreComing: "More coming",
            membersHint: "Invite on Members",
            openMembers: "Open Members",
            ndaSaved: "NDA agreement saved",
            enabled: "On",
            disabled: "Off",
            fields: {
              stage: "Stage",
              status: "Status",
              nda: "NDA required",
              requiresApproval: "Access approval",
              members: "Members",
            },
            stage: { preparing: "Preparing", open: "Open" },
            stageHint: "Derived",
            status: { active: "Active", archived: "Archived", pending: "Pending" },
          },
          members: {
            ndaAgreement: "NDA agreement",
            ndaAgreementHint: "Every member of this room signs the same agreement.",
            ndaAgreementPlaceholder: "Select an NDA agreement",
            ndaAgreementRequired: "Select an NDA agreement before inviting members",
            ndaAgreementEmpty: "No NDA agreements yet.",
            inviteTitle: "Invite members",
            inviteDescription: "Invite people",
            inviteDescriptionNda: "They must sign the NDA.",
            listTitle: "Room members",
            listDescription: "Change grantable roles.",
            oversightHint: "View only",
            empty: "No members yet.",
            roles: { owner: "Owner", admin: "Admin", member: "Member", guest: "Visitor" },
          },
          detail: { invite: "Invite" },
        },
      },
    },
  });
  return render(<I18nextProvider i18n={instance}>{ui}</I18nextProvider>);
}

vi.mock("@/hooks/useWorkspaceAccess", () => ({
  useWorkspaceAccess: () => ({
    role: "owner",
    loading: false,
    canRead: true,
    canWrite: true,
    canManage: true,
    isGuest: false,
  }),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getWorkspaces: vi.fn().mockResolvedValue({ data: [] }),
    listNDATemplates: vi.fn().mockResolvedValue({ data: [] }),
    getDocuments: vi.fn().mockResolvedValue({ data: [] }),
    getDealRoomById: vi.fn().mockResolvedValue({
      id: "room-1",
      ndaEnabled: true,
      ndaTemplateId: "",
      ndaDocumentId: "",
    }),
    patchDealRoomNdaAgreement: vi.fn(),
    inviteDealRoomMember: vi.fn(),
    getDealRoomMembers: vi.fn().mockResolvedValue({ data: [] }),
    updateDealRoomMemberRole: vi.fn(),
    removeDealRoomMember: vi.fn(),
  },
}));

describe("deal room nav B′ pieces", () => {
  it("deriveRoomStage uses active links", () => {
    expect(deriveRoomStage(0)).toBe("preparing");
    expect(deriveRoomStage(2)).toBe("open");
  });

  it("badgeCountForTab maps share failures, outbound setup, and qa", () => {
    expect(
      badgeCountForTab("links", {
        failedDeliveries: 3,
        unreadQuestions: 1,
        activeLinkCount: 0,
      })
    ).toBe(3);
    expect(
      badgeCountForTab("links", {
        failedDeliveries: 0,
        unreadQuestions: 0,
        activeLinkCount: 0,
      })
    ).toBe(1);
    expect(
      badgeCountForTab("links", {
        failedDeliveries: 0,
        unreadQuestions: 0,
        activeLinkCount: 2,
      })
    ).toBe(0);
    expect(
      badgeCountForTab("qa", { failedDeliveries: 3, unreadQuestions: 1, activeLinkCount: 0 })
    ).toBe(1);
  });

  it("findMissingRecommendedFiles ignores matched titles", () => {
    expect(
      findMissingRecommendedFiles(["Pitch deck", "Cap table"], ["Acme Seed Round Pitch Deck"])
    ).toEqual(["Cap table"]);
  });

  it("documents home only shows attention when needed, not command strip or readiness", async () => {
    const onJumpTab = vi.fn();
    await withI18n(
      <DealRoomDocumentsHome
        activeLinkCount={2}
        failedDeliveries={2}
        unreadQuestions={1}
        onJumpTab={onJumpTab}
      >
        <div>tree</div>
      </DealRoomDocumentsHome>
    );

    expect(screen.queryByTestId("deal-room-command-strip")).not.toBeInTheDocument();
    expect(screen.queryByTestId("deal-room-readiness")).not.toBeInTheDocument();
    expect(screen.getByTestId("deal-room-attention-banner")).toBeInTheDocument();
    expect(screen.getByText(/2 access codes failed/i)).toBeInTheDocument();
    fireEvent.click(screen.getByText(/2 access codes failed/i));
    expect(onJumpTab).toHaveBeenCalledWith("links");
  });

  it("does not show no-link attention on the resources tab", async () => {
    await withI18n(
      <DealRoomDocumentsHome
        activeLinkCount={0}
        failedDeliveries={0}
        unreadQuestions={0}
        onJumpTab={vi.fn()}
      >
        <div>tree</div>
      </DealRoomDocumentsHome>
    );

    expect(screen.queryByText(/no active share links/i)).not.toBeInTheDocument();
    expect(screen.queryByTestId("deal-room-attention-banner")).not.toBeInTheDocument();
  });

  it("hides attention when room signals are healthy", async () => {
    await withI18n(
      <DealRoomDocumentsHome
        activeLinkCount={1}
        failedDeliveries={0}
        unreadQuestions={0}
        onJumpTab={vi.fn()}
      >
        <div>tree</div>
      </DealRoomDocumentsHome>
    );

    expect(screen.queryByTestId("deal-room-attention-banner")).not.toBeInTheDocument();
    expect(screen.getByTestId("deal-room-documents-chrome")).toBeInTheDocument();
    expect(screen.getByTestId("deal-room-resources-toolbar-host")).toBeInTheDocument();
  });

  it("activity tab lists recent visitors", async () => {
    await withI18n(
      <DealRoomActivityTab
        recentVisitors={[
          {
            email: "lp@example.com",
            name: "LP",
            heatLevel: "warm",
            lastSeenAt: "2026-07-20T10:00:00Z",
          },
        ]}
      />
    );
    expect(screen.getByText("LP")).toBeInTheDocument();
    expect(screen.getByText("lp@example.com")).toBeInTheDocument();
  });

  it("settings tab shows derived open stage", async () => {
    await withI18n(
      <DealRoomSettingsTab
        roomId="room-1"
        room={{
          status: "active",
          ndaEnabled: true,
          requiresApproval: false,
          memberCount: 4,
        }}
        canManage
        activeLinkCount={2}
      />
    );
    expect(screen.getByTestId("deal-room-settings-tab")).toBeInTheDocument();
    expect(screen.getAllByText("Open").length).toBeGreaterThan(0);
    expect(screen.getByText("On")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /invite members/i })).not.toBeInTheDocument();
    expect(screen.queryByTestId("deal-room-members-panel")).not.toBeInTheDocument();
  });
});
