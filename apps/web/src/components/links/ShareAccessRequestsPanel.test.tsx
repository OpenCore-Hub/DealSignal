// @vitest-environment jsdom
import { describe, expect, it, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { ShareAccessRequestsPanel } from "./ShareAccessRequestsPanel";

const mockGetPending = vi.fn();
const mockApprove = vi.fn();
const mockReject = vi.fn();

vi.mock("@/lib/api", () => ({
  api: {
    getPendingLinkAccessRequests: (...args: unknown[]) => mockGetPending(...args),
    approveLinkAccessRequest: (...args: unknown[]) => mockApprove(...args),
    rejectLinkAccessRequest: (...args: unknown[]) => mockReject(...args),
  },
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, opts?: { title?: string; name?: string; email?: string }) => {
      if (key === "links:accessRequests.forDocument") return `Document: ${opts?.title ?? ""}`;
      if (key === "linkShare:accessRequests.applicantWithName") {
        return `${opts?.name} · ${opts?.email}`;
      }
      return key;
    },
  }),
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

const pendingRequest = {
  id: "lar_1",
  link_id: "link_focus",
  email: "visitor@example.com",
  signer_name: "Visitor",
  reason: "Need access",
  status: "pending" as const,
  created_at: "2026-08-05T00:00:00Z",
  updated_at: "2026-08-05T00:00:00Z",
  document_title: "Roadmap.pdf",
};

function renderPanel(path = "/ws/documents?tab=shared") {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <ShareAccessRequestsPanel />
    </MemoryRouter>,
  );
}

describe("ShareAccessRequestsPanel", () => {
  beforeEach(() => {
    mockGetPending.mockReset();
    mockApprove.mockReset();
    mockReject.mockReset();
    mockGetPending.mockResolvedValue({ data: [pendingRequest] });
    mockApprove.mockResolvedValue({ data: { ...pendingRequest, status: "approved" } });
  });

  it("renders pending inbox and approves via link-scoped API", async () => {
    renderPanel();

    await waitFor(() => {
      expect(screen.getByTestId("share-access-requests-panel")).toBeInTheDocument();
    });
    expect(mockGetPending).toHaveBeenCalledWith({ scope: "document" });
    expect(screen.getByText("Document: Roadmap.pdf")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "accessRequests.approve" }));
    await waitFor(() => {
      expect(mockApprove).toHaveBeenCalledWith("link_focus", "lar_1");
    });
  });

  it("highlights the deep-linked linkId from the URL", async () => {
    renderPanel("/ws/documents?tab=shared&linkId=link_focus");

    await waitFor(() => {
      expect(screen.getByTestId("share-access-request-lar_1")).toHaveAttribute(
        "data-focused",
        "true",
      );
    });
  });

  it("filters by linkIds when provided", async () => {
    render(
      <MemoryRouter initialEntries={["/ws/documents?tab=shared"]}>
        <ShareAccessRequestsPanel linkIds={["other_link"]} />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(mockGetPending).toHaveBeenCalled();
    });
    expect(screen.queryByTestId("share-access-requests-panel")).not.toBeInTheDocument();
  });
});
