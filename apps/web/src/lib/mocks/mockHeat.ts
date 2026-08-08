import {
  computeHeatScore,
  keyPageRulesForCircle,
  type HeatScoreInput,
} from "@/lib/heat/heatScore";
import type { Circle, HeatLevel, HeatScoreResult, Link, PageAnalytics } from "@/types";

function keyKeywordsForCircle(circle: Circle): string[] {
  return keyPageRulesForCircle(circle).flatMap((r) => r.keywords);
}

/**
 * Build heat.Compute inputs from mock Link fields.
 * ForwardSignals stay 0 (marker-only — MSW has no access_logs markers).
 * Never invent alerts or inflate scores with constant offsets.
 */
export function mockHeatInputFromLink(
  link: Pick<Link, "accessCount" | "avgDurationSeconds">,
  pages?: PageAnalytics[],
  circle: Circle = "founder",
): HeatScoreInput {
  const opens = Math.max(0, link.accessCount ?? 0);
  const approxUV = opens > 0 ? Math.min(opens, Math.max(1, Math.ceil(opens / 3))) : 0;
  const revisits = Math.max(0, opens - approxUV);
  const avgDurationMinutes = Math.max(0, (link.avgDurationSeconds ?? 0) / 60);
  const keywords = keyKeywordsForCircle(circle);

  let keyPageViews = 0;
  if (pages && pages.length > 0) {
    for (const p of pages) {
      const title = (p.title ?? "").toLowerCase();
      const engaged = (p.viewCount ?? 0) > 0 && (p.avgDurationSeconds ?? 0) >= 3;
      if (!engaged) continue;
      if (keywords.some((kw) => title.includes(kw.toLowerCase()))) {
        keyPageViews += Math.max(1, Math.round(p.viewCount * 0.25));
      }
    }
  } else if (opens > 0 && avgDurationMinutes > 0) {
    // Conservative stand-in when page titles are unavailable.
    keyPageViews = Math.min(opens, Math.floor(opens * 0.2));
  }

  return {
    opens,
    revisits,
    avgDurationMinutes,
    keyPageViews,
    forwardSignals: 0,
    downloads: 0,
    bouncePenalty: opens > 0 && avgDurationMinutes === 0 ? Math.min(opens, 5) : 0,
  };
}

export function computeMockLinkHeat(
  link: Pick<Link, "accessCount" | "avgDurationSeconds">,
  circle: Circle = "founder",
  pages?: PageAnalytics[],
): HeatScoreResult {
  return computeHeatScore(circle, mockHeatInputFromLink(link, pages, circle), pages);
}

/** Align Link.heatLevel with founder-circle heat.Compute (same thresholds as API). */
export function syncMockLinkHeatLevels(
  links: Link[],
  pagesByDocument?: Record<string, PageAnalytics[]>,
  circle: Circle = "founder",
): void {
  for (const link of links) {
    const docId = link.documentId;
    const pages = docId && pagesByDocument ? pagesByDocument[docId] : undefined;
    link.heatLevel = computeMockLinkHeat(link, circle, pages).level;
  }
}

export function countLinksByHeatLevel(links: Link[]): Record<HeatLevel, number> {
  const counts: Record<HeatLevel, number> = { hot: 0, warm: 0, cold: 0 };
  for (const link of links) {
    const level = link.heatLevel ?? "cold";
    counts[level] = (counts[level] ?? 0) + 1;
  }
  return counts;
}
