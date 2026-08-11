// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router";
import { RadarEvidenceRail } from "./RadarEvidenceRail";
import { createTestI18n } from "@/i18n/test-utils";
import type { RadarEvidencePack, RadarWorkItem } from "@/lib/radarQueue";

const mockFns = vi.hoisted(() => ({
  getRadarEvidence: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: mockFns,
}));

function makeItem(over: Partial<RadarWorkItem> = {}): RadarWorkItem {
  return {
    id: "act-evidence-1",
    product: "leak_watch",
    headline: "Review forward risk",
    headlineCode: "radar.headline.leak_watch",
    subtitle: "",
    verb: "review",
    priority: "high",
    slaDueAt: "2026-08-11T18:00:00Z",
    createdAt: "2026-08-11T12:00:00Z",
    dealKey: "link:link-1",
    dealName: "Pitch",
    actionId: "act-evidence-1",
    whyNowCode: "leak_watch",
    whyNowHours: 2,
    ...over,
  };
}

async function renderRail(item: RadarWorkItem | null) {
  const i18n = await createTestI18n({
    dashboard: {
      "radar.products.leak_watch": "Leak watch",
      "radar.products.buying_window": "Hot intent",
      "radar.evidenceRail.title": "Evidence",
      "radar.evidenceRail.empty": "Select a radar item",
      "radar.evidenceRail.loading": "Loading evidence…",
      "radar.evidenceRail.error": "Could not load evidence.",
      "radar.products.diligence_gate": "Diligence gate",
      "radar.evidenceRail.degraded": "Some evidence facets failed to load.",
      "radar.evidenceRail.degradedSections.metrics": "24h metrics",
      "radar.evidenceRail.metrics.opens24h": "Opens (24h)",
      "radar.evidenceRail.metrics.visitors24h": "Visitors (24h)",
      "radar.evidenceRail.metrics.forwards24h": "Forwards (24h)",
      "radar.evidenceRail.metrics.downloads24h": "Downloads (24h)",
      "radar.evidenceRail.recentVisitors": "Recent visitors",
      "radar.evidenceRail.keyPages": "Key pages",
      "radar.evidenceRail.topPages": "Top pages",
      "radar.evidenceRail.page": "Page {{page}}",
      "radar.evidenceRail.securityEvents": "Security events",
      "radar.evidenceRail.eventTypes.forward_signal": "Forward signal",
      "radar.evidenceRail.reasons.abnormal_access": "Abnormal access",
      "radar.evidenceRail.views_one": "{{count}} view",
      "radar.evidenceRail.views_other": "{{count}} views",
      "radar.evidenceRail.openFull": "Open full evidence",
      "radar.evidenceRail.openShareInbox": "Review in Share",
      "radar.evidenceRail.gateEvents": "Gate events",
      "radar.evidenceRail.gateTimeline.beforeAndAfter":
        "Hit the gate {{before}}× before requesting, then {{after}}× after — still blocked.",
      "radar.evidenceRail.gateTimeline.beforeOnly":
        "Hit the gate {{before}}× before requesting access.",
      "radar.evidenceRail.gateTimeline.afterOnly":
        "Requested access, then hit the gate {{after}}× — still blocked.",
      "radar.evidenceRail.gateTimeline.eventsOnly":
        "Hit the gate {{total}}× — still blocked.",
      "radar.evidenceRail.coalesced.count": "{{count}}×",
      "radar.evidenceRail.gateNoSuccessfulOpens":
        "No successful opens yet — the visitor is still at the gate.",
      "radar.evidenceRail.emptyPrimary.leak_watch":
        "No corroborating 24h forward/download activity on this link yet.",
      "radar.evidenceRail.accessRequest.title": "Access request",
      "radar.evidenceRail.accessRequest.reason": "Reason",
      "radar.evidenceRail.accessRequest.surface": "Surface",
      "radar.evidenceRail.accessRequest.surfaces.document_link": "Document share link",
      "radar.evidenceRail.accessRequest.noReason": "No reason provided",
      "radar.evidenceRail.reasons.email_code_required": "Email verification required",
      "radar.evidenceRail.eventTypes.security_gate_failed": "Security gate failed",
      "radar.whyNow.leak_watch": "Leak risk in the last {{hours}}h",
      "radar.whyNow.fallback.leak_watch": "Review soon",
      "radar.whyNow.buying_window": "Hot intent — follow up while interest is warm",
      "radar.whyNow.fallback.buying_window": "Follow up while interest is warm",
      "radar.whyNow.diligence_gate": "Someone is waiting at the gate",
      "radar.whyNow.fallback.diligence_gate": "Approve or reject",
      "radar.headline.leak_watch": "Forward risk on Pitch",
      "radar.headline.diligence_gate": "Approve access request",
    },
  });
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter>
        <RadarEvidenceRail item={item} workspaceSlug="acme" />
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("RadarEvidenceRail", () => {
  beforeEach(() => {
    mockFns.getRadarEvidence.mockReset();
  });

  it("shows empty state without fetching when no item is selected", async () => {
    await renderRail(null);
    expect(screen.getByTestId("radar-evidence-rail")).toHaveTextContent(
      "Select a radar item",
    );
    expect(mockFns.getRadarEvidence).not.toHaveBeenCalled();
  });

  it("fetches evidence and renders metrics", async () => {
    const pack: RadarEvidencePack = {
      itemId: "act-evidence-1",
      product: "leak_watch",
      headline: "Review forward risk",
      whyNowCode: "leak_watch",
      whyNowHours: 2,
      insightsPath: "/acme/links/link-1",
      metrics: {
        opens24h: 9,
        uniqueVisitors24h: 4,
        forwardSignals24h: 3,
        downloads24h: 1,
      },
    };
    mockFns.getRadarEvidence.mockResolvedValue(pack);
    await renderRail(makeItem());
    expect(mockFns.getRadarEvidence).toHaveBeenCalledWith("act-evidence-1");
    expect(screen.getByText("Loading evidence…")).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByTestId("radar-evidence-metrics")).toBeInTheDocument();
    });
    // Card echo (deal / headline / why-now) stays on the radar row — not the rail.
    expect(screen.queryByTestId("radar-evidence-why-now")).not.toBeInTheDocument();
    expect(screen.getByTestId("radar-evidence-rail")).not.toHaveTextContent(
      "Review forward risk",
    );
    expect(screen.getByText("9")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
    expect(screen.getByTestId("radar-evidence-open")).toHaveAttribute(
      "href",
      "/acme/links/link-1",
    );
  });

  it("surfaces fetch errors", async () => {
    mockFns.getRadarEvidence.mockRejectedValue(new Error("boom"));
    await renderRail(makeItem());
    await waitFor(() => {
      expect(screen.getByText("Could not load evidence.")).toBeInTheDocument();
    });
  });

  it("surfaces degradedSections honesty banner", async () => {
    mockFns.getRadarEvidence.mockResolvedValue({
      itemId: "act-evidence-1",
      product: "leak_watch",
      headline: "Review forward risk",
      degradedSections: ["metrics"],
    } satisfies RadarEvidencePack);
    await renderRail(makeItem());
    await waitFor(() => {
      expect(screen.getByTestId("radar-evidence-degraded")).toBeInTheDocument();
    });
    expect(screen.getByTestId("radar-evidence-degraded")).toHaveTextContent(
      "24h metrics",
    );
  });

  it("shows access request + coalesced gate story for diligence_gate (not zero engagement tiles)", async () => {
    mockFns.getRadarEvidence.mockResolvedValue({
      itemId: "act-gate-1",
      product: "diligence_gate",
      headline: "Approve access request from lp@vc.com",
      whyNowCode: "diligence_gate",
      navigatePath: "/acme/documents?tab=shared&linkId=link-1",
      accessRequest: {
        email: "lp@vc.com",
        reason: "Need the deck for IC",
        status: "pending",
        requestedAt: "2026-08-11T10:15:00Z",
        surface: "document_link",
      },
      metrics: {
        opens24h: 0,
        uniqueVisitors24h: 0,
        forwardSignals24h: 0,
        downloads24h: 0,
      },
      securityEvents: [
        {
          eventType: "security_gate_failed",
          reason: "email_code_required",
          createdAt: "2026-08-11T10:23:33Z",
        },
        {
          eventType: "security_gate_failed",
          reason: "email_code_required",
          createdAt: "2026-08-11T10:23:27Z",
        },
        {
          eventType: "security_gate_failed",
          reason: "email_code_required",
          createdAt: "2026-08-11T10:23:23Z",
        },
        {
          eventType: "security_gate_failed",
          reason: "email_code_required",
          createdAt: "2026-08-11T10:14:46Z",
        },
      ],
    } satisfies RadarEvidencePack);

    await renderRail(
      makeItem({
        id: "act-gate-1",
        product: "diligence_gate",
        headline: "Approve access request from lp@vc.com",
        headlineCode: "radar.headline.diligence_gate",
        whyNowCode: "diligence_gate",
        verb: "approve",
      }),
    );

    await waitFor(() => {
      expect(screen.getByTestId("radar-evidence-access-request")).toBeInTheDocument();
    });
    // Applicant / headline already on the radar row — Evidence keeps unique facets only.
    expect(screen.queryByTestId("radar-evidence-applicant")).not.toBeInTheDocument();
    expect(screen.getByText("Need the deck for IC")).toBeInTheDocument();
    expect(screen.queryByTestId("radar-evidence-why-now")).not.toBeInTheDocument();
    expect(screen.getByTestId("radar-evidence-gate-timeline")).toHaveTextContent(
      "Hit the gate 1× before requesting, then 3× after — still blocked.",
    );
    const events = screen.getByTestId("radar-evidence-security-events");
    expect(events).toHaveTextContent("Email verification required");
    expect(events).toHaveTextContent("4×");
    expect(events).not.toHaveTextContent("Show times");
    expect(events).not.toHaveTextContent("last ");
    expect(events).not.toHaveTextContent("2026");
    // Gate / security rows use the shared embedded card tile.
    expect(events.querySelectorAll("li.rounded-md.border").length).toBeGreaterThan(0);
    const access = screen.getByTestId("radar-evidence-access-request");
    expect(access.querySelectorAll(".rounded-md.border").length).toBeGreaterThanOrEqual(2);
    expect(access).not.toHaveTextContent("Requested");
    expect(access).not.toHaveTextContent("2026");
    expect(screen.getByTestId("radar-evidence-gate-no-opens")).toBeInTheDocument();
    expect(screen.queryByTestId("radar-evidence-metrics")).not.toBeInTheDocument();
    expect(screen.getByTestId("radar-evidence-open")).toHaveAttribute(
      "href",
      "/acme/documents?tab=shared&linkId=link-1",
    );
    expect(screen.getByTestId("radar-evidence-open")).toHaveTextContent("Review in Share");
  });

  it("skips deal / headline / why-now card echo for all products", async () => {
    mockFns.getRadarEvidence.mockResolvedValue({
      itemId: "act-hot-1",
      product: "buying_window",
      headline: "[radar-stress] Email partner@stress.example.com on Riverview Closing",
      whyNowCode: "buying_window",
      metrics: {
        opens24h: 3,
        uniqueVisitors24h: 2,
        forwardSignals24h: 2,
        downloads24h: 1,
      },
      recentVisitors: [
        {
          visitorId: "v1",
          email: "analyst@stress.example.com",
          totalViews: 0,
          lastAccessAt: "2026-08-11T19:03:06Z",
        },
        {
          visitorId: "v2",
          email: "unknown+7@forward.test",
          totalViews: 0,
          lastAccessAt: "2026-08-11T19:02:06Z",
        },
      ],
      keyPageTitles: ["Executive summary", "Financials"],
    } satisfies RadarEvidencePack);

    await renderRail(
      makeItem({
        id: "act-hot-1",
        product: "buying_window",
        headline:
          "[radar-stress] Email partner@stress.example.com on Riverview Closing",
        headlineCode: undefined,
        whyNowCode: "buying_window",
        actor: "partner@stress.example.com",
        dealName: "[radar-stress] link 4 a87ff6",
        verb: "email",
      }),
    );

    await waitFor(() => {
      expect(screen.getByTestId("radar-evidence-metrics")).toBeInTheDocument();
    });
    const rail = screen.getByTestId("radar-evidence-rail");
    expect(rail).toHaveTextContent("Evidence");
    expect(rail).not.toHaveTextContent("[radar-stress] link 4 a87ff6");
    expect(rail).not.toHaveTextContent("Riverview Closing");
    expect(rail).not.toHaveTextContent("partner@stress.example.com");
    expect(screen.queryByTestId("radar-evidence-why-now")).not.toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
    const visitors = screen.getByTestId("radar-evidence-recent-visitors");
    expect(visitors).toHaveTextContent("analyst@stress.example.com");
    expect(visitors).toHaveTextContent("0 views");
    expect(visitors).not.toHaveTextContent("2026");
    expect(visitors).not.toHaveTextContent("PM");
    // Hot intent nests key pages / visitors as embedded cards (metric-tile style).
    const keyPages = screen.getByTestId("radar-evidence-key-pages");
    expect(keyPages.querySelectorAll("li.rounded-md.border").length).toBe(2);
    expect(visitors.querySelectorAll("li.rounded-md.border").length).toBe(2);
  });

  it("uses embedded cards for leak_watch list facets (all products share the tile)", async () => {
    mockFns.getRadarEvidence.mockResolvedValue({
      itemId: "act-leak-cards",
      product: "leak_watch",
      headline: "Forward risk",
      whyNowCode: "leak_watch",
      metrics: {
        opens24h: 2,
        uniqueVisitors24h: 1,
        forwardSignals24h: 3,
        downloads24h: 1,
      },
      keyPageTitles: ["Pricing", "Roadmap"],
      topPages: [{ pageNumber: 3, views: 12, avgDurationSeconds: 40 }],
      recentVisitors: [
        {
          visitorId: "v-leak",
          email: "buyer@acme.test",
          totalViews: 4,
          lastAccessAt: "2026-08-11T18:00:00Z",
        },
      ],
      securityEvents: [
        {
          eventType: "forward_signal",
          reason: "abnormal_access",
          email: "analyst@stress.example.com",
          createdAt: "2026-08-11T17:00:00Z",
        },
      ],
    } satisfies RadarEvidencePack);

    await renderRail(
      makeItem({
        id: "act-leak-cards",
        product: "leak_watch",
        headline: "Forward risk",
        whyNowCode: "leak_watch",
        actor: "analyst@stress.example.com",
        dealName: "[radar-stress] link 8 c9f0f8",
        verb: "review",
      }),
    );

    await waitFor(() => {
      expect(screen.getByTestId("radar-evidence-metrics")).toBeInTheDocument();
    });
    const rail = screen.getByTestId("radar-evidence-rail");
    expect(rail).not.toHaveTextContent("[radar-stress] link 8 c9f0f8");
    expect(rail).not.toHaveTextContent("Forward risk");
    expect(screen.queryByTestId("radar-evidence-why-now")).not.toBeInTheDocument();
    expect(
      screen.getByTestId("radar-evidence-key-pages").querySelectorAll("li.rounded-md.border")
        .length,
    ).toBe(2);
    expect(
      screen.getByTestId("radar-evidence-top-pages").querySelectorAll("li.rounded-md.border")
        .length,
    ).toBe(1);
    expect(
      screen
        .getByTestId("radar-evidence-recent-visitors")
        .querySelectorAll("li.rounded-md.border").length,
    ).toBe(1);
    expect(screen.getByTestId("radar-evidence-recent-visitors")).not.toHaveTextContent(
      "2026",
    );
    const events = screen.getByTestId("radar-evidence-security-events");
    expect(events.querySelectorAll("li.rounded-md.border").length).toBe(1);
    expect(events).toHaveTextContent("Abnormal access");
    // Actor email already on the radar row — do not repeat on each security tile.
    expect(events).not.toHaveTextContent("analyst@stress.example.com");
    expect(events).not.toHaveTextContent("last ");
  });

  it("does not echo deal / headline / actor for leak_watch", async () => {
    mockFns.getRadarEvidence.mockResolvedValue({
      itemId: "act-leak-dup",
      product: "leak_watch",
      headline: "Review forward risk from analyst@stress.example.com",
      whyNowCode: "leak_watch",
      metrics: {
        opens24h: 3,
        uniqueVisitors24h: 2,
        forwardSignals24h: 1,
        downloads24h: 0,
      },
    } satisfies RadarEvidencePack);

    await renderRail(
      makeItem({
        id: "act-leak-dup",
        product: "leak_watch",
        headline: "Review forward risk from analyst@stress.example.com",
        headlineCode: undefined,
        whyNowCode: "leak_watch",
        actor: "analyst@stress.example.com",
        dealName: "[radar-stress] link 8 c9f0f8",
        verb: "review",
      }),
    );

    await waitFor(() => {
      expect(screen.getByText("3")).toBeInTheDocument();
    });
    const rail = screen.getByTestId("radar-evidence-rail");
    expect(rail).not.toHaveTextContent("[radar-stress] link 8 c9f0f8");
    expect(rail).not.toHaveTextContent("Review forward risk");
    expect(rail).not.toHaveTextContent("analyst@stress.example.com");
    expect(screen.queryByTestId("radar-evidence-why-now")).not.toBeInTheDocument();
  });

  it("does not lead leak_watch with zero metric tiles when activity is empty", async () => {
    mockFns.getRadarEvidence.mockResolvedValue({
      itemId: "act-leak-1",
      product: "leak_watch",
      headline: "Forward risk",
      whyNowCode: "leak_watch",
      metrics: {
        opens24h: 0,
        uniqueVisitors24h: 0,
        forwardSignals24h: 0,
        downloads24h: 0,
      },
    } satisfies RadarEvidencePack);

    await renderRail(
      makeItem({
        id: "act-leak-1",
        product: "leak_watch",
        headline: "Forward risk",
        whyNowCode: "leak_watch",
      }),
    );

    await waitFor(() => {
      expect(screen.getByTestId("radar-evidence-empty-primary")).toBeInTheDocument();
    });
    expect(screen.queryByTestId("radar-evidence-metrics")).not.toBeInTheDocument();
  });
});

