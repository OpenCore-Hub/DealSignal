import type { HeatLevel, Link } from "@/types";

const HEAT_RANK: Record<HeatLevel, number> = {
  cold: 0,
  warm: 1,
  hot: 2,
};

/**
 * Library-link fallback when document-native heat is unavailable.
 * Never derive from raw view-count thresholds. Do not attach room shares here.
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

export interface DocumentNativeHeatScore {
  id: string;
  views: number;
  heatLevel: HeatLevel;
}

/**
 * Overlay Insights-grain document heat onto library rows.
 * Does not attach shares — room links stay off the library list.
 */
export function overlayDocumentNativeHeat<
  T extends { id: string; heatLevel: HeatLevel; totalViews: number },
>(rows: T[], scores: DocumentNativeHeatScore[]): T[] {
  if (scores.length === 0) return rows;
  const byId = new Map(scores.map((score) => [score.id, score]));
  return rows.map((row) => {
    const score = byId.get(row.id);
    if (!score) return row;
    return { ...row, heatLevel: score.heatLevel, totalViews: score.views };
  });
}
