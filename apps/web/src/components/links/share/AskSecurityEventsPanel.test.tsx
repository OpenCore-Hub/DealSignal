// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { AskSecurityEventsPanel } from "./AskSecurityEventsPanel";
import { createTestI18n } from "@/i18n/test-utils";
import { ApiError } from "@/lib/apiClient";
import type { AskSecurityEvent } from "@/types";

const {
  listLinkAskSecurityEventsMock,
  listRoomAskSecurityEventsMock,
} = vi.hoisted(() => ({
  listLinkAskSecurityEventsMock: vi.fn(),
  listRoomAskSecurityEventsMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    listLinkAskSecurityEvents: listLinkAskSecurityEventsMock,
    listRoomAskSecurityEvents: listRoomAskSecurityEventsMock,
  },
}));

const securityI18n = {
  "askSecurityEvents.title": "Visitor Ask security events",
  "askSecurityEvents.description":
    "High-risk Ask Docs / Ask Host events: blocks, scope violations, and rate limits.",
  "askSecurityEvents.roomTitle": "Visitor Ask security events",
  "askSecurityEvents.roomDescription":
    "Room-wide high-risk Ask events. Filter by link.",
  "askSecurityEvents.loading": "Loading security events...",
  "askSecurityEvents.loadFailed": "Failed to load security events",
  "askSecurityEvents.forbidden":
    "You do not have permission to view Ask security events.",
  "askSecurityEvents.empty": "No high-risk Ask security events yet.",
  "askSecurityEvents.filterAllLinks": "All links",
  "askSecurityEvents.filterByLink": "Filter by link",
  "askSecurityEvents.anonymous": "Anonymous visitor",
  "askSecurityEvents.highRiskBadge": "High risk",
  "askSecurityEvents.reasonLabel": "Detail",
  "askSecurityEvents.eventTypes.rate_limit_exceeded": "Rate limit exceeded",
  "askSecurityEvents.eventTypes.scope_violation": "Scope violation",
  "askSecurityEvents.eventTypes.blocked_email": "Blocked email",
  "askSecurityEvents.eventTypes.blocked_domain": "Blocked domain",
  "askSecurityEvents.eventTypes.not_in_allow_list": "Removed from allowlist",
  "askSecurityEvents.reasons.ask_docs": "Ask Docs",
  "askSecurityEvents.reasons.ask_host": "Ask Host",
  "askSecurityEvents.reasons.out_of_scope_evidence": "Out-of-scope evidence",
};

function makeEvent(overrides: Partial<AskSecurityEvent> = {}): AskSecurityEvent {
  return {
    id: "ev-1",
    link_id: "link-1",
    event_type: "rate_limit_exceeded",
    visitor_id: "v1",
    email: "blocked@example.com",
    reason: "ask_docs",
    created_at: "2026-07-18T10:00:00Z",
    ...overrides,
  };
}

async function renderPanel(
  props:
    | { mode: "link"; linkId: string }
    | {
        mode: "room";
        roomId: string;
        links?: Array<{ id: string; name?: string }>;
      },
) {
  const i18n = await createTestI18n({ linkShare: securityI18n });
  return render(
    <I18nextProvider i18n={i18n}>
      <AskSecurityEventsPanel {...props} />
    </I18nextProvider>,
  );
}

describe("AskSecurityEventsPanel — link mode", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listLinkAskSecurityEventsMock.mockResolvedValue({ data: [makeEvent()] });
  });

  it("loads and renders high-risk events", async () => {
    listLinkAskSecurityEventsMock.mockResolvedValue({
      data: [
        makeEvent(),
        makeEvent({
          id: "ev-2",
          event_type: "not_in_allow_list",
          email: "removed@vc.com",
          reason: undefined,
        }),
      ],
    });
    await renderPanel({ mode: "link", linkId: "link-1" });
    await waitFor(() => {
      expect(listLinkAskSecurityEventsMock).toHaveBeenCalledWith("link-1");
    });
    expect(await screen.findByText("Rate limit exceeded")).toBeInTheDocument();
    expect(screen.getByText("Removed from allowlist")).toBeInTheDocument();
    expect(screen.getByText("blocked@example.com")).toBeInTheDocument();
    expect(screen.getByText("removed@vc.com")).toBeInTheDocument();
    expect(screen.getByText("Detail: Ask Docs")).toBeInTheDocument();
    expect(screen.getAllByText("High risk").length).toBeGreaterThanOrEqual(1);
  });

  it("shows empty state", async () => {
    listLinkAskSecurityEventsMock.mockResolvedValue({ data: [] });
    await renderPanel({ mode: "link", linkId: "link-1" });
    expect(
      await screen.findByText("No high-risk Ask security events yet."),
    ).toBeInTheDocument();
  });

  it("shows forbidden on 403", async () => {
    listLinkAskSecurityEventsMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "forbidden",
        message: "ask security events forbidden",
        requestId: "r1",
      }),
    );
    await renderPanel({ mode: "link", linkId: "link-1" });
    expect(
      await screen.findByText(
        "You do not have permission to view Ask security events.",
      ),
    ).toBeInTheDocument();
  });
});

describe("AskSecurityEventsPanel — room mode", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listRoomAskSecurityEventsMock.mockResolvedValue({
      data: [
        makeEvent({
          id: "ev-a",
          link_id: "link-a",
          event_type: "scope_violation",
          reason: "out_of_scope_evidence",
        }),
      ],
    });
  });

  it("loads room events and filters by link", async () => {
    await renderPanel({
      mode: "room",
      roomId: "room-1",
      links: [
        { id: "link-a", name: "Memo link" },
        { id: "link-b", name: "Deck link" },
      ],
    });
    await waitFor(() => {
      expect(listRoomAskSecurityEventsMock).toHaveBeenCalledWith("room-1", {
        linkId: undefined,
      });
    });
    expect(await screen.findByText("Scope violation")).toBeInTheDocument();
    expect(screen.getAllByText("Memo link").length).toBeGreaterThanOrEqual(1);

    fireEvent.change(screen.getByLabelText("Filter by link"), {
      target: { value: "link-b" },
    });
    await waitFor(() => {
      expect(listRoomAskSecurityEventsMock).toHaveBeenLastCalledWith("room-1", {
        linkId: "link-b",
      });
    });
  });
});
