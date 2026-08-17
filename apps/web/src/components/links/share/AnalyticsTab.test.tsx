// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import type { Link, LinkAnalytics } from "@/types";
import { AnalyticsTab } from "./AnalyticsTab";
import { useWorkspaceAccess } from "@/hooks/useWorkspaceAccess";
import enLinkShare from "@/i18n/locales/en/linkShare.json";
import { toast } from "sonner";

const i18nInstance = i18n.createInstance();
i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: {
    en: {
      linkShare: enLinkShare,
      common: { loading: "Loading..." },
      links: {
        accessLog: {
          timestamp: "Time",
          visitor: "Visitor",
          page: "Page",
          duration: "Duration",
          device: "Device",
          location: "Location",
          anonymous: "Anonymous",
          empty: { title: "Empty", description: "No logs" },
        },
      },
    },
  },
  interpolation: { escapeValue: false },
});

function Wrapper({ children }: { children: React.ReactNode }) {
  return <I18nextProvider i18n={i18nInstance}>{children}</I18nextProvider>;
}

vi.mock("@/lib/api", () => ({
  api: {
    getLinkAnalytics: vi.fn(),
    getAccessLogs: vi.fn(),
    listLinkRecentVisitors: vi.fn(),
    listLinkAccessCodeContacts: vi.fn(),
    listLinkAsk: vi.fn(() => Promise.resolve({ data: [] })),
    listLinkFileRequests: vi.fn(() => Promise.resolve({ data: [] })),
    resendLinkAccessCode: vi.fn(() => Promise.resolve(undefined)),
    resendFailedLinkAccessCodes: vi.fn(() =>
      Promise.resolve({ data: { attempted: 0, sent: 0, failed: 0, skipped: 0 } }),
    ),
  },
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), message: vi.fn() },
}));

vi.mock("./ManagementTab", () => ({
  ManagementTab: () => <div data-testid="management-tab-stub" />,
}));

vi.mock("./AskSecurityEventsPanel", () => ({
  AskSecurityEventsPanel: () => null,
}));

const baseLink = {
  id: "link-1",
  name: "Acme Corp",
  requireEmailVerification: true,
  accessCount: 0,
  avgDurationSeconds: 0,
} as unknown as Link;

const emptyAnalytics: LinkAnalytics = {
  total_views: 0,
  unique_visitors: 0,
  download_attempts: 0,
  views_over_time: [],
  average_duration_seconds: 0,
  recent_visitors: [],
  recent_visitors_has_more: false,
  key_pages: [],
  qa_records: [],
  access_code_contacts: [],
};

describe("AnalyticsTab", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(api.listLinkAsk).mockResolvedValue({ data: [] });
    vi.mocked(api.listLinkFileRequests).mockResolvedValue({ data: [] });
    vi.mocked(api.listLinkRecentVisitors).mockResolvedValue({
      data: [],
      has_more: false,
    });
    vi.mocked(api.listLinkAccessCodeContacts).mockResolvedValue({
      data: [],
      has_more: false,
    });
    vi.mocked(api.getAccessLogs).mockResolvedValue({ data: [], has_more: false });
  });

  it("shows verification code delivery statuses for contacts", async () => {
    vi.mocked(api.getLinkAnalytics).mockResolvedValue({
      data: {
        ...emptyAnalytics,
        access_code_remediable_count: 1,
        access_code_failed_count: 1,
        access_code_contacts: [
          {
            email: "ok@example.com",
            name: "Ok User",
            send_status: "sent",
            code_sent_at: "2026-07-20T02:00:00Z",
            can_resend: false,
          },
          {
            email: "bad@example.com",
            send_status: "failed",
            can_resend: true,
          },
          {
            email: "wait@example.com",
            send_status: "pending",
            can_resend: false,
          },
        ],
      },
    });

    render(
      <Wrapper>
        <AnalyticsTab link={baseLink} logs={[]} />
      </Wrapper>,
    );

    // Remediable delivery auto-opens the Delivery tab.
    expect(await screen.findByText("Verification code delivery")).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /Delivery/ })).toHaveAttribute(
      "data-active",
      "",
    );
    expect(screen.getByText(/Ok User · ok@example.com/)).toBeInTheDocument();
    expect(screen.getByText("bad@example.com")).toBeInTheDocument();
    expect(screen.getByText("wait@example.com")).toBeInTheDocument();
    expect(screen.getAllByText("Sent").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("Failed").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("Pending").length).toBeGreaterThanOrEqual(1);
    expect(
      screen.getByText("Could not deliver the verification code. Try resending."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Resend" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Resend undelivered" })).toBeInTheDocument();
  });

  it("defaults to Visitors when there is no remediable delivery", async () => {
    vi.mocked(api.getLinkAnalytics).mockResolvedValue({
      data: {
        ...emptyAnalytics,
        recent_visitors: [
          {
            visitor_id: "v1",
            visitor_email: "guest@example.com",
            first_access_at: "2026-07-20T01:00:00Z",
            last_access_at: "2026-07-20T02:00:00Z",
            total_views: 3,
          },
        ],
      },
    });

    render(
      <Wrapper>
        <AnalyticsTab
          link={{ ...baseLink, requireEmailVerification: false } as Link}
          logs={[]}
        />
      </Wrapper>,
    );

    expect(await screen.findByText("guest@example.com")).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Visitors" })).toHaveAttribute(
      "data-active",
      "",
    );
    expect(screen.queryByRole("tab", { name: /Delivery/ })).not.toBeInTheDocument();
  });

  it("uses analytics unique visitors, not the complete activity log", async () => {
    vi.mocked(api.getLinkAnalytics).mockResolvedValue({
      data: { ...emptyAnalytics, unique_visitors: 1, total_views: 2 },
    });

    render(
      <Wrapper>
        <AnalyticsTab
          link={{ ...baseLink, accessCount: 99, requireEmailVerification: false } as Link}
          logs={[
            { visitorEmail: "owner@acme.com" },
            { visitorEmail: "owner@acme.com" },
            { visitorEmail: "guest@example.com" },
          ] as never}
        />
      </Wrapper>,
    );

    expect(
      await screen.findByText(
        "Views, unique visitors, duration, and last visit exclude workspace members (same as Deal Radar and heat). The activity log stays complete.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
    expect(screen.queryByText("99")).not.toBeInTheDocument();
  });

  it("resends only remediable contacts and refreshes analytics", async () => {
    vi.mocked(api.getLinkAnalytics)
      .mockResolvedValueOnce({
        data: {
          ...emptyAnalytics,
          access_code_remediable_count: 1,
          access_code_failed_count: 1,
          access_code_contacts: [
            {
              email: "bad@example.com",
              send_status: "failed",
              can_resend: true,
            },
          ],
        },
      })
      .mockResolvedValueOnce({
        data: {
          ...emptyAnalytics,
          access_code_remediable_count: 0,
          access_code_failed_count: 0,
          access_code_contacts: [
            {
              email: "bad@example.com",
              send_status: "sent",
              can_resend: false,
            },
          ],
        },
      });
    vi.mocked(api.resendLinkAccessCode).mockResolvedValue(undefined);
    vi.mocked(api.listLinkAccessCodeContacts)
      .mockResolvedValueOnce({
        data: [
          {
            email: "bad@example.com",
            send_status: "sent",
            can_resend: false,
          },
        ],
        has_more: false,
      });

    render(
      <Wrapper>
        <AnalyticsTab link={baseLink} logs={[]} />
      </Wrapper>,
    );

    fireEvent.click(await screen.findByRole("button", { name: "Resend" }));
    await waitFor(() => {
      expect(api.resendLinkAccessCode).toHaveBeenCalledWith("link-1", "bad@example.com");
    });
    await waitFor(() => {
      expect(screen.queryByRole("button", { name: "Resend" })).not.toBeInTheDocument();
    });
  });

  it("shows a send-failed toast when resend hits access_code_send_failed", async () => {
    vi.mocked(api.getLinkAnalytics).mockResolvedValue({
      data: {
        ...emptyAnalytics,
        access_code_remediable_count: 1,
        access_code_failed_count: 1,
        access_code_contacts: [
          {
            email: "bad@example.com",
            send_status: "failed",
            can_resend: true,
          },
        ],
      },
    });
    vi.mocked(api.resendLinkAccessCode).mockRejectedValue(
      new ApiError({
        status: 502,
        code: "access_code_send_failed",
        message: 'could not send verification code',
        requestId: "req-send-fail",
      }),
    );

    render(
      <Wrapper>
        <AnalyticsTab link={baseLink} logs={[]} />
      </Wrapper>,
    );

    fireEvent.click(await screen.findByRole("button", { name: "Resend" }));
    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(
        "Could not send the verification code. Please try again.",
      );
    });
  });

  it("hides verification code section when verification is off and no contacts", async () => {
    vi.mocked(api.getLinkAnalytics).mockResolvedValue({ data: emptyAnalytics });

    render(
      <Wrapper>
        <AnalyticsTab
          link={{ ...baseLink, requireEmailVerification: false } as Link}
          logs={[]}
        />
      </Wrapper>,
    );

    await waitFor(() => {
      expect(api.getLinkAnalytics).toHaveBeenCalledWith("link-1");
    });
    expect(screen.queryByRole("tab", { name: /Delivery/ })).not.toBeInTheDocument();
    expect(screen.queryByText("Verification code delivery")).not.toBeInTheDocument();
  });

  it("loads the next visitors page when the scroll sentinel intersects", async () => {
    const observers: Array<{
      callback: IntersectionObserverCallback;
      disconnect: ReturnType<typeof vi.fn>;
    }> = [];
    vi.stubGlobal(
      "IntersectionObserver",
      class {
        callback: IntersectionObserverCallback;
        disconnect = vi.fn();
        observe = vi.fn();
        unobserve = vi.fn();
        constructor(callback: IntersectionObserverCallback) {
          this.callback = callback;
          observers.push({ callback, disconnect: this.disconnect });
        }
      },
    );

    const firstPage = Array.from({ length: 10 }, (_, i) => ({
      visitor_id: `v-${i}`,
      visitor_email: `user${i}@example.com`,
      first_access_at: "2026-07-20T01:00:00Z",
      last_access_at: "2026-07-20T02:00:00Z",
      total_views: 1,
    }));
    const secondPage = [
      {
        visitor_id: "v-10",
        visitor_email: "user10@example.com",
        first_access_at: "2026-07-20T01:00:00Z",
        last_access_at: "2026-07-20T01:30:00Z",
        total_views: 2,
      },
    ];

    vi.mocked(api.getLinkAnalytics).mockResolvedValue({
      data: {
        ...emptyAnalytics,
        recent_visitors: firstPage,
        recent_visitors_has_more: true,
      },
    });
    vi.mocked(api.listLinkRecentVisitors).mockResolvedValue({
      data: secondPage,
      has_more: false,
    });

    render(
      <Wrapper>
        <AnalyticsTab
          link={{ ...baseLink, requireEmailVerification: false } as Link}
          logs={[]}
        />
      </Wrapper>,
    );

    expect(await screen.findByText("user0@example.com")).toBeInTheDocument();
    expect(screen.getByText("user9@example.com")).toBeInTheDocument();

    await waitFor(() => {
      expect(observers.length).toBeGreaterThan(0);
    });
    observers[observers.length - 1]!.callback(
      [{ isIntersecting: true } as IntersectionObserverEntry],
      {} as IntersectionObserver,
    );

    await waitFor(() => {
      expect(api.listLinkRecentVisitors).toHaveBeenCalledWith("link-1", {
        limit: 10,
        offset: 10,
      });
    });
    expect(await screen.findByText("user10@example.com")).toBeInTheDocument();
  });

  it("keeps loaded visitors after analytics refresh", async () => {
    const observers: Array<{
      callback: IntersectionObserverCallback;
    }> = [];
    vi.stubGlobal(
      "IntersectionObserver",
      class {
        callback: IntersectionObserverCallback;
        disconnect = vi.fn();
        observe = vi.fn();
        unobserve = vi.fn();
        constructor(callback: IntersectionObserverCallback) {
          this.callback = callback;
          observers.push({ callback });
        }
      },
    );

    const firstPage = Array.from({ length: 10 }, (_, i) => ({
      visitor_id: `v-${i}`,
      visitor_email: `user${i}@example.com`,
      first_access_at: "2026-07-20T01:00:00Z",
      last_access_at: "2026-07-20T02:00:00Z",
      total_views: 1,
    }));
    const secondPage = [
      {
        visitor_id: "v-10",
        visitor_email: "user10@example.com",
        first_access_at: "2026-07-20T01:00:00Z",
        last_access_at: "2026-07-20T01:30:00Z",
        total_views: 2,
      },
    ];

    vi.mocked(api.getLinkAnalytics)
      .mockResolvedValueOnce({
        data: {
          ...emptyAnalytics,
          recent_visitors: firstPage,
          recent_visitors_has_more: true,
          access_code_remediable_count: 1,
          access_code_failed_count: 1,
          access_code_contacts: [
            {
              email: "bad@example.com",
              send_status: "failed",
              can_resend: true,
            },
          ],
        },
      })
      .mockResolvedValueOnce({
        data: {
          ...emptyAnalytics,
          recent_visitors: firstPage,
          recent_visitors_has_more: true,
          access_code_remediable_count: 0,
          access_code_failed_count: 0,
          access_code_contacts: [
            {
              email: "bad@example.com",
              send_status: "sent",
              can_resend: false,
            },
          ],
        },
      });
    vi.mocked(api.listLinkRecentVisitors).mockResolvedValue({
      data: secondPage,
      has_more: false,
    });
    vi.mocked(api.resendLinkAccessCode).mockResolvedValue(undefined);
    vi.mocked(api.listLinkAccessCodeContacts).mockResolvedValue({
      data: [
        {
          email: "bad@example.com",
          send_status: "sent",
          can_resend: false,
        },
      ],
      has_more: false,
    });

    render(
      <Wrapper>
        <AnalyticsTab link={baseLink} logs={[]} />
      </Wrapper>,
    );

    // Wait for auto-priority Delivery tab, then switch to Visitors.
    expect(await screen.findByText("Verification code delivery")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("tab", { name: /^Visitors$/ }));
    expect(await screen.findByText("user0@example.com")).toBeInTheDocument();

    await waitFor(() => {
      expect(observers.length).toBeGreaterThan(0);
    });
    observers[observers.length - 1]!.callback(
      [{ isIntersecting: true } as IntersectionObserverEntry],
      {} as IntersectionObserver,
    );
    expect(await screen.findByText("user10@example.com")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: /Delivery/ }));
    fireEvent.click(await screen.findByRole("button", { name: "Resend" }));
    await waitFor(() => {
      expect(api.resendLinkAccessCode).toHaveBeenCalled();
    });
    await waitFor(() => {
      expect(api.getLinkAnalytics).toHaveBeenCalledTimes(2);
    });

    fireEvent.click(screen.getByRole("tab", { name: /^Visitors$/ }));
    expect(screen.getByText("user10@example.com")).toBeInTheDocument();
  });

  it("loads the next access-log page when the activity sentinel intersects", async () => {
    const observers: Array<{
      callback: IntersectionObserverCallback;
    }> = [];
    vi.stubGlobal(
      "IntersectionObserver",
      class {
        callback: IntersectionObserverCallback;
        disconnect = vi.fn();
        observe = vi.fn();
        unobserve = vi.fn();
        constructor(callback: IntersectionObserverCallback) {
          this.callback = callback;
          observers.push({ callback });
        }
      },
    );

    const firstPage = Array.from({ length: 10 }, (_, i) => ({
      id: `log-${i}`,
      linkId: "link-1",
      visitorEmail: `activity${i}@example.com`,
      durationSeconds: 0,
      device: "DealSignalSeed/1.0",
      timestamp: `2026-07-20T10:${String(50 - i).padStart(2, "0")}:00Z`,
    }));
    const secondPage = [
      {
        id: "log-10",
        linkId: "link-1",
        visitorEmail: "activity10@example.com",
        durationSeconds: 0,
        device: "DealSignalSeed/1.0",
        timestamp: "2026-07-20T10:40:00Z",
      },
    ];

    vi.mocked(api.getLinkAnalytics).mockResolvedValue({
      data: { ...emptyAnalytics, recent_visitors_has_more: false },
    });
    vi.mocked(api.getAccessLogs)
      .mockResolvedValueOnce({ data: firstPage, has_more: true })
      .mockResolvedValueOnce({ data: secondPage, has_more: false });

    render(
      <Wrapper>
        <AnalyticsTab
          link={{ ...baseLink, requireEmailVerification: false } as Link}
          logs={[]}
        />
      </Wrapper>,
    );

    fireEvent.click(await screen.findByRole("tab", { name: "Activity log" }));
    expect(await screen.findByText("activity0@example.com")).toBeInTheDocument();

    await waitFor(() => {
      expect(observers.length).toBeGreaterThan(0);
    });
    observers[observers.length - 1]!.callback(
      [{ isIntersecting: true } as IntersectionObserverEntry],
      {} as IntersectionObserver,
    );

    await waitFor(() => {
      expect(api.getAccessLogs).toHaveBeenCalledWith("link-1", {
        limit: 10,
        offset: 10,
      });
    });
    expect(await screen.findByText("activity10@example.com")).toBeInTheDocument();
  });

  it("loads the next delivery contacts page when the sentinel intersects", async () => {
    const observers: Array<{
      callback: IntersectionObserverCallback;
    }> = [];
    vi.stubGlobal(
      "IntersectionObserver",
      class {
        callback: IntersectionObserverCallback;
        disconnect = vi.fn();
        observe = vi.fn();
        unobserve = vi.fn();
        constructor(callback: IntersectionObserverCallback) {
          this.callback = callback;
          observers.push({ callback });
        }
      },
    );

    const firstPage = Array.from({ length: 10 }, (_, i) => ({
      email: `contact${i}@example.com`,
      send_status: "sent" as const,
      can_resend: false,
    }));
    const secondPage = [
      {
        email: "contact10@example.com",
        send_status: "failed" as const,
        can_resend: true,
      },
    ];

    vi.mocked(api.getLinkAnalytics).mockResolvedValue({
      data: {
        ...emptyAnalytics,
        access_code_contacts: firstPage,
        access_code_contacts_has_more: true,
        access_code_remediable_count: 1,
        access_code_failed_count: 1,
      },
    });
    vi.mocked(api.listLinkAccessCodeContacts).mockResolvedValue({
      data: secondPage,
      has_more: false,
    });

    render(
      <Wrapper>
        <AnalyticsTab link={baseLink} logs={[]} />
      </Wrapper>,
    );

    expect(await screen.findByText("contact0@example.com")).toBeInTheDocument();
    await waitFor(() => {
      expect(observers.length).toBeGreaterThan(0);
    });
    observers[observers.length - 1]!.callback(
      [{ isIntersecting: true } as IntersectionObserverEntry],
      {} as IntersectionObserver,
    );

    await waitFor(() => {
      expect(api.listLinkAccessCodeContacts).toHaveBeenCalledWith("link-1", {
        limit: 10,
        offset: 10,
      });
    });
    expect(await screen.findByText("contact10@example.com")).toBeInTheDocument();
  });

  it("shows Ask activity summary on Engage tab for deal-room links", async () => {
    vi.mocked(api.getLinkAnalytics).mockResolvedValue({
      data: {
        ...emptyAnalytics,
        ask_summary: {
          total_turns: 5,
          ai_answered: 2,
          ai_refused: 1,
          host_pending: 1,
          host_answered: 1,
          user_escalated: 1,
          auto_escalated: 0,
          deflection_rate: 2 / 3,
          refuse_rate: 1 / 3,
          escalation_rate: 1 / 3,
        },
      },
    });
    vi.mocked(api.listLinkAsk).mockResolvedValue({ data: [] });

    render(
      <Wrapper>
        <AnalyticsTab
          link={{ ...baseLink, dealRoomId: "room_1", canManageAsk: true } as Link}
          logs={[]}
        />
      </Wrapper>,
    );

    await waitFor(() => {
      expect(api.getLinkAnalytics).toHaveBeenCalled();
    });
    fireEvent.click(screen.getByRole("tab", { name: /Engage/i }));
    expect(screen.getAllByText("Ask activity").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("AI answered")).toBeInTheDocument();
    expect(screen.getByText("67%")).toBeInTheDocument();
    expect(screen.getByText("Refuse rate")).toBeInTheDocument();
    expect(screen.getByText("Escalation rate")).toBeInTheDocument();
    expect(screen.getAllByText("33%").length).toBeGreaterThanOrEqual(2);
  });

  it("hides Engage on deal-room links without room manage even if workspace can write", async () => {
    vi.mocked(useWorkspaceAccess).mockReturnValue({
      role: "admin",
      loading: false,
      canRead: true,
      canWrite: true,
      canManage: true,
      isGuest: false,
    });
    vi.mocked(api.getLinkAnalytics).mockResolvedValue({ data: emptyAnalytics });

    render(
      <Wrapper>
        <AnalyticsTab link={{ ...baseLink, dealRoomId: "room_1" } as Link} logs={[]} />
      </Wrapper>,
    );

    await waitFor(() => {
      expect(api.getLinkAnalytics).toHaveBeenCalled();
    });
    expect(screen.queryByRole("tab", { name: /Engage/i })).not.toBeInTheDocument();
    expect(api.listLinkAsk).not.toHaveBeenCalled();
  });

  it("does not fetch management data or toast for read-only guests", async () => {
    vi.mocked(useWorkspaceAccess).mockReturnValue({
      role: "guest",
      loading: false,
      canRead: true,
      canWrite: false,
      canManage: false,
      isGuest: true,
    });
    vi.mocked(api.getLinkAnalytics).mockResolvedValue({ data: emptyAnalytics });
    vi.mocked(api.listLinkAsk).mockRejectedValue(
      new ApiError({ status: 403, code: "forbidden", message: "forbidden", requestId: "r1" }),
    );

    render(
      <Wrapper>
        <AnalyticsTab link={baseLink} logs={[]} />
      </Wrapper>,
    );

    await waitFor(() => {
      expect(api.getLinkAnalytics).toHaveBeenCalledWith("link-1");
    });
    expect(api.listLinkAsk).not.toHaveBeenCalled();
    expect(api.listLinkFileRequests).not.toHaveBeenCalled();
    expect(toast.error).not.toHaveBeenCalled();
    expect(screen.queryByRole("tab", { name: /Engage/i })).not.toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /Visitors/i })).toBeInTheDocument();
  });
});
