// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { InviteMemberDialog } from "./InviteMemberDialog";
import { Button } from "@/components/ui/button";

const {
  inviteDealRoomMemberMock,
  patchDealRoomNdaAgreementMock,
  getDealRoomByIdMock,
  listNDATemplatesMock,
  getDocumentsMock,
} = vi.hoisted(() => ({
  inviteDealRoomMemberMock: vi.fn(),
  patchDealRoomNdaAgreementMock: vi.fn(),
  getDealRoomByIdMock: vi.fn(),
  listNDATemplatesMock: vi.fn(),
  getDocumentsMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    inviteDealRoomMember: inviteDealRoomMemberMock,
    patchDealRoomNdaAgreement: patchDealRoomNdaAgreementMock,
    getDealRoomById: getDealRoomByIdMock,
    listNDATemplates: listNDATemplatesMock,
    getDocuments: getDocumentsMock,
  },
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

async function renderInvite(props: {
  ndaEnabled?: boolean;
  ndaTemplateId?: string;
  actorRoomRole?: "owner" | "admin" | "";
}) {
  const instance = i18n.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    resources: {
      en: {
        dealRooms: {
          detail: { invite: "Invite" },
          members: {
            inviteTitle: "Invite members",
            inviteDescription: "Invite people to access this data room.",
            inviteDescriptionNda: "They must sign the selected NDA.",
            ndaAgreement: "NDA agreement",
            ndaAgreementHint: "Every member signs the same agreement.",
            ndaAgreementPlaceholder: "Select an NDA agreement",
            ndaAgreementRequired: "Select an NDA agreement before inviting members",
            ndaAgreementEmpty: "No NDA agreements yet.",
            ndaAgreementUntitled: "Untitled agreement",
            email: "Email",
            emailPlaceholder: "investor@example.com",
            role: "Role",
            roles: { owner: "Owner", admin: "Admin", guest: "Visitor", member: "Member" },
            inviting: "Inviting...",
            invited: "Invited {{email}}",
            addEmail: "Add email",
            removeEmail: "Remove email",
          },
        },
        common: { cancel: "Cancel" },
      },
    },
  });
  return render(
    <I18nextProvider i18n={instance}>
      <InviteMemberDialog
        roomId="room-1"
        actorRoomRole={props.actorRoomRole}
        ndaEnabled={props.ndaEnabled}
        ndaTemplateId={props.ndaTemplateId}
        onInvited={() => undefined}
      >
        <Button>Invite members</Button>
      </InviteMemberDialog>
    </I18nextProvider>,
  );
}

describe("InviteMemberDialog", () => {
  beforeEach(() => {
    inviteDealRoomMemberMock.mockReset();
    patchDealRoomNdaAgreementMock.mockReset();
    getDealRoomByIdMock.mockReset();
    listNDATemplatesMock.mockReset();
    getDocumentsMock.mockReset();
    inviteDealRoomMemberMock.mockResolvedValue({ data: { id: "m1" } });
    patchDealRoomNdaAgreementMock.mockResolvedValue({ id: "room-1" });
    listNDATemplatesMock.mockResolvedValue({
      data: [{ id: "tpl-1", name: "Standard NDA", source_document_id: "doc-1" }],
    });
    getDocumentsMock.mockResolvedValue({ data: [] });
  });

  it("does not require an NDA when the room does not require one", async () => {
    getDealRoomByIdMock.mockResolvedValue({
      id: "room-1",
      ndaEnabled: false,
    });
    await renderInvite({ ndaEnabled: false, actorRoomRole: "owner" });
    fireEvent.click(screen.getByRole("button", { name: /invite members/i }));
    await waitFor(() => {
      expect(screen.getByLabelText("Email")).toBeInTheDocument();
    });
    expect(screen.queryByLabelText("NDA agreement")).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Email"), {
      target: { value: "lp@example.com" },
    });
    fireEvent.click(screen.getByRole("button", { name: /^invite$/i }));
    await waitFor(() => {
      expect(inviteDealRoomMemberMock).toHaveBeenCalledWith("room-1", {
        email: "lp@example.com",
        role: "guest",
      });
    });
    expect(patchDealRoomNdaAgreementMock).not.toHaveBeenCalled();
  });

  it("does not offer Owner when a room owner invites", async () => {
    getDealRoomByIdMock.mockResolvedValue({
      id: "room-1",
      ndaEnabled: false,
    });
    await renderInvite({ ndaEnabled: false, actorRoomRole: "owner" });
    fireEvent.click(screen.getByRole("button", { name: /invite members/i }));
    await waitFor(() => {
      expect(screen.getByTestId("deal-room-invite-role")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByTestId("deal-room-invite-role"));
    expect(await screen.findByRole("option", { name: "Admin" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Member" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Visitor" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Owner" })).not.toBeInTheDocument();
  });

  it("does not offer Admin when a room admin invites", async () => {
    getDealRoomByIdMock.mockResolvedValue({
      id: "room-1",
      ndaEnabled: false,
    });
    await renderInvite({ ndaEnabled: false, actorRoomRole: "admin" });
    fireEvent.click(screen.getByRole("button", { name: /invite members/i }));
    await waitFor(() => {
      expect(screen.getByTestId("deal-room-invite-role")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByTestId("deal-room-invite-role"));
    expect(await screen.findByRole("option", { name: "Member" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Visitor" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Admin" })).not.toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Owner" })).not.toBeInTheDocument();
  });

  it("persists a selected agreement document then invites", async () => {
    getDealRoomByIdMock.mockResolvedValue({
      id: "room-1",
      ndaEnabled: true,
    });
    listNDATemplatesMock.mockResolvedValue({ data: [] });
    getDocumentsMock.mockResolvedValue({
      data: [
        {
          id: "b6d63b2f-71cf-4712-9629-28a69f4d9fc3",
          title: "Mutual NDA",
          status: "ready",
        },
      ],
    });
    await renderInvite({ ndaEnabled: true, actorRoomRole: "owner" });
    fireEvent.click(screen.getByRole("button", { name: /invite members/i }));
    await waitFor(() => {
      expect(screen.getByLabelText("NDA agreement")).toBeInTheDocument();
    });
    fireEvent.change(screen.getByLabelText("Email"), {
      target: { value: "lp@example.com" },
    });
    fireEvent.click(screen.getByTestId("room-nda-agreement-select"));
    fireEvent.click(await screen.findByRole("option", { name: "Mutual NDA" }));
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^invite$/i })).toBeEnabled();
    });
    fireEvent.click(screen.getByRole("button", { name: /^invite$/i }));
    await waitFor(() => {
      expect(patchDealRoomNdaAgreementMock).toHaveBeenCalledWith("room-1", {
        nda_template_id: undefined,
        nda_document_id: "b6d63b2f-71cf-4712-9629-28a69f4d9fc3",
      });
      expect(inviteDealRoomMemberMock).toHaveBeenCalledWith("room-1", {
        email: "lp@example.com",
        role: "guest",
      });
    });
  });

  it("blocks invite until an NDA agreement is selected", async () => {
    getDealRoomByIdMock.mockResolvedValue({
      id: "room-1",
      ndaEnabled: true,
      ndaTemplateId: "",
    });
    await renderInvite({ ndaEnabled: true, actorRoomRole: "owner" });
    fireEvent.click(screen.getByRole("button", { name: /invite members/i }));
    await waitFor(() => {
      expect(screen.getByLabelText("NDA agreement")).toBeInTheDocument();
    });
    fireEvent.change(screen.getByLabelText("Email"), {
      target: { value: "lp@example.com" },
    });
    expect(screen.getByRole("button", { name: /^invite$/i })).toBeDisabled();
  });

  it("persists the room NDA then invites when an agreement is already bound", async () => {
    getDealRoomByIdMock.mockResolvedValue({
      id: "room-1",
      ndaEnabled: true,
      ndaTemplateId: "tpl-1",
      ndaDocumentId: "doc-1",
    });
    await renderInvite({ ndaEnabled: true, ndaTemplateId: "tpl-1", actorRoomRole: "owner" });
    fireEvent.click(screen.getByRole("button", { name: /invite members/i }));
    await waitFor(() => {
      expect(screen.getByLabelText("NDA agreement")).toBeInTheDocument();
    });
    fireEvent.change(screen.getByLabelText("Email"), {
      target: { value: "lp@example.com" },
    });
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^invite$/i })).toBeEnabled();
    });
    fireEvent.click(screen.getByRole("button", { name: /^invite$/i }));
    await waitFor(() => {
      expect(patchDealRoomNdaAgreementMock).toHaveBeenCalledWith("room-1", {
        nda_template_id: "tpl-1",
        nda_document_id: "doc-1",
      });
      expect(inviteDealRoomMemberMock).toHaveBeenCalledWith("room-1", {
        email: "lp@example.com",
        role: "guest",
      });
    });
  });

  it("does not invite when the actor has no grantable room role", async () => {
    getDealRoomByIdMock.mockResolvedValue({
      id: "room-1",
      ndaEnabled: false,
    });
    await renderInvite({ ndaEnabled: false, actorRoomRole: "" });
    fireEvent.click(screen.getByRole("button", { name: /invite members/i }));
    await waitFor(() => {
      expect(screen.getByLabelText("Email")).toBeInTheDocument();
    });
    fireEvent.change(screen.getByLabelText("Email"), {
      target: { value: "lp@example.com" },
    });
    expect(screen.getByRole("button", { name: /^invite$/i })).toBeDisabled();
    expect(screen.queryByRole("option")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /^invite$/i }));
    expect(inviteDealRoomMemberMock).not.toHaveBeenCalled();
  });
});
