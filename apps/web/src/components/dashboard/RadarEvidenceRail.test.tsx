// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
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
      "radar.evidenceRail.metrics.linkLevel":
        "These counts are for this share over the last 24 hours, not the person held at the gate.",
      "radar.evidenceRail.shareActivity.summary":
        "Share activity (this link, not the person held)",
      "radar.evidenceRail.recentVisitors": "Recent visitors",
      "radar.evidenceRail.keyPages": "Key pages",
      "radar.evidenceRail.topPages": "Top pages",
      "radar.evidenceRail.page": "Page {{page}}",
      "radar.evidenceRail.pageOnDocument": "{{title}} · p.{{page}}",
      "radar.evidenceRail.securityEvents": "Security events",
      "radar.evidenceRail.eventTypes.forward_signal": "Forward signal",
      "radar.evidenceRail.eventTypes.not_in_allow_list": "Not on allow list",
      "radar.evidenceRail.reasons.abnormal_access": "Abnormal access",
      "radar.evidenceRail.views_one": "{{count}} view",
      "radar.evidenceRail.views_other": "{{count}} views",
      "radar.evidenceRail.openFull": "Open full evidence",
      "radar.evidenceRail.openShareInbox": "Review in Share",
      "radar.evidenceRail.gateEvents": "Gate events",
      "radar.evidenceRail.gateTimeline.beforeAndAfter":
        "Hit the gate {{before}}× before requesting, then {{after}}× after.",
      "radar.evidenceRail.gateTimeline.beforeAndAfterPending":
        "Hit the gate {{before}}× before requesting, then {{after}}× after. The request is still waiting for approval, so more holds are expected.",
      "radar.evidenceRail.gateTimeline.beforeOnly":
        "Hit the gate {{before}}× before requesting access.",
      "radar.evidenceRail.gateTimeline.beforeOnlyPending":
        "Hit the gate {{before}}× before requesting access. The request is still waiting for approval.",
      "radar.evidenceRail.gateTimeline.afterOnly":
        "Requested access, then hit the gate {{after}}×.",
      "radar.evidenceRail.gateTimeline.afterOnlyPending":
        "Requested access, then hit the gate {{after}}×. The request is still waiting for approval, so more holds are expected.",
      "radar.evidenceRail.gateTimeline.eventsOnly":
        "Hit the gate {{total}}×. The allowlist is still in effect.",
      "radar.evidenceRail.gateTimeline.eventsOnlyPending":
        "Hit the gate {{total}}×. The request is still waiting for approval.",
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
      "radar.evidenceRail.gatePromptBadge": "Gate prompt · not a hold",
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
    expect(screen.queryByTestId("radar-evidence-metrics-link-level")).not.toBeInTheDocument();
    expect(screen.queryByTestId("radar-evidence-share-activity")).not.toBeInTheDocument();
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
    expect(screen.queryByTestId("radar-evidence-gate-timeline")).not.toBeInTheDocument();
    const events = screen.getByTestId("radar-evidence-security-events");
    expect(events).toHaveTextContent("Email verification required");
    expect(events).toHaveTextContent("Gate prompt · not a hold");
    expect(events.querySelector("[data-gate-prompt='true']")).toBeTruthy();
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

  it("states pending approval on a real hold after the request", async () => {
    mockFns.getRadarEvidence.mockResolvedValue({
      itemId: "act-gate-hold-1",
      product: "diligence_gate",
      headline: "An investor is still waiting to enter",
      whyNowCode: "diligence_gate",
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
          eventType: "not_in_allow_list",
          email: "lp@vc.com",
          createdAt: "2026-08-11T10:23:33Z",
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
        id: "act-gate-hold-1",
        product: "diligence_gate",
        headline: "An investor is still waiting to enter",
        headlineCode: "radar.headline.diligence_gate",
        whyNowCode: "diligence_gate",
        verb: "review",
      }),
    );

    await waitFor(() => {
      expect(screen.getByTestId("radar-evidence-gate-timeline")).toHaveTextContent(
        "Requested access, then hit the gate 1×. The request is still waiting for approval, so more holds are expected.",
      );
    });
    expect(screen.getByTestId("radar-evidence-gate-timeline")).not.toHaveTextContent(
      "still blocked",
    );
  });

  it("states the allowlist is still in effect when there is no access request", async () => {
    mockFns.getRadarEvidence.mockResolvedValue({
      itemId: "act-gate-allowlist",
      product: "diligence_gate",
      headline: "Waiting to enter",
      whyNowCode: "diligence_gate",
      metrics: {
        opens24h: 0,
        uniqueVisitors24h: 0,
        forwardSignals24h: 0,
        downloads24h: 0,
      },
      securityEvents: [
        {
          eventType: "not_in_allow_list",
          email: "yqx-401@126.com",
          createdAt: "2026-08-11T17:00:00Z",
        },
      ],
    } satisfies RadarEvidencePack);

    await renderRail(
      makeItem({
        id: "act-gate-allowlist",
        product: "diligence_gate",
        verb: "review",
        actor: "yqx-401@126.com",
        headline: "Waiting to enter",
      }),
    );

    await waitFor(() => {
      expect(screen.getByTestId("radar-evidence-gate-timeline")).toHaveTextContent(
        "Hit the gate 1×. The allowlist is still in effect.",
      );
    });
    expect(screen.getByTestId("radar-evidence-gate-timeline")).not.toHaveTextContent(
      "still blocked",
    );
    expect(screen.getByTestId("radar-evidence-gate-timeline")).not.toHaveTextContent(
      "waiting for approval",
    );
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
    expect(screen.getByTestId("radar-evidence-top-pages")).toHaveTextContent("Page 3");
    expect(screen.getByTestId("radar-evidence-top-pages").querySelector("a")).toBeNull();
  });

  it("disambiguates bundle top pages with the same page number", async () => {
    mockFns.getRadarEvidence.mockResolvedValue({
      itemId: "act-bundle-pages",
      product: "leak_watch",
      headline: "Forward risk",
      whyNowCode: "leak_watch",
      metrics: {
        opens24h: 2,
        uniqueVisitors24h: 1,
        forwardSignals24h: 1,
        downloads24h: 0,
      },
      topPages: [
        {
          documentId: "doc-xlsx",
          documentTitle: "Financials.xlsx",
          pageNumber: 8,
          views: 4,
          avgDurationSeconds: 20,
        },
        {
          documentId: "doc-pdf",
          documentTitle: "Deck.pdf",
          pageNumber: 8,
          views: 9,
          avgDurationSeconds: 30,
        },
      ],
    } satisfies RadarEvidencePack);

    await renderRail(makeItem({ id: "act-bundle-pages" }));

    await waitFor(() => {
      expect(screen.getByTestId("radar-evidence-top-pages")).toBeInTheDocument();
    });
    const rail = screen.getByTestId("radar-evidence-top-pages");
    expect(rail).toHaveTextContent("Financials.xlsx · p.8");
    expect(rail).toHaveTextContent("Deck.pdf · p.8");
    expect(rail).not.toHaveTextContent("Page 8");
    expect(screen.getByRole("link", { name: "Financials.xlsx · p.8" })).toHaveAttribute(
      "href",
      "/acme/documents/doc-xlsx?tab=content&page=8",
    );
    expect(screen.getByRole("link", { name: "Deck.pdf · p.8" })).toHaveAttribute(
      "href",
      "/acme/documents/doc-pdf?tab=content&page=8",
    );
    expect(screen.queryByTestId("radar-evidence-open")).not.toBeInTheDocument();
  });

  it("opens share-level evidence for a multi-document leak_watch, not one PDF", async () => {
    mockFns.getRadarEvidence.mockResolvedValue({
      itemId: "act-bundle-open",
      product: "leak_watch",
      headline: "Forward risk",
      whyNowCode: "leak_watch",
      navigatePath: "/acme/documents/doc-xlsx?tab=analytics",
      evidencePath: "/acme/documents/doc-xlsx?tab=content&page=8",
      insightsPath: "/acme/links/link-room",
      topPages: [
        {
          documentId: "doc-xlsx",
          documentTitle: "Financials.xlsx",
          pageNumber: 8,
          views: 4,
          avgDurationSeconds: 20,
        },
        {
          documentId: "doc-pdf",
          documentTitle: "Deck.pdf",
          pageNumber: 8,
          views: 9,
          avgDurationSeconds: 30,
        },
      ],
    } satisfies RadarEvidencePack);

    await renderRail(makeItem({ id: "act-bundle-open" }));

    await waitFor(() => {
      expect(screen.getByTestId("radar-evidence-open")).toBeInTheDocument();
    });
    expect(screen.getByTestId("radar-evidence-open")).toHaveAttribute(
      "href",
      "/acme/links/link-room",
    );
    expect(screen.getByTestId("radar-evidence-open")).toHaveTextContent("Open full evidence");
    expect(screen.getByRole("link", { name: "Deck.pdf · p.8" })).toHaveAttribute(
      "href",
      "/acme/documents/doc-pdf?tab=content&page=8",
    );
  });

  it("sends a deal-room allowlist hold to Access, not a document", async () => {
    mockFns.getRadarEvidence.mockResolvedValue({
      itemId: "act-gate-room",
      product: "diligence_gate",
      headline: "Review allow list",
      whyNowCode: "diligence_gate",
      navigatePath: "/acme/documents/doc-1?tab=analytics",
      insightsPath: "/acme/links/link-room",
      linkId: "link-room",
      securityEvents: [
        {
          eventType: "not_in_allow_list",
          createdAt: "2026-08-11T17:00:00Z",
        },
      ],
    } satisfies RadarEvidencePack);

    await renderRail(
      makeItem({
        id: "act-gate-room",
        product: "diligence_gate",
        verb: "review",
        dealRoomId: "room-1",
        linkId: "link-room",
      }),
    );

    await waitFor(() => {
      expect(screen.getByTestId("radar-evidence-open")).toBeInTheDocument();
    });
    expect(screen.getByTestId("radar-evidence-open")).toHaveAttribute(
      "href",
      "/acme/deal-rooms/room-1?tab=access&linkId=link-room",
    );
    expect(screen.getByTestId("radar-evidence-open")).toHaveTextContent("Review in Share");
  });

  it("sends a document-library gate hold to the share link, not the request inbox", async () => {
    mockFns.getRadarEvidence.mockResolvedValue({
      itemId: "act-gate-lib",
      product: "diligence_gate",
      headline: "Review block",
      whyNowCode: "diligence_gate",
      navigatePath: "/acme/documents/doc-1?tab=analytics",
      insightsPath: "/acme/links/link-1",
      linkId: "link-1",
      securityEvents: [
        {
          eventType: "not_in_allow_list",
          createdAt: "2026-08-11T17:00:00Z",
        },
      ],
    } satisfies RadarEvidencePack);

    await renderRail(
      makeItem({
        id: "act-gate-lib",
        product: "diligence_gate",
        verb: "review",
        linkId: "link-1",
      }),
    );

    await waitFor(() => {
      expect(screen.getByTestId("radar-evidence-open")).toBeInTheDocument();
    });
    expect(screen.getByTestId("radar-evidence-open")).toHaveAttribute(
      "href",
      "/acme/links/link-1",
    );
    expect(screen.getByTestId("radar-evidence-open")).toHaveTextContent(
      "Open full evidence",
    );
  });

  it("keeps solo-share top page labels as Page N when a document id is present", async () => {
    mockFns.getRadarEvidence.mockResolvedValue({
      itemId: "act-solo-page",
      product: "leak_watch",
      headline: "Forward risk",
      whyNowCode: "leak_watch",
      metrics: {
        opens24h: 2,
        uniqueVisitors24h: 1,
        forwardSignals24h: 1,
        downloads24h: 0,
      },
      topPages: [
        {
          documentId: "doc-deck",
          documentTitle: "Pitch Deck",
          pageNumber: 3,
          views: 12,
          avgDurationSeconds: 40,
        },
        {
          documentId: "doc-deck",
          documentTitle: "Pitch Deck",
          pageNumber: 1,
          views: 8,
          avgDurationSeconds: 22,
        },
      ],
    } satisfies RadarEvidencePack);

    await renderRail(makeItem({ id: "act-solo-page" }));

    await waitFor(() => {
      expect(screen.getByTestId("radar-evidence-top-pages")).toBeInTheDocument();
    });
    const rail = screen.getByTestId("radar-evidence-top-pages");
    expect(rail).toHaveTextContent("Page 3");
    expect(rail).toHaveTextContent("Page 1");
    expect(rail).not.toHaveTextContent("Pitch Deck");
    expect(screen.getByRole("link", { name: "Page 3" })).toHaveAttribute(
      "href",
      "/acme/documents/doc-deck?tab=content&page=3",
    );
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

  it("labels 24h metrics as share-level on a waiting-to-enter card", async () => {
    mockFns.getRadarEvidence.mockResolvedValue({
      itemId: "act-gate-hold",
      product: "diligence_gate",
      headline: "Waiting to enter",
      metrics: {
        opens24h: 5,
        uniqueVisitors24h: 2,
        forwardSignals24h: 0,
        downloads24h: 0,
      },
      securityEvents: [
        {
          eventType: "not_in_allow_list",
          email: "yqx-401@126.com",
          createdAt: "2026-08-11T17:00:00Z",
        },
      ],
    } satisfies RadarEvidencePack);

    await renderRail(
      makeItem({
        id: "act-gate-hold",
        product: "diligence_gate",
        verb: "review",
        actor: "张姐",
        headline: "Waiting to enter",
      }),
    );

    await waitFor(() => {
      expect(screen.getByTestId("radar-evidence-share-activity")).toBeInTheDocument();
    });
    const fold = screen.getByTestId("radar-evidence-share-activity");
    expect(fold).not.toHaveAttribute("open");
    expect(screen.getByTestId("radar-evidence-share-activity-summary")).toHaveTextContent(
      "Share activity (this link, not the person held)",
    );
    expect(screen.getByTestId("radar-evidence-metrics-link-level")).toHaveTextContent(
      "not the person held at the gate",
    );
    expect(screen.getByTestId("radar-evidence-security-events")).toHaveTextContent(
      "yqx-401@126.com",
    );
    fireEvent.click(screen.getByTestId("radar-evidence-share-activity-summary"));
    expect(fold).toHaveAttribute("open");
    expect(screen.getByTestId("radar-evidence-metrics")).toHaveTextContent("5");
  });

  it("folds visitors who got in on a waiting-to-enter card", async () => {
    mockFns.getRadarEvidence.mockResolvedValue({
      itemId: "act-gate-visitors",
      product: "diligence_gate",
      headline: "Waiting to enter",
      metrics: {
        opens24h: 0,
        uniqueVisitors24h: 0,
        forwardSignals24h: 0,
        downloads24h: 0,
      },
      recentVisitors: [
        {
          visitorId: "v-other",
          email: "analyst@acme.test",
          totalViews: 4,
          lastAccessAt: "2026-08-11T18:00:00Z",
        },
      ],
      securityEvents: [
        {
          eventType: "not_in_allow_list",
          email: "yqx-401@126.com",
          createdAt: "2026-08-11T17:00:00Z",
        },
      ],
    } satisfies RadarEvidencePack);

    await renderRail(
      makeItem({
        id: "act-gate-visitors",
        product: "diligence_gate",
        verb: "review",
        actor: "yqx-401@126.com",
        headline: "Waiting to enter",
      }),
    );

    await waitFor(() => {
      expect(screen.getByTestId("radar-evidence-share-activity")).toBeInTheDocument();
    });
    const fold = screen.getByTestId("radar-evidence-share-activity");
    expect(fold).not.toHaveAttribute("open");
    expect(fold).toContainElement(screen.getByTestId("radar-evidence-recent-visitors"));
    expect(screen.queryByTestId("radar-evidence-metrics")).not.toBeInTheDocument();
    expect(screen.getByTestId("radar-evidence-recent-visitors")).toHaveTextContent(
      "analyst@acme.test",
    );
  });

  it("does not fold share activity on a pending-approve waiting-to-enter card", async () => {
    mockFns.getRadarEvidence.mockResolvedValue({
      itemId: "act-gate-approve",
      product: "diligence_gate",
      headline: "Approve access request",
      metrics: {
        opens24h: 5,
        uniqueVisitors24h: 2,
        forwardSignals24h: 0,
        downloads24h: 0,
      },
    } satisfies RadarEvidencePack);

    await renderRail(
      makeItem({
        id: "act-gate-approve",
        product: "diligence_gate",
        verb: "approve",
        headline: "Approve access request",
      }),
    );

    await waitFor(() => {
      expect(screen.getByTestId("radar-evidence-metrics")).toBeInTheDocument();
    });
    expect(screen.queryByTestId("radar-evidence-share-activity")).not.toBeInTheDocument();
    expect(
      screen.queryByTestId("radar-evidence-metrics-link-level"),
    ).not.toBeInTheDocument();
    expect(screen.getByTestId("radar-evidence-metrics")).toHaveTextContent("5");
  });

  it("states pending approval when holds are entirely before the request", async () => {
    mockFns.getRadarEvidence.mockResolvedValue({
      itemId: "act-gate-before-pending",
      product: "diligence_gate",
      headline: "Waiting to enter",
      whyNowCode: "diligence_gate",
      accessRequest: {
        email: "lp@vc.com",
        reason: "Need the deck for IC",
        status: "pending",
        requestedAt: "2026-08-11T18:00:00Z",
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
          eventType: "not_in_allow_list",
          email: "lp@vc.com",
          createdAt: "2026-08-11T17:00:00Z",
        },
      ],
    } satisfies RadarEvidencePack);

    await renderRail(
      makeItem({
        id: "act-gate-before-pending",
        product: "diligence_gate",
        verb: "review",
        headline: "Waiting to enter",
      }),
    );

    await waitFor(() => {
      expect(screen.getByTestId("radar-evidence-gate-timeline")).toHaveTextContent(
        "Hit the gate 1× before requesting access. The request is still waiting for approval.",
      );
    });
    expect(screen.getByTestId("radar-evidence-gate-timeline")).not.toHaveTextContent(
      "allowlist",
    );
  });
});

