// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { createTestI18n } from "@/i18n/test-utils";
import { DealRoomAccessRequestsPanel } from "./DealRoomAccessRequestsPanel";

const {
  getDealRoomAccessRequestsMock,
  getDealRoomLinksMock,
  getPendingLinkAccessRequestsMock,
  approveLinkAccessRequestMock,
} = vi.hoisted(() => ({
  getDealRoomAccessRequestsMock: vi.fn(),
  getDealRoomLinksMock: vi.fn(),
  getPendingLinkAccessRequestsMock: vi.fn(),
  approveLinkAccessRequestMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getDealRoomAccessRequests: getDealRoomAccessRequestsMock,
    getDealRoomLinks: getDealRoomLinksMock,
    getPendingLinkAccessRequests: getPendingLinkAccessRequestsMock,
    approveLinkAccessRequest: approveLinkAccessRequestMock,
    rejectLinkAccessRequest: vi.fn(),
    approveDealRoomAccessRequest: vi.fn(),
    rejectDealRoomAccessRequest: vi.fn(),
  },
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

async function renderPanel() {
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
    },
    common: {
      loading: "Loading…",
      retry: "Retry",
    },
  });
  const view = render(
    <I18nextProvider i18n={i18nInstance}>
      <DealRoomAccessRequestsPanel roomId="room-1" />
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
    getDealRoomLinksMock.mockReset();
    getPendingLinkAccessRequestsMock.mockReset();
    approveLinkAccessRequestMock.mockReset();

    getDealRoomAccessRequestsMock.mockResolvedValue({ data: [] });
    getDealRoomLinksMock.mockResolvedValue({
      data: [{ id: "link-1", name: "测啊" }],
    });
    getPendingLinkAccessRequestsMock.mockResolvedValue({
      data: [
        {
          id: "req-1",
          link_id: "link-1",
          email: "visitor@example.com",
          reason: "need docs",
          signer_name: "Visitor",
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
    expect(getPendingLinkAccessRequestsMock).toHaveBeenCalled();
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
});
