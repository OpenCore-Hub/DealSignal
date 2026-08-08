import type { HeatLevel, Link } from "@/types";

const HEAT_RANK: Record<HeatLevel, number> = {
  cold: 0,
  warm: 1,
  hot: 2,
};

/**
 * Document heat for library lists: max heat.Compute level across the
 * document's share links. Never derive from raw view-count thresholds.
 */
export function documentHeatFromLinks(links: Link[]): HeatLevel {
  let best: HeatLevel = "cold";
  for (const link of links) {
    const level = link.heatLevel ?? "cold";
    if (HEAT_RANK[level] > HEAT_RANK[best]) {
      best = level;
    }
  }
  return best;
}
