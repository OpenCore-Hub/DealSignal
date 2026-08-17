// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act, within } from "@testing-library/react";
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
      name: "Investor share",
      title: "Investor share",
      documentTitle: "Q3 Pitch",
      documentId: "doc-1",
      shortUrl: "http://localhost:8080/l/abc",
      views: 12,
      score: 55,
      heatLevel: "warm",
    },
  ],
  scenarioPack: {
    scenario: "startup-fundraising",
    defaultCircle: "founder",
    depth: "p0",
    roomCount: 2,
    keyPageCategories: ["cap_table", "use_of_funds"],
    kpis: [
      { id: "active_rooms", value: 2 },
      { id: "gate_pending", value: 1 },
      { id: "key_page_views", value: 12 },
      { id: "open_signals", value: 3 },
    ],
  },
};

async function initI18n() {
  const instance = i18n.createInstance();
  const insightsJson = JSON.parse(
    readFileSync(resolve(__dirname, "../../i18n/locales/en/insights.json"), "utf-8"),
  );
  const commonJson = JSON.parse(
    readFileSync(resolve(__dirname, "../../i18n/locales/en/common.json"), "utf-8"),
  );
  const dashboardJson = JSON.parse(
    readFileSync(resolve(__dirname, "../../i18n/locales/en/dashboard.json"), "utf-8"),
  );
  // Match apps/web/src/i18n/config.ts — defaultNS is common, not insights.
  // Scenario KPI labels regress if exists()/lookups omit ns: "insights".
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["common", "insights", "dashboard"],
    defaultNS: "common",
    resources: {
      en: { insights: insightsJson, common: commonJson, dashboard: dashboardJson },
    },
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

    expect(screen.getByTestId("insights-scenario-pack")).toHaveAttribute(
      "data-scenario",
      "startup-fundraising",
    );
    const gateKpi = screen.getByTestId("insights-scenario-kpi-gate_pending");
    expect(gateKpi).toHaveTextContent("1");
    expect(gateKpi).toHaveTextContent(/Pending gates/i);
    expect(screen.getByTestId("insights-scenario-kpi-active_rooms")).toHaveTextContent(
      /Active rooms/i,
    );
    expect(screen.getByTestId("insights-scenario-kpi-key_page_views")).toHaveTextContent(
      /Key-page views/i,
    );
    expect(screen.getByTestId("insights-scenario-kpi-open_signals")).toHaveTextContent(
      /Open signals/i,
    );
    expect(within(screen.getByTestId("insights-scenario-pack")).queryByText(/^Metric$/)).toBeNull();
    expect(screen.getByText(/Startup fundraising/i)).toBeInTheDocument();

    expect(getInsightsOverviewMock).toHaveBeenCalledTimes(1);
    expect(getInsightsOverviewMock).toHaveBeenCalledWith({ days: 7 });
    expect(screen.getByText("4")).toBeInTheDocument();
    expect(screen.getAllByText("2").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("Q3 Pitch")).toBeInTheDocument();
    expect(screen.getByText("Investor share")).toBeInTheDocument();
    expect(screen.getByText("Document")).toBeInTheDocument();
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
    expect(screen.getByText(/Workspace members are excluded/i)).toBeInTheDocument();
    expect(screen.getByText(/Median dwell/i)).toBeInTheDocument();
    expect(screen.getByText(/48s/i)).toBeInTheDocument();
    expect(screen.getByText(/Completion rate/i)).toBeInTheDocument();
    expect(screen.getByText("50%")).toBeInTheDocument();
    expect(screen.getByText(/2 of 4 measurable sessions/i)).toBeInTheDocument();
    expect(
      screen.queryByText(/Rankings below use lifetime heat/i),
    ).not.toBeInTheDocument();
    expect(screen.getByText(/Opens & visitors/i)).toBeInTheDocument();
    expect(screen.getByText(/Updated /i)).toBeInTheDocument();
  });

  it("labels unnamed document shares with the absolute URL, not the short path or file title", async () => {
    getInsightsOverviewMock.mockResolvedValue({
      ...mockOverview,
      topLinks: [
        {
          id: "link-1",
          name: "",
          title: "",
          documentTitle: "CFI-Case-Study-Three-Statement.xlsx",
          documentId: "doc-1",
          shareKind: "document",
          shortUrl: "/l/abc",
          views: 12,
          score: 55,
          heatLevel: "warm",
        },
      ],
    });
    await renderPage();
    const longUrl = `${window.location.origin}/l/abc`;
    await waitFor(() => {
      expect(screen.getByText(longUrl)).toBeInTheDocument();
    });
    expect(screen.queryByText("CFI-Case-Study-Three-Statement.xlsx")).not.toBeInTheDocument();
    expect(screen.queryByText(/Share ·/)).not.toBeInTheDocument();
    expect(screen.queryByText(/^\/l\/abc$/)).not.toBeInTheDocument();
    expect(screen.getByText("Document")).toBeInTheDocument();
  });

  it("renders lite scenario pack depth, rules, and translated key-page categories", async () => {
    getInsightsOverviewMock.mockResolvedValue({
      ...mockOverview,
      scenarioPack: {
        scenario: "real-estate-transaction",
        label: "Real estate transaction",
        digestLead:
          "This week’s focus: unlock counterparty diligence, renew decaying access, and protect property materials.",
        defaultCircle: "founder",
        depth: "lite",
        roomCount: 1,
        keyPageCategories: ["title", "leases"],
        keyPageRules: [
          { category: "title", keywords: ["title report", "ownership"] },
          { category: "leases", keywords: ["rent roll", "lease"] },
        ],
        kpis: [
          { id: "active_rooms", value: 1 },
          { id: "gate_pending", value: 2 },
          { id: "key_page_views", value: 4 },
          { id: "open_signals", value: 1 },
        ],
      },
    });
    await renderPage();
    await waitFor(() => {
      expect(screen.getByTestId("insights-scenario-pack")).toHaveAttribute(
        "data-scenario",
        "real-estate-transaction",
      );
    });
    const pack = screen.getByTestId("insights-scenario-pack");
    expect(pack).toHaveAttribute("data-pack-depth", "lite");
    expect(pack).toHaveTextContent(/Lite depth/i);
    expect(pack).toHaveTextContent(/Same six Deal Radar products/i);
    expect(pack).toHaveTextContent(/lifetime heat/i);
    expect(pack).toHaveTextContent(/Title & ownership/i);
    expect(pack).toHaveTextContent(/Leases & tenancies/i);
    const gateKpi = screen.getByTestId("insights-scenario-kpi-gate_pending");
    expect(gateKpi).toHaveTextContent("2");
    expect(gateKpi).toHaveTextContent(/Pending gates/i);
    expect(screen.getByTestId("insights-scenario-key-page-rules")).toHaveTextContent(
      /Why these pages count as key pages/i,
    );
    expect(screen.getByTestId("insights-scenario-open-radar")).toBeInTheDocument();
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
    });
    const cta = screen.getByTestId("deal-radar-cta");
    expect(
      within(cta).getByRole("button", { name: /open deal radar/i }),
    ).toBeInTheDocument();
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
