// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter, Routes, Route } from "react-router";
import { DashboardPage } from "./DashboardPage";
import { createTestI18n } from "@/i18n/test-utils";
import type { DashboardStats, InsightsOverview } from "@/lib/api";
import type { DealRoom, Document, Link, Signal, ActionItem, RiskAlert, HeatAlert } from "@/types";

const mockFns = vi.hoisted(() => ({
  getDashboardStats: vi.fn(),
  getDealRooms: vi.fn(),
  getInsightsOverview: vi.fn(),
  getSignals: vi.fn(),
  updateActionStatus: vi.fn(),
  fetchSignals: vi.fn(),
  navigate: vi.fn(),
  storeActions: [] as ActionItem[],
}));

vi.mock("@/lib/api", () => ({
  api: mockFns,
}));

vi.mock("@/stores/signalStore", () => ({
  useSignalStore: () => ({
    signals: [] as Signal[],
    actions: mockFns.storeActions,
    fetchSignals: mockFns.fetchSignals,
    updateActionStatus: mockFns.updateActionStatus,
  }),
}));

vi.mock("react-router", async () => {
  const actual = await vi.importActual<typeof import("react-router")>("react-router");
  return {
    ...actual,
    useParams: () => ({ workspaceSlug: "acme" }),
    useNavigate: () => mockFns.navigate,
  };
});

function makeStats(): DashboardStats {
  return {
    hotCount: 2,
    warmCount: 1,
    coldCount: 0,
    weeklyVisitors: 5,
    pendingQuestions: 1,
    recentDocuments: [] as Document[],
    recentLinks: [] as Link[],
    heatAlerts: [] as HeatAlert[],
    riskAlerts: [] as RiskAlert[],
    signals: [] as Signal[],
    actionItems: [] as ActionItem[],
    recentActivities: [],
  };
}

function makeInsights(): InsightsOverview {
  return {
    tierCounts: { hot: 1, warm: 0, cold: 0 },
    topDocuments: [],
    topLinks: [],
    topContacts: [],
  };
}

function makeRoom(): DealRoom {
  return {
    id: "room-1",
    name: "Active Room",
    description: "Description",
    template: "startup-fundraising",
    status: "active",
    documentCount: 1,
    memberCount: 1,
    pendingApprovals: 0,
    ndaEnabled: false,
    createdAt: "2026-01-01T00:00:00Z",
    visitorCount: 3,
    unreadQuestions: 0,
    heatScore: 45,
  };
}

async function renderPage(waitForLoad = true) {
  const i18n = await createTestI18n({
    dashboard: {
      title: "Dashboard",
      "welcome.title": "Welcome back",
      "metrics.title": "Key metrics",
      "metrics.activeRooms": "Active rooms",
      "metrics.weeklyVisitors": "Weekly visitors",
      "metrics.pendingQuestions": "Pending Q&A",
      "metrics.highIntentContacts": "High-intent contacts",
      "metrics.aria.activeRooms": "{{count}} active rooms",
      "metrics.aria.weeklyVisitors": "{{count}} weekly visitors",
      "metrics.aria.pendingQuestions": "{{count}} pending questions",
      "metrics.aria.highIntentContacts": "{{count}} high-intent contacts",
      "sections.activeRooms": "Active deal rooms",
      "sections.heatMap": "Heat map",
      "sections.activityFeed": "Activity timeline",
      "sections.recentVisitors": "Recent visitors",
      "sections.attention": "Attention",
      "attention.actions": "Actions",
      "attention.signals": "High-intent signals",
      "attention.risks": "Risks",
      "empty.actions.title": "No pending actions",
      "empty.actions.description": "All done",
      "empty.rooms.title": "No active rooms",
      "empty.rooms.description": "Create one",
      "empty.rooms.action": "Create",
      "empty.rooms.noDescription": "No description",
      "empty.visitors.description": "No visitors",
      "empty.signals.title": "No signals",
      "empty.signals.description": "No signals yet",
      "empty.activity.description": "No activity",
      "riskAlerts.title": "Risk alerts",
      "empty.risks.description": "No risks",
      "room.enter": "Enter {{name}}",
      "room.visitors": "{{count}} visitors",
      "room.lastAccessed": "Last active",
      "room.heatLabel": "Engagement",
      "room.status.active": "Active",
      "room.status.inactive": "Inactive",
      "visitor.score": "Score {{score}}",
      "visitor.viewProfile": "View profile of {{email}}",
      "heatMap.linkCount_one": "{{count}} link",
      "heatMap.linkCount_other": "{{count}} links",
      "heatMap.accessCount_one": "{{count}} view",
      "heatMap.accessCount_other": "{{count}} views",
      "actions.completedWithCount": "Completed ({{count}})",
      "actions.moreOptions": "More options",
      "actions.postpone": "Postpone",
      "actions.ignore": "Ignore",
      "actions.hiddenWithCount": "Hidden ({{count}})",
      "actions.reactivate": "Reactivate",
      "activity.events.visit": "visited",
      "activity.returnToDashboard": "Back",
    },
    common: {
      back: "Back",
      retry: "Retry",
      viewAll: "View all",
      viewDetails: "View details",
      createLink: "Create link",
      "heat.hot": "Hot",
      "heat.warm": "Warm",
      "heat.cold": "Cold",
    },
  });
  const view = render(
    <MemoryRouter initialEntries={["/acme"]}>
      <I18nextProvider i18n={i18n}>
        <Routes>
          <Route path="/:workspaceSlug/*" element={<DashboardPage />} />
        </Routes>
      </I18nextProvider>
    </MemoryRouter>
  );
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  if (waitForLoad) {
    await waitFor(() => {
      expect(
        screen.queryByText("Active deal rooms") || screen.queryByText("Failed to load")
      ).toBeInTheDocument();
    });
  }
  return view;
}

beforeEach(() => {
  vi.clearAllMocks();
  mockFns.storeActions = [];
  mockFns.getDashboardStats.mockResolvedValue(makeStats());
  mockFns.getDealRooms.mockResolvedValue({ data: [makeRoom()] });
  mockFns.getInsightsOverview.mockResolvedValue(makeInsights());
  mockFns.fetchSignals.mockResolvedValue(undefined);
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

describe("DashboardPage", () => {
  it("shows loading skeleton initially", async () => {
    mockFns.getDashboardStats.mockReturnValue(new Promise(() => {}));
    mockFns.getDealRooms.mockReturnValue(new Promise(() => {}));
    mockFns.getInsightsOverview.mockReturnValue(new Promise(() => {}));
    const { container } = await renderPage(false);
    expect(container.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(14);
  });

  it("renders key sections after data loads", async () => {
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText("Active deal rooms")).toBeInTheDocument();
    });
    expect(screen.getByText("Heat map")).toBeInTheDocument();
    expect(screen.getByText("Recent visitors")).toBeInTheDocument();
    expect(screen.getByText("Activity timeline")).toBeInTheDocument();
  });

  it("shows error state and allows retry", async () => {
    mockFns.getDashboardStats.mockRejectedValue(new Error("Network error"));
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText("Failed to load")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("button", { name: /Retry/i }));
    await waitFor(() => {
      expect(mockFns.getDashboardStats).toHaveBeenCalledTimes(2);
    });
  });

  it("routes document and deal-room share todos to isolated surfaces", async () => {
    const now = "2026-06-20T18:00:00Z";
    mockFns.storeActions = [
      {
        id: "act-doc",
        sourceType: "link_access_request",
        sourceId: "link-doc",
        title: "Approve access request from doc@example.com for Pitch",
        impact: "high",
        dueAt: "2026-06-21T18:00:00Z",
        status: "pending",
        actionType: "approve",
        createdAt: now,
        updatedAt: now,
      },
      {
        id: "act-room-link",
        sourceType: "deal_room_link_access_request",
        sourceId: "link-room",
        targetId: "room-9",
        title: "Approve deal room share access from room@example.com for Seed",
        impact: "high",
        dueAt: "2026-06-21T18:00:00Z",
        status: "pending",
        actionType: "approve",
        createdAt: now,
        updatedAt: now,
      },
      {
        id: "act-room-member",
        sourceType: "room_access_request",
        sourceId: "room-9",
        title: "Approve room access request from member@example.com for Seed",
        impact: "high",
        dueAt: "2026-06-21T18:00:00Z",
        status: "pending",
        actionType: "approve",
        createdAt: now,
        updatedAt: now,
      },
    ];

    await renderPage();
    await waitFor(() => {
      expect(screen.getByText(/Approve access request from doc@example.com/i)).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText(/Approve access request from doc@example.com/i));
    expect(mockFns.navigate).toHaveBeenCalledWith(
      "/acme/documents?tab=shared&linkId=link-doc",
    );

    fireEvent.click(screen.getByText(/Approve deal room share access from room@example.com/i));
    expect(mockFns.navigate).toHaveBeenCalledWith(
      "/acme/deal-rooms/room-9?tab=access&linkId=link-room",
    );

    fireEvent.click(screen.getByText(/Approve room access request from member@example.com/i));
    expect(mockFns.navigate).toHaveBeenCalledWith("/acme/deal-rooms/room-9?tab=access");
  });

  it("routes formal Ask review todos to the QA formal queue tab", async () => {
    const now = "2026-06-20T18:00:00Z";
    mockFns.storeActions = [
      {
        id: "act-formal",
        sourceType: "deal_room_link_question",
        sourceId: "turn-formal-1",
        targetId: "room-9/link-formal",
        title: "Review formal Q&A from visitor@example.com on Seed Link",
        impact: "medium",
        dueAt: "2026-06-21T18:00:00Z",
        status: "pending",
        actionType: "review",
        createdAt: now,
        updatedAt: now,
      },
    ];

    await renderPage();
    await waitFor(() => {
      expect(screen.getByText(/Review formal Q&A from visitor@example.com/i)).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText(/Review formal Q&A from visitor@example.com/i));
    expect(mockFns.navigate).toHaveBeenCalledWith(
      "/acme/deal-rooms/room-9?tab=qa&linkId=link-formal&askInbox=formal_queue",
    );
  });
});
