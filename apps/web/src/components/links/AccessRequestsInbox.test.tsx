// @vitest-environment jsdom
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { AccessRequestsInbox } from "./AccessRequestsInbox";
import type { AccessRequestInboxItem } from "./AccessRequestsInbox";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

const request: AccessRequestInboxItem = {
  id: "req-1",
  link_id: "link-1",
  email: "visitor@example.com",
  status: "pending",
  created_at: "2026-08-05T00:00:00Z",
  updated_at: "2026-08-05T00:00:00Z",
};

describe("AccessRequestsInbox", () => {
  it("hides approve/reject unless canReview is true", () => {
    const { rerender } = render(
      <AccessRequestsInbox
        title="Inbox"
        description="Pending"
        requests={[request]}
        busyId={null}
        itemTestIdPrefix="item"
        onApprove={() => undefined}
        onReject={() => undefined}
      />,
    );
    expect(screen.queryByRole("button", { name: "accessRequests.approve" })).not.toBeInTheDocument();

    rerender(
      <AccessRequestsInbox
        title="Inbox"
        description="Pending"
        requests={[request]}
        busyId={null}
        itemTestIdPrefix="item"
        onApprove={() => undefined}
        onReject={() => undefined}
        canReview={false}
      />,
    );
    expect(screen.queryByRole("button", { name: "accessRequests.approve" })).not.toBeInTheDocument();

    rerender(
      <AccessRequestsInbox
        title="Inbox"
        description="Pending"
        requests={[request]}
        busyId={null}
        itemTestIdPrefix="item"
        onApprove={() => undefined}
        onReject={() => undefined}
        canReview
      />,
    );
    expect(screen.getByRole("button", { name: "accessRequests.approve" })).toBeInTheDocument();
  });
});
