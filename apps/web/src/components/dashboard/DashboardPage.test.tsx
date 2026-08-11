// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor, within } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter, Routes, Route } from "react-router";
import { DashboardPage } from "./DashboardPage";
import { createTestI18n } from "@/i18n/test-utils";
import type { RadarFeed, RadarWorkItem } from "@/lib/radarQueue";

const mockFns = vi.hoisted(() => ({
  getRadar: vi.fn(),
  getRadarEvidence: vi.fn(),
  updateRadarItem: vi.fn(),
  navigate: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: mockFns,
}));

vi.mock("sonner", () => ({
  toast: Object.assign(vi.fn(), { error: vi.fn(), message: vi.fn() }),
}));

vi.mock("react-router", async () => {
  const actual = await vi.importActual<typeof import("react-router")>("react-router");
  return {
    ...actual,
    useParams: () => ({ workspaceSlug: "acme" }),
    useNavigate: () => mockFns.navigate,
  };
});

function makeItem(over: Partial<RadarWorkItem> = {}): RadarWorkItem {
  return {
    id: "act-1",
    product: "diligence_gate",
    headline: "Approve access request from doc@example.com for Pitch",
    subtitle: "",
    verb: "approve",
    priority: "high",
    slaDueAt: "2026-06-21T18:00:00Z",
    createdAt: "2026-06-20T18:00:00Z",
    dealKey: "link:link-doc",
    dealName: "Pitch",
    actionId: "act-1",
    navigatePath: "/acme/documents?tab=shared&linkId=link-doc",
    evidencePath: "/acme/documents?tab=shared&linkId=link-doc",
    ...over,
  };
}

function makeFeed(items: RadarWorkItem[] = []): RadarFeed {
  return {
    nextUp: items[0] ?? null,
    strands: [],
    items,
    clearedToday: 0,
    counts: {
      all: items.length,
      buying_window: 0,
      diligence_gate: items.filter((i) => i.product === "diligence_gate").length,
      commitment_ask: 0,
      leak_watch: 0,
      access_decay: 0,
      abuse_guard: 0,
    },
    lens: "founder",
    defaultLens: "founder",
    lensSource: "default",
  };
}

async function renderPage(waitForLoad = true, entry = "/acme/dashboard") {
  const i18n = await createTestI18n({
    dashboard: {
      "radar.title": "Deal Radar",
      "radar.upNext": "Today's focus",
      "radar.openCount_one": "{{count}} open",
      "radar.openCount_other": "{{count}} open",
      "radar.filtersLabel": "Queue filters",
      "radar.filters.all": "All",
      "radar.filters.buying_window": "Hot intent",
      "radar.filters.diligence_gate": "Diligence gate",
      "radar.filters.commitment_ask": "Ask",
      "radar.filters.leak_watch": "Leak watch",
      "radar.filters.access_decay": "Access decay",
      "radar.filters.abuse_guard": "Abuse guard",
      "radar.products.diligence_gate": "Diligence gate",
      "radar.products.buying_window": "Hot intent",
      "radar.cta.approve": "Approve",
      "radar.cta.reply": "Reply",
      "radar.cta.email": "Email",
      "radar.cta.renew": "Renew",
      "radar.cta.review": "Review",
      "radar.cta.open": "Open",
      "radar.evidence": "Evidence",
      "radar.clearedToday": "Cleared today",
      "radar.analyzeInInsights": "Analyze in Insights",
      "radar.undo": "Undo",
      "radar.nextUpLabel": "Do this next",
      "radar.byDeal": "By deal",
      "radar.strandCount_one": "{{count}} item",
      "radar.strandCount_other": "{{count}} items",
      "radar.showMoreInStrand_one": "Show {{count}} more",
      "radar.showMoreInStrand_other": "Show {{count}} more",
      "radar.evidenceRail.title": "Evidence",
      "radar.evidenceRail.empty": "Select an item",
      "radar.evidenceRail.loading": "Loading",
      "radar.evidenceRail.error": "Error",
      "radar.evidenceRail.keyPages": "Key pages",
      "radar.evidenceRail.topPages": "Top pages",
      "radar.evidenceRail.recentVisitors": "Recent visitors",
      "radar.evidenceRail.securityEvents": "Security",
      "radar.evidenceRail.anonymousVisitor": "Anonymous",
      "radar.evidenceRail.page": "Page {{page}}",
      "radar.evidenceRail.views_one": "{{count}} view",
      "radar.evidenceRail.views_other": "{{count}} views",
      "radar.evidenceRail.openFull": "Open full evidence",
      "radar.evidenceRail.metrics.opens24h": "Opens",
      "radar.evidenceRail.metrics.visitors24h": "Visitors",
      "radar.evidenceRail.metrics.forwards24h": "Forwards",
      "radar.evidenceRail.metrics.downloads24h": "Downloads",
      "radar.evidenceRail.metrics.captures24h": "Captures",
      "radar.toast.done": "Marked done",
      "radar.toast.snoozed": "Snoozed",
      "radar.toast.ignored": "Ignored",
      "radar.toast.primaryUnavailable": "No open destination for this item",
      "radar.snoozeHours.24": "Snooze 1 day",
      "radar.snoozeHours.72": "Snooze 3 days",
      "radar.snoozeHours.168": "Snooze 7 days",
      "radar.lensLabel": "Role lens",
      "radar.lens.founder": "Founder",
      "radar.lens.investor_ir": "Investor IR",
      "radar.lens.sales": "Sales",
      "radar.confidence.low": "Low confidence",
      "radar.confidence.medium": "Medium confidence",
      "radar.confidence.high": "High confidence",
      "radar.outcome.choose": "Choose completion reason",
      "radar.outcome.acted": "Acted",
      "radar.outcome.false_positive": "False positive",
      "radar.outcome.renewed": "Renewed",
      "radar.outcome.approved": "Approved",
      "radar.outcome.replied": "Replied",
      "radar.outcome.other": "Other",
      "radar.inboxZero.title": "Lane clear for today",
      "radar.inboxZero.description": "Cleared {{count}}",
      "radar.noiseHint": "{{product}} demoted {{rate}}% / {{sample}}",
      "radar.feedDegraded": "Some ranking signals failed to load.",
      "radar.feedDegradedSections.internal_emails": "Member email filter",
      "radar.feedDegradedSections.capture_metrics": "Capture metrics",
      "radar.feedDegradedSections.ip_metrics": "IP cluster metrics",
      "radar.shortcuts.hint": "J/K move · E done · S snooze 1d",
      "radar.dealFallback": "Workspace",
      "radar.toast.undoFailed": "Could not undo",
      "radar.whyNow.diligence_gate": "Gate waiting",
      "radar.products.commitment_ask": "Ask",
      "radar.products.leak_watch": "Leak",
      "radar.products.access_decay": "Decay",
      "radar.products.abuse_guard": "Abuse",
      "empty.actions.title": "You're clear",
      "empty.actions.description": "No open items",
      "empty.actions.filteredTitle": "Nothing in this filter",
      "empty.actions.filteredDescription": "Try another filter",
      "empty.actions.createDealRoom": "Create data room",
      "actions.moreOptions": "More",
      "actions.ignore": "Ignore",
    },
    common: {
      retry: "Retry",
      back: "Back",
      complete: "Complete",
      dueDate: "Due",
      "overdue.days_one": "{{count}} day overdue",
      "overdue.days_other": "{{count}} days overdue",
    },
  });

  const view = render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter initialEntries={[entry]}>
        <Routes>
          <Route path="/:workspaceSlug/dashboard" element={<DashboardPage />} />
        </Routes>
      </MemoryRouter>
    </I18nextProvider>,
  );

  if (waitForLoad) {
    await waitFor(() => {
      const retry = screen.queryByRole("button", { name: /Retry/i });
      const queue = screen.queryByTestId("radar-queue");
      expect(retry || queue).toBeTruthy();
      expect(view.container.querySelector('[data-slot="skeleton"]')).toBeNull();
    });
  }
  return view;
}

beforeEach(() => {
  vi.clearAllMocks();
  mockFns.getRadar.mockResolvedValue(makeFeed());
  mockFns.getRadarEvidence.mockResolvedValue({
    itemId: "act-1",
    product: "diligence_gate",
    headline: "Approve",
    metrics: {
      opens24h: 2,
      uniqueVisitors24h: 1,
      forwardSignals24h: 0,
      downloads24h: 0,
    },
  });
  mockFns.updateRadarItem.mockResolvedValue({});
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });
});

describe("DashboardPage inbox", () => {
  it("shows loading skeleton initially", async () => {
    mockFns.getRadar.mockReturnValue(new Promise(() => {}));
    const { container } = await renderPage(false);
    expect(container.querySelectorAll('[data-slot="skeleton"]').length).toBeGreaterThan(0);
  });

  it("renders inbox queue without analysis collage or room sidebar", async () => {
    await renderPage();
    expect(screen.getByTestId("radar-queue")).toBeInTheDocument();
    expect(screen.getByText("Today's focus")).toBeInTheDocument();
    expect(screen.getByTestId("radar-evidence-rail")).toBeInTheDocument();
    expect(screen.getByTestId("radar-insights-link")).toBeInTheDocument();
    expect(screen.queryByText("Rooms needing you")).not.toBeInTheDocument();
    expect(screen.queryByText("Heat map")).not.toBeInTheDocument();
    expect(screen.queryByText("Activity timeline")).not.toBeInTheDocument();
    expect(screen.queryByText("Welcome back")).not.toBeInTheDocument();
  });

  it("renders closed-loop noise hint from feed", async () => {
    mockFns.getRadar.mockResolvedValue({
      ...makeFeed([makeItem({ product: "leak_watch", id: "act-leak", actionId: "act-leak" })]),
      noiseHints: [
        {
          product: "leak_watch",
          falsePositiveRate: 0.4,
          sample: 5,
          demoteBoost: 2,
        },
      ],
    });
    await renderPage();
    await waitFor(() => {
      expect(screen.getByTestId("radar-noise-hints")).toBeInTheDocument();
    });
    expect(screen.getByTestId("radar-noise-hints")).toHaveTextContent(/Leak/i);
  });

  it("shows next-up card when items exist", async () => {
    mockFns.getRadar.mockResolvedValue(makeFeed([makeItem()]));
    await renderPage();
    await waitFor(() => {
      expect(screen.getByTestId("radar-next-up")).toBeInTheDocument();
    });
    expect(screen.getByText("Do this next")).toBeInTheDocument();
  });

  it("routes the empty-state action to the create data room page", async () => {
    await renderPage();
    fireEvent.click(
      await screen.findByRole("button", { name: "Create data room" }),
    );
    expect(mockFns.navigate).toHaveBeenCalledWith("/acme/deal-rooms/new");
  });

  it("shows error state and allows retry", async () => {
    mockFns.getRadar.mockRejectedValue(new Error("Network error"));
    await renderPage();
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /Retry/i })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("button", { name: /Retry/i }));
    await waitFor(() => {
      expect(mockFns.getRadar).toHaveBeenCalledTimes(2);
    });
  });

  it("uses server navigatePath for diligence gate items", async () => {
    mockFns.getRadar.mockResolvedValue(makeFeed([makeItem()]));

    await renderPage();
    const nextUp = await screen.findByTestId("radar-next-up");
    expect(nextUp).toHaveTextContent("doc@example.com");

    fireEvent.click(
      within(nextUp).getByRole("button", { name: /^Approve$/i }),
    );
    expect(mockFns.navigate).toHaveBeenCalled();
    const firstPath = mockFns.navigate.mock.calls[0][0] as string;
    expect(firstPath).toContain("tab=shared");
    expect(firstPath).toContain("linkId=link-doc");
  });

  it("toasts when primary CTA has no destination", async () => {
    const { toast } = await import("sonner");
    mockFns.getRadar.mockResolvedValue(
      makeFeed([
        makeItem({
          navigatePath: undefined,
          evidencePath: undefined,
        }),
      ]),
    );
    await renderPage();
    const nextUp = await screen.findByTestId("radar-next-up");
    fireEvent.click(within(nextUp).getByRole("button", { name: /^Approve$/i }));
    expect(mockFns.navigate).not.toHaveBeenCalled();
    expect(toast.message).toHaveBeenCalledWith(
      "No open destination for this item",
    );
  });

  it("hides Email primary CTA for Hot intent and never opens mailto", async () => {
    const openSpy = vi.spyOn(window, "open").mockImplementation(() => null);
    const item = makeItem({
      id: "act-hot",
      actionId: "act-hot",
      product: "buying_window",
      verb: "email",
      headline: "Follow up with buyer",
      contactEmail: "buyer@acme.test",
      navigatePath: "/acme/links/l1",
      evidencePath: "/acme/links/l1",
    });
    mockFns.getRadar.mockResolvedValue(
      makeFeed([item, makeItem({ id: "act-2", actionId: "act-2" })]),
    );

    await renderPage();
    const nextUp = await screen.findByTestId("radar-next-up");

    expect(
      within(nextUp).queryByRole("button", { name: /^Email$/i }),
    ).not.toBeInTheDocument();
    expect(
      within(nextUp).getByRole("button", { name: /Evidence/i }),
    ).toBeInTheDocument();

    // Row body selects only — no navigate / mailto.
    fireEvent.click(within(nextUp).getByText("Follow up with buyer"));
    expect(mockFns.navigate).not.toHaveBeenCalled();
    expect(openSpy).not.toHaveBeenCalled();
    expect(nextUp.querySelector('[data-radar-selected="true"]')).toBeTruthy();

    openSpy.mockRestore();
  });

  it("refetches after direct complete on buying_window", async () => {
    const item = makeItem({
      id: "act-buy",
      actionId: "act-buy",
      product: "buying_window",
      verb: "email",
      headline: "Follow up with buyer",
      navigatePath: "/acme/links/l1",
      evidencePath: "/acme/links/l1",
    });
    mockFns.getRadar
      .mockResolvedValueOnce(makeFeed([item]))
      .mockResolvedValueOnce(makeFeed([]));

    await renderPage();
    await screen.findByTestId("radar-next-up");

    fireEvent.click(screen.getAllByRole("button", { name: /Complete/i })[0]);

    await waitFor(() => {
      expect(mockFns.updateRadarItem).toHaveBeenCalledWith(
        "act-buy",
        "done",
        undefined,
        "acted",
      );
    });
    await waitFor(() => {
      expect(mockFns.getRadar.mock.calls.length).toBeGreaterThanOrEqual(2);
    });
  });

  it("renders feed degradedSections honesty banner", async () => {
    mockFns.getRadar.mockResolvedValue({
      ...makeFeed([makeItem()]),
      degradedSections: ["internal_emails", "capture_metrics"],
    });
    await renderPage();
    await waitFor(() => {
      expect(screen.getByTestId("radar-feed-degraded")).toBeInTheDocument();
    });
    expect(screen.getByTestId("radar-feed-degraded")).toHaveTextContent(
      /Member email filter/i,
    );
  });

  it("rolls back optimistic clear when PATCH returns not_found", async () => {
    const { toast } = await import("sonner");
    const { ApiError } = await import("@/lib/apiClient");
    const item = makeItem({
      id: "act-gone",
      actionId: "act-gone",
      product: "buying_window",
      verb: "email",
      headline: "Follow up gone buyer",
      navigatePath: "/acme/links/l1",
      evidencePath: "/acme/links/l1",
    });
    mockFns.getRadar.mockResolvedValue(makeFeed([item]));
    mockFns.updateRadarItem.mockRejectedValue(
      new ApiError({
        status: 404,
        code: "not_found",
        message: "radar item not found",
        requestId: "test",
      }),
    );

    await renderPage();
    await screen.findByTestId("radar-next-up");
    fireEvent.click(screen.getAllByRole("button", { name: /Complete/i })[0]);

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalled();
    });
    // Item restored after rollback (still in next-up / queue).
    expect(screen.getByTestId("radar-next-up")).toHaveTextContent(
      "Follow up gone buyer",
    );
  });
});
