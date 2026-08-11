// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { createTestI18n } from "@/i18n/test-utils";
import { DealRoomAccessRequestsPanel } from "./DealRoomAccessRequestsPanel";

const {
  getDealRoomAccessRequestsMock,
  getPendingLinkAccessRequestsMock,
  approveLinkAccessRequestMock,
} = vi.hoisted(() => ({
  getDealRoomAccessRequestsMock: vi.fn(),
  getPendingLinkAccessRequestsMock: vi.fn(),
  approveLinkAccessRequestMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getDealRoomAccessRequests: getDealRoomAccessRequestsMock,
    getPendingLinkAccessRequests: getPendingLinkAccessRequestsMock,
    approveLinkAccessRequest: approveLinkAccessRequestMock,
    rejectLinkAccessRequest: vi.fn(),
    approveDealRoomAccessRequest: vi.fn(),
    rejectDealRoomAccessRequest: vi.fn(),
  },
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), warning: vi.fn(), message: vi.fn() },
}));

async function renderPanel(opts?: { focusLinkId?: string }) {
  const i18nInstance = await createTestI18n({
    dealRooms: {
      "accessRequests.title": "Access requests",
      "accessRequests.description": "Pending visitor requests",
      "accessRequests.linkLabel": "Share link: {{name}}",
      "accessRequests.signerName": "Signer: {{name}}",
      "accessRequests.approve": "Approve",
      "accessRequests.reject": "Reject",
      "accessRequests.approveSuccess": "ok",
      "accessRequests.approveError": "fail",
      "accessRequests.rejectSuccess": "ok",
      "accessRequests.rejectError": "fail",
      "accessRequests.loadFailed": "load failed",
    },
    linkShare: {
      "accessRequests.approveSuccess": "ok",
      "accessRequests.approveError": "fail",
      "accessRequests.rejectSuccess": "ok",
      "accessRequests.rejectError": "fail",
      "accessRequests.codeSendFailed": "approved but code failed",
      "accessRequests.resendCode": "Resend",
    },
    common: {
      loading: "Loading…",
      retry: "Retry",
    },
  });
  const view = render(
    <I18nextProvider i18n={i18nInstance}>
      <DealRoomAccessRequestsPanel roomId="room-1" focusLinkId={opts?.focusLinkId} />
    </I18nextProvider>
  );
  await act(async () => {
    await Promise.resolve();
  });
  return view;
}

describe("DealRoomAccessRequestsPanel", () => {
  beforeEach(() => {
    getDealRoomAccessRequestsMock.mockReset();
    getPendingLinkAccessRequestsMock.mockReset();
    approveLinkAccessRequestMock.mockReset();

    getDealRoomAccessRequestsMock.mockResolvedValue({ data: [] });
    getPendingLinkAccessRequestsMock.mockResolvedValue({
      data: [
        {
          id: "req-1",
          link_id: "link-1",
          email: "visitor@example.com",
          reason: "need docs",
          signer_name: "Visitor",
          link_name: "测啊",
          status: "pending",
          created_at: "2026-07-21T00:00:00Z",
          updated_at: "2026-07-21T00:00:00Z",
        },
      ],
    });
    approveLinkAccessRequestMock.mockResolvedValue({ data: { id: "req-1", status: "approved" } });
  });

  it("aggregates pending share-link access requests for the room", async () => {
    await renderPanel();

    await waitFor(() => {
      expect(screen.getByTestId("deal-room-access-requests-panel")).toBeInTheDocument();
    });
    expect(screen.getByText("visitor@example.com")).toBeInTheDocument();
    expect(screen.getByText(/测啊/)).toBeInTheDocument();
    expect(getPendingLinkAccessRequestsMock).toHaveBeenCalledWith({
      scope: "deal_room",
      dealRoomId: "room-1",
    });
  });

  it("approves via the link access-request API", async () => {
    await renderPanel();

    await waitFor(() => {
      expect(screen.getByTestId("deal-room-access-request-req-1")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("button", { name: /approve/i }));
    await waitFor(() => {
      expect(approveLinkAccessRequestMock).toHaveBeenCalledWith("link-1", "req-1");
    });
  });

  it("removes the request when approve returns code-send warning", async () => {
    const { toast } = await import("sonner");
    approveLinkAccessRequestMock.mockResolvedValue({
      data: { id: "req-1", status: "approved" },
      warning: {
        code: "access_code_send_failed",
        message: "could not send verification code",
      },
    });
    getPendingLinkAccessRequestsMock
      .mockResolvedValueOnce({
        data: [
          {
            id: "req-1",
            link_id: "link-1",
            email: "visitor@example.com",
            reason: "need docs",
            signer_name: "Visitor",
            link_name: "测啊",
            status: "pending",
            created_at: "2026-07-21T00:00:00Z",
            updated_at: "2026-07-21T00:00:00Z",
          },
        ],
      })
      .mockResolvedValue({ data: [] });

    await renderPanel();
    await waitFor(() => {
      expect(screen.getByTestId("deal-room-access-request-req-1")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("button", { name: /approve/i }));

    await waitFor(() => {
      expect(toast.warning).toHaveBeenCalledWith(
        "approved but code failed",
        expect.objectContaining({
          action: expect.objectContaining({ label: "Resend" }),
        }),
      );
    });
    await waitFor(() => {
      expect(screen.queryByTestId("deal-room-access-requests-panel")).not.toBeInTheDocument();
    });
  });

  it("highlights the deep-linked share link applicant", async () => {
    await renderPanel({ focusLinkId: "link-1" });

    await waitFor(() => {
      expect(screen.getByTestId("deal-room-access-request-req-1")).toHaveAttribute(
        "data-focused",
        "true",
      );
    });
  });
});
