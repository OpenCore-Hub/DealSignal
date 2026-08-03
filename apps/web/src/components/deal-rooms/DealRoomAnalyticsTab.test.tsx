// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { MemoryRouter } from "react-router";
import { api } from "@/lib/api";
import {
  DealRoomAnalyticsTab,
  buildDealRoomTrendSeries,
} from "./DealRoomAnalyticsTab";

const i18nInstance = i18n.createInstance();
i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: {
    en: {
      dealRooms: {
        activity: {
          documents: "Documents",
          noVisitors: "No visitors yet",
        },
        analytics: {
          loadFailed: "Failed to load analytics",
          trendEmptyTitle: "No view trend yet",
          trendEmpty: "No view trend data yet for this deal room.",
          views: "Views",
          activeLinks: "Active links",
          uniqueVisitors: "Unique visitors",
          recentVisitors: "Recent visitors",
          visitorViews_one: "{{count}} view",
          visitorViews_other: "{{count}} views",
          visitorLastSeen: "Last seen {{time}}",
          trend: "View trend",
        },
      },
      common: {
        loading: "Loading...",
        retry: "Retry",
        trendEmptyTitle: "No trend data yet",
        start: "Start",
        now: "Now",
        empty: { description: "No data" },
      },
      linkShare: {
        askSecurity: {
          title: "Security events",
          empty: "No events",
        },
      },
    },
  },
  interpolation: { escapeValue: false },
});

vi.mock("@/lib/api", () => ({
  api: {
    getDealRoomAnalytics: vi.fn(),
  },
}));

vi.mock("@/components/links/share/AskSecurityEventsPanel", () => ({
  AskSecurityEventsPanel: () => <div data-testid="ask-security-events-panel" />,
}));

describe("buildDealRoomTrendSeries", () => {
  it("returns a zero-filled 30-day window when there are no daily buckets", () => {
    const now = new Date("2026-08-02T12:00:00.000Z");
    const series = buildDealRoomTrendSeries([], now);
    expect(series.data).toHaveLength(30);
    expect(series.labels).toHaveLength(30);
    expect(series.data.every((v) => v === 0)).toBe(true);
    expect(series.labels[0]).toBe("2026-07-04");
    expect(series.labels[series.labels.length - 1]).toBe("2026-08-02");
  });

  it("fills the last 30 UTC days and keeps sparse views", () => {
    const now = new Date("2026-08-02T12:00:00.000Z");
    const series = buildDealRoomTrendSeries([{ day: "2026-08-02", views: 4 }], now);
    expect(series.data).toHaveLength(30);
    expect(series.labels[series.labels.length - 1]).toBe("2026-08-02");
    expect(series.data[series.data.length - 1]).toBe(4);
    expect(series.data.slice(0, -1).every((v) => v === 0)).toBe(true);
  });
});

describe("DealRoomAnalyticsTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders analytics cards from the room analytics API", async () => {
    vi.mocked(api.getDealRoomAnalytics).mockResolvedValue({
      totalViews: 12,
      uniqueVisitors: 3,
      activeLinkCount: 2,
      documentCount: 5,
      viewsOverTime: [{ day: "2026-08-02", views: 12 }],
      recentVisitors: [
        {
          visitorId: "v1",
          visitorEmail: "alice@example.com",
          firstAccessAt: "2026-08-01T00:00:00Z",
          lastAccessAt: "2026-08-02T00:00:00Z",
          totalViews: 4,
        },
      ],
    });

    render(
      <MemoryRouter>
        <I18nextProvider i18n={i18nInstance}>
          <DealRoomAnalyticsTab roomId="room-1" />
        </I18nextProvider>
      </MemoryRouter>,
    );

    expect(await screen.findByTestId("deal-room-analytics-tab")).toBeInTheDocument();
    expect(screen.getByText("12")).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
    expect(screen.getByText("alice@example.com")).toBeInTheDocument();
    expect(api.getDealRoomAnalytics).toHaveBeenCalledWith("room-1");
  });

  it("shows a retryable error when analytics fails to load", async () => {
    vi.mocked(api.getDealRoomAnalytics).mockRejectedValue(new Error("boom"));

    render(
      <MemoryRouter>
        <I18nextProvider i18n={i18nInstance}>
          <DealRoomAnalyticsTab roomId="room-1" />
        </I18nextProvider>
      </MemoryRouter>,
    );

    expect(await screen.findByText("Failed to load analytics")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
  });

  it("shows empty visitor copy when there are no visitors", async () => {
    vi.mocked(api.getDealRoomAnalytics).mockResolvedValue({
      totalViews: 0,
      uniqueVisitors: 0,
      activeLinkCount: 1,
      documentCount: 1,
      viewsOverTime: [],
      recentVisitors: [],
    });

    render(
      <MemoryRouter>
        <I18nextProvider i18n={i18nInstance}>
          <DealRoomAnalyticsTab roomId="room-1" />
        </I18nextProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText("No visitors yet")).toBeInTheDocument();
    });
    expect(screen.getByText("No view trend yet")).toBeInTheDocument();
  });

  it("keeps a zero-filled trend when lifetime views exist outside the window", async () => {
    vi.mocked(api.getDealRoomAnalytics).mockResolvedValue({
      totalViews: 9,
      uniqueVisitors: 2,
      activeLinkCount: 1,
      documentCount: 1,
      viewsOverTime: [],
      recentVisitors: [],
    });

    render(
      <MemoryRouter>
        <I18nextProvider i18n={i18nInstance}>
          <DealRoomAnalyticsTab roomId="room-1" />
        </I18nextProvider>
      </MemoryRouter>,
    );

    expect(await screen.findByTestId("deal-room-analytics-tab")).toBeInTheDocument();
    expect(screen.getByText("9")).toBeInTheDocument();
    expect(screen.queryByText("No view trend yet")).not.toBeInTheDocument();
  });
});
