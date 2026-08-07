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
    "High-risk Ask events: blocks, scope violations, and rate limits.",
  "askSecurityEvents.roomTitle": "Visitor Ask security events",
  "askSecurityEvents.roomDescription":
    "Room-wide high-risk Ask events. Filter by link.",
  "askSecurityEvents.loading": "Loading security events...",
  "askSecurityEvents.loadFailed": "Failed to load security events",
  "askSecurityEvents.loadMore": "Load more",
  "askSecurityEvents.loadingMore": "Loading more…",
  "askSecurityEvents.forbidden":
    "You do not have permission to view Ask security events.",
  "askSecurityEvents.empty": "No high-risk Ask security events yet.",
  "askSecurityEvents.filterAllLinks": "All links",
  "askSecurityEvents.filterByLink": "Filter by link",
  "askSecurityEvents.filterByTime": "Time range",
  "askSecurityEvents.filterAllTime": "All time",
  "askSecurityEvents.filterLast24h": "Last 24 hours",
  "askSecurityEvents.filterLast7d": "Last 7 days",
  "askSecurityEvents.filterLast30d": "Last 30 days",
  "askSecurityEvents.filterByEventType": "Event type",
  "askSecurityEvents.filterAllEventTypes": "All event types",
  "askSecurityEvents.anonymous": "Anonymous visitor",
  "askSecurityEvents.highRiskBadge": "High risk",
  "askSecurityEvents.reasonLabel": "Detail",
  "askSecurityEvents.eventTypes.rate_limit_exceeded": "Rate limit exceeded",
  "askSecurityEvents.eventTypes.scope_violation": "Scope violation",
  "askSecurityEvents.eventTypes.blocked_email": "Blocked email",
  "askSecurityEvents.eventTypes.blocked_domain": "Blocked domain",
  "askSecurityEvents.eventTypes.not_in_allow_list": "Removed from allowlist",
  "askSecurityEvents.reasons.ask_host": "Ask",
  "askSecurityEvents.reasons.out_of_scope_evidence": "Out-of-scope evidence",
};

function makeEvent(overrides: Partial<AskSecurityEvent> = {}): AskSecurityEvent {
  return {
    id: "ev-1",
    link_id: "link-1",
    event_type: "rate_limit_exceeded",
    visitor_id: "v1",
    email: "blocked@example.com",
    reason: "ask_host",
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
  const i18n = await createTestI18n({
    linkShare: securityI18n,
    common: { retry: "Retry" },
  });
  return render(
    <I18nextProvider i18n={i18n}>
      <AskSecurityEventsPanel {...props} />
    </I18nextProvider>,
  );
}

describe("AskSecurityEventsPanel — link mode", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listLinkAskSecurityEventsMock.mockResolvedValue({
      data: [makeEvent()],
      has_more: false,
    });
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
      has_more: false,
    });
    await renderPanel({ mode: "link", linkId: "link-1" });
    await waitFor(() => {
      expect(listLinkAskSecurityEventsMock).toHaveBeenCalledWith("link-1", {
        eventType: undefined,
        since: undefined,
        limit: 20,
        offset: 0,
      });
    });
    expect(
      await screen.findAllByText("Rate limit exceeded"),
    ).not.toHaveLength(0);
    expect(screen.getAllByText("Removed from allowlist").length).toBeGreaterThan(0);
  });

  it("filters by event type and time range", async () => {
    await renderPanel({ mode: "link", linkId: "link-1" });
    await waitFor(() => {
      expect(listLinkAskSecurityEventsMock).toHaveBeenCalled();
    });

    fireEvent.change(screen.getByLabelText("Event type"), {
      target: { value: "scope_violation" },
    });
    await waitFor(() => {
      expect(listLinkAskSecurityEventsMock).toHaveBeenLastCalledWith("link-1", {
        eventType: "scope_violation",
        since: undefined,
        limit: 20,
        offset: 0,
      });
    });

    const before = Date.now();
    fireEvent.change(screen.getByLabelText("Time range"), {
      target: { value: "7d" },
    });
    await waitFor(() => {
      const lastCall = listLinkAskSecurityEventsMock.mock.calls.at(-1);
      expect(lastCall?.[0]).toBe("link-1");
      expect(lastCall?.[1]).toMatchObject({
        eventType: "scope_violation",
        limit: 20,
        offset: 0,
      });
      const since = Date.parse(String(lastCall?.[1]?.since));
      expect(Number.isFinite(since)).toBe(true);
      const ageMs = before - since;
      expect(ageMs).toBeGreaterThanOrEqual(6.9 * 24 * 60 * 60 * 1000);
      expect(ageMs).toBeLessThanOrEqual(7.1 * 24 * 60 * 60 * 1000);
    });
  });

  it("shows empty state", async () => {
    listLinkAskSecurityEventsMock.mockResolvedValue({ data: [], has_more: false });
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

  it("retries after a generic load failure", async () => {
    listLinkAskSecurityEventsMock
      .mockRejectedValueOnce(
        new ApiError({
          status: 500,
          code: "internal_error",
          message: "boom",
          requestId: "r2",
        }),
      )
      .mockResolvedValueOnce({ data: [makeEvent()], has_more: false });

    await renderPanel({ mode: "link", linkId: "link-1" });
    expect(
      await screen.findByText("Failed to load security events"),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByTestId("ask-security-event-row")).toBeInTheDocument();
    expect(listLinkAskSecurityEventsMock).toHaveBeenCalledTimes(2);
  });

  it("loads more when has_more is true", async () => {
    listLinkAskSecurityEventsMock
      .mockResolvedValueOnce({
        data: [makeEvent({ id: "ev-1" })],
        has_more: true,
      })
      .mockResolvedValueOnce({
        data: [
          makeEvent({
            id: "ev-2",
            event_type: "blocked_email",
            email: "more@example.com",
          }),
        ],
        has_more: false,
      });

    await renderPanel({ mode: "link", linkId: "link-1" });
    expect(await screen.findByTestId("ask-security-event-row")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("ask-security-events-load-more"));
    await waitFor(() => {
      expect(screen.getAllByTestId("ask-security-event-row")).toHaveLength(2);
    });
    expect(listLinkAskSecurityEventsMock).toHaveBeenLastCalledWith("link-1", {
      eventType: undefined,
      since: undefined,
      limit: 20,
      offset: 1,
    });
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
      has_more: false,
    });
  });

  it("loads room events and filters by link, type, and time", async () => {
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
        eventType: undefined,
        since: undefined,
        limit: 20,
        offset: 0,
      });
    });
    expect(await screen.findByTestId("ask-security-event-row")).toBeInTheDocument();
    expect(screen.getAllByText("Scope violation").length).toBeGreaterThan(0);

    fireEvent.change(screen.getByLabelText("Filter by link"), {
      target: { value: "link-b" },
    });
    await waitFor(() => {
      expect(listRoomAskSecurityEventsMock).toHaveBeenLastCalledWith("room-1", {
        linkId: "link-b",
        eventType: undefined,
        since: undefined,
        limit: 20,
        offset: 0,
      });
    });

    fireEvent.change(screen.getByLabelText("Event type"), {
      target: { value: "rate_limit_exceeded" },
    });
    await waitFor(() => {
      expect(listRoomAskSecurityEventsMock).toHaveBeenLastCalledWith("room-1", {
        linkId: "link-b",
        eventType: "rate_limit_exceeded",
        since: undefined,
        limit: 20,
        offset: 0,
      });
    });

    const before = Date.now();
    fireEvent.change(screen.getByLabelText("Time range"), {
      target: { value: "24h" },
    });
    await waitFor(() => {
      const last = listRoomAskSecurityEventsMock.mock.calls.at(-1)?.[1];
      expect(last).toMatchObject({
        linkId: "link-b",
        eventType: "rate_limit_exceeded",
        limit: 20,
        offset: 0,
      });
      const since = Date.parse(String(last?.since));
      expect(Number.isFinite(since)).toBe(true);
      expect(before - since).toBeGreaterThanOrEqual(23 * 60 * 60 * 1000);
      expect(before - since).toBeLessThanOrEqual(25 * 60 * 60 * 1000);
    });
  });
});
