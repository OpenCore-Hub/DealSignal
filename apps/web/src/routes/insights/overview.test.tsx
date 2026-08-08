// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { InsightsOverviewPage } from "./overview";
import type { InsightsOverview } from "@/lib/api";

const __dirname = dirname(fileURLToPath(import.meta.url));

const { getInsightsOverviewMock } = vi.hoisted(() => ({
  getInsightsOverviewMock: vi.fn(),
}));

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    api: {
      ...actual.api,
      getInsightsOverview: getInsightsOverviewMock,
    },
  };
});

const today = new Date();
today.setUTCHours(0, 0, 0, 0);

const mockOverview: InsightsOverview = {
  tierEntity: "link",
  tierCounts: { hot: 2, warm: 3, cold: 5 },
  activeLinkCount: 10,
  rangeDays: 7,
  generatedAt: today.toISOString(),
  eventRetentionDays: 90,
  pageViewRetentionDays: 90,
  periodOpens: 4,
  previousPeriodOpens: 2,
  periodUniqueVisitors: 2,
  previousPeriodUniqueVisitors: 1,
  periodMedianDurationSeconds: 48,
  previousPeriodMedianDurationSeconds: 24,
  periodAvgDurationSeconds: 60,
  periodPageViewCount: 6,
  periodSessionCount: 4,
  periodMeasurableSessions: 4,
  periodCompletedSessions: 2,
  periodCompletionRate: 0.5,
  previousPeriodSessionCount: 2,
  previousPeriodCompletedSessions: 1,
  previousPeriodCompletionRate: 0.5,
  openSignalCount: 3,
  dailyVisits: Array.from({ length: 7 }, (_, i) => {
    const d = new Date(today);
    d.setUTCDate(d.getUTCDate() - (6 - i));
    return {
      date: d.toISOString(),
      opens: i === 6 ? 4 : 0,
      uniqueVisitors: i === 6 ? 2 : 0,
    };
  }),
  topDocuments: [
    {
      id: "doc-1",
      title: "Q3 Pitch",
      views: 42,
      score: 80,
      heatLevel: "hot",
      primaryLinkId: "link-1",
    },
  ],
  topLinks: [
    {
      id: "link-1",
      title: "Q3 Pitch",
      documentId: "doc-1",
      shortUrl: "http://localhost:8080/l/abc",
      views: 12,
      score: 55,
      heatLevel: "warm",
    },
  ],
};

async function initI18n() {
  const instance = i18n.createInstance();
  const insightsJson = JSON.parse(
    readFileSync(resolve(__dirname, "../../i18n/locales/en/insights.json"), "utf-8"),
  );
  const commonJson = JSON.parse(
    readFileSync(resolve(__dirname, "../../i18n/locales/en/common.json"), "utf-8"),
  );
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["insights", "common"],
    defaultNS: "insights",
    resources: { en: { insights: insightsJson, common: commonJson } },
    interpolation: { escapeValue: false },
  });
  return instance;
}

async function renderPage() {
  const i18nInstance = await initI18n();
  let result!: ReturnType<typeof render>;
  await act(async () => {
    result = render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter initialEntries={["/acme/insights/overview"]}>
          <Routes>
            <Route path=":workspaceSlug/insights/overview" element={<InsightsOverviewPage />} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>,
    );
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  return result;
}

describe("InsightsOverviewPage", () => {
  beforeEach(() => {
    getInsightsOverviewMock.mockReset();
    getInsightsOverviewMock.mockResolvedValue(mockOverview);
  });

  it("renders overview stats and top lists without N+1 access-log fetches", async () => {
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText(/4 opens · 2 unique visitors in 7d/i)).toBeInTheDocument();
    });

    expect(getInsightsOverviewMock).toHaveBeenCalledTimes(1);
    expect(getInsightsOverviewMock).toHaveBeenCalledWith({ days: 7 });
    expect(screen.getByText("4")).toBeInTheDocument();
    expect(screen.getAllByText("2").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("Q3 Pitch").length).toBeGreaterThanOrEqual(2);
    // Primary label is document title — not localhost URL as visible text.
    expect(screen.queryByText(/localhost/i)).not.toBeInTheDocument();
    // Contact action list lives on Deal Radar — Insights only shows a CTA.
    expect(screen.getByTestId("deal-radar-cta")).toBeInTheDocument();
    expect(screen.getByText(/3 open signals on Deal Radar/i)).toBeInTheDocument();
    expect(screen.getByText(/Active links/i)).toBeInTheDocument();
    expect(screen.getAllByText(/Based on 10 share links/i).length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText(/\+100% vs prior 7d/i).length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText(/Link opens/i).length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText(/Unique visitors/i).length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText(/Median dwell/i)).toBeInTheDocument();
    expect(screen.getByText(/48s/i)).toBeInTheDocument();
    expect(screen.getByText(/Completion rate/i)).toBeInTheDocument();
    expect(screen.getByText("50%")).toBeInTheDocument();
    expect(screen.getByText(/2 of 4 measurable sessions/i)).toBeInTheDocument();
    expect(screen.getByText(/lifetime heat/i)).toBeInTheDocument();
    expect(screen.getByText(/Opens & visitors/i)).toBeInTheDocument();
    expect(screen.getByText(/Updated /i)).toBeInTheDocument();
  });

  it("refetches when trend range changes", async () => {
    getInsightsOverviewMock.mockImplementation(async (params: number | { days?: number; from?: string; to?: string } = 7) => {
      const days =
        typeof params === "number" ? params : (params.from ? 14 : (params.days ?? 7));
      return {
        ...mockOverview,
        rangeDays: days === 30 || days === 90 ? days : 7,
        rangeCustom: false,
        periodOpens: days === 30 ? 12 : 4,
        previousPeriodOpens: days === 30 ? 6 : 2,
        dailyVisits: Array.from({ length: days === 30 || days === 90 ? days : 7 }, (_, i) => {
          const d = new Date(today);
          const len = days === 30 || days === 90 ? days : 7;
          d.setUTCDate(d.getUTCDate() - (len - 1 - i));
          return { date: d.toISOString(), opens: i === len - 1 ? 4 : 0, uniqueVisitors: 1 };
        }),
      };
    });
    await renderPage();
    await waitFor(() => {
      expect(getInsightsOverviewMock).toHaveBeenCalledWith({ days: 7 });
    });

    fireEvent.click(screen.getByRole("button", { name: /^30d$/i }));

    await waitFor(() => {
      expect(getInsightsOverviewMock).toHaveBeenCalledWith({ days: 30 });
    });
    expect(screen.getByText(/Opens & visitors · last 30 days/i)).toBeInTheDocument();
  });

  it("applies a custom UTC from/to window", async () => {
    getInsightsOverviewMock.mockImplementation(async (params: number | { days?: number; from?: string; to?: string } = 7) => {
      if (typeof params === "object" && params.from && params.to) {
        return {
          ...mockOverview,
          rangeDays: 14,
          rangeFrom: params.from,
          rangeTo: params.to,
          rangeCustom: true,
          periodOpens: 9,
          previousPeriodOpens: 3,
          dailyVisits: Array.from({ length: 14 }, (_, i) => {
            const d = new Date(`${params.from}T00:00:00Z`);
            d.setUTCDate(d.getUTCDate() + i);
            return { date: d.toISOString(), opens: i === 13 ? 9 : 0, uniqueVisitors: 1 };
          }),
        };
      }
      return mockOverview;
    });
    await renderPage();
    await waitFor(() => {
      expect(getInsightsOverviewMock).toHaveBeenCalledWith({ days: 7 });
    });

    fireEvent.click(screen.getByRole("button", { name: /^custom$/i }));
    fireEvent.change(screen.getByLabelText(/^from$/i), { target: { value: "2026-07-01" } });
    fireEvent.change(screen.getByLabelText(/^to$/i), { target: { value: "2026-07-14" } });
    fireEvent.click(screen.getByRole("button", { name: /^apply$/i }));

    await waitFor(() => {
      expect(getInsightsOverviewMock).toHaveBeenCalledWith({ from: "2026-07-01", to: "2026-07-14" });
    });
    expect(screen.getByText(/Opens & visitors · 2026-07-01 – 2026-07-14/i)).toBeInTheDocument();
  });

  it("shows empty trend when dailyVisits have no opens", async () => {
    getInsightsOverviewMock.mockResolvedValue({
      ...mockOverview,
      periodOpens: 0,
      previousPeriodOpens: 0,
      periodUniqueVisitors: 0,
      previousPeriodUniqueVisitors: 0,
      periodMedianDurationSeconds: 0,
      periodSessionCount: 0,
      periodMeasurableSessions: 0,
      periodCompletedSessions: 0,
      periodCompletionRate: 0,
      previousPeriodSessionCount: 0,
      previousPeriodCompletedSessions: 0,
      previousPeriodCompletionRate: 0,
      dailyVisits: mockOverview.dailyVisits.map((d) => ({ ...d, opens: 0, uniqueVisitors: 0 })),
    });
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText(/No link opens yet/i)).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: /export csv/i })).toBeDisabled();
  });

  it("offers explainable heat breakdown for top links and documents", async () => {
    await renderPage();

    await waitFor(() => {
      expect(screen.getAllByRole("button", { name: /explain/i }).length).toBeGreaterThanOrEqual(2);
    });
    expect(screen.getByText(/55 pts/i)).toBeInTheDocument();
    expect(screen.getByText(/80 pts/i)).toBeInTheDocument();
  });

  it("discloses event retention days from the API", async () => {
    await renderPage();
    await waitFor(() => {
      expect(
        screen.getByText(/Event history is retained for 90 days \(opens\) and 90 days \(page views\)/i),
      ).toBeInTheDocument();
    });
  });

  it("routes follow-ups to Deal Radar with real open-signal counts", async () => {
    getInsightsOverviewMock.mockResolvedValue({
      ...mockOverview,
      openSignalCount: 4,
    });
    await renderPage();

    await waitFor(() => {
      expect(screen.getByTestId("deal-radar-cta")).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /open deal radar/i })).toBeInTheDocument();
    });
    expect(screen.getByText(/Follow-ups live on Deal Radar/i)).toBeInTheDocument();
    expect(screen.getByText(/4 open signals on Deal Radar/i)).toBeInTheDocument();
    // No duplicated contact rows on Insights — action feed stays on Deal Radar.
    expect(screen.queryByText("a@example.com")).not.toBeInTheDocument();
  });

  it("shows sample-insufficient KPI empty states without hard-coded dashes", async () => {
    getInsightsOverviewMock.mockResolvedValue({
      ...mockOverview,
      periodOpens: 2,
      periodUniqueVisitors: 1,
      periodMedianDurationSeconds: 0,
      previousPeriodMedianDurationSeconds: 0,
      periodPageViewCount: 0,
      periodMeasurableSessions: 0,
      periodCompletedSessions: 0,
      periodCompletionRate: 0,
    });
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText(/Needs page-view samples in this window/i)).toBeInTheDocument();
    });
    expect(screen.getByText(/Needs documents with known page counts/i)).toBeInTheDocument();
    expect(screen.getAllByText("—").length).toBeGreaterThanOrEqual(2);
  });

  it("shows error and retries on failure", async () => {
    getInsightsOverviewMock.mockRejectedValue(new Error("network error"));
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText(/Failed to load/i)).toBeInTheDocument();
    });

    getInsightsOverviewMock.mockResolvedValue(mockOverview);
    fireEvent.click(screen.getByRole("button", { name: /retry/i }));

    await waitFor(() => {
      expect(screen.getAllByText("Q3 Pitch").length).toBeGreaterThanOrEqual(1);
    });
  });
});
