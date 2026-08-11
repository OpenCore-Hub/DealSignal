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
      "radar.evidenceRail.title": "Evidence",
      "radar.evidenceRail.empty": "Select a radar item",
      "radar.evidenceRail.loading": "Loading evidence…",
      "radar.evidenceRail.error": "Could not load evidence.",
      "radar.evidenceRail.degraded": "Some evidence facets failed to load.",
      "radar.evidenceRail.degradedSections.metrics": "24h metrics",
      "radar.evidenceRail.metrics.opens24h": "Opens (24h)",
      "radar.evidenceRail.metrics.visitors24h": "Visitors (24h)",
      "radar.evidenceRail.metrics.forwards24h": "Forwards (24h)",
      "radar.evidenceRail.metrics.downloads24h": "Downloads (24h)",
      "radar.evidenceRail.openFull": "Open full evidence",
      "radar.whyNow.leak_watch": "Leak risk in the last {{hours}}h",
      "radar.whyNow.fallback.leak_watch": "Review soon",
      "radar.headline.leak_watch": "Forward risk on Pitch",
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
      expect(screen.getByTestId("radar-evidence-why-now")).toBeInTheDocument();
    });
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
});
