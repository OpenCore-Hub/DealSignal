import type { PageAnalytics } from "@/types";

/** Exit rate at/above this is surfaced as an optimization risk. */
export const DOCUMENT_INSIGHT_EXIT_THRESHOLD = 0.25;
export const DOCUMENT_INSIGHT_MAX_EXIT = 2;
export const DOCUMENT_INSIGHT_SPARSE_RATIO = 0.6;

export type DocumentInsight =
  | {
      kind: "top_dwell";
      key: string;
      pageNumber: number;
      avgDurationSeconds: number;
      viewCount: number;
    }
  | {
      kind: "exit_risk";
      key: string;
      pageNumber: number;
      exitRate: number;
      viewCount: number;
    }
  | {
      kind: "sparse";
      key: string;
      engagedCount: number;
      totalPages: number;
      zeroCount: number;
    };

function isEngaged(page: PageAnalytics): boolean {
  return page.avgDurationSeconds > 0 || page.viewCount > 0;
}

/**
 * Derive deterministic reading insights from per-page analytics.
 * No LLM — only engagement facts owners can act on.
 */
export function projectDocumentInsights(pages: PageAnalytics[]): DocumentInsight[] {
  const sorted = [...pages].sort((a, b) => a.pageNumber - b.pageNumber);
  if (sorted.length === 0) return [];

  const engaged = sorted.filter(isEngaged);
  const insights: DocumentInsight[] = [];

  if (engaged.length > 0) {
    const top = [...engaged].sort((a, b) => {
      if (b.avgDurationSeconds !== a.avgDurationSeconds) {
        return b.avgDurationSeconds - a.avgDurationSeconds;
      }
      return b.viewCount - a.viewCount;
    })[0]!;
    insights.push({
      kind: "top_dwell",
      key: `top-${top.pageNumber}`,
      pageNumber: top.pageNumber,
      avgDurationSeconds: top.avgDurationSeconds,
      viewCount: top.viewCount,
    });
  }

  const exitRisks = engaged
    .filter((p) => p.exitRate >= DOCUMENT_INSIGHT_EXIT_THRESHOLD)
    .sort((a, b) => {
      if (b.exitRate !== a.exitRate) return b.exitRate - a.exitRate;
      return b.viewCount - a.viewCount;
    })
    .slice(0, DOCUMENT_INSIGHT_MAX_EXIT);

  for (const page of exitRisks) {
    insights.push({
      kind: "exit_risk",
      key: `exit-${page.pageNumber}`,
      pageNumber: page.pageNumber,
      exitRate: page.exitRate,
      viewCount: page.viewCount,
    });
  }

  const zeroCount = sorted.length - engaged.length;
  if (
    engaged.length > 0 &&
    zeroCount / sorted.length >= DOCUMENT_INSIGHT_SPARSE_RATIO
  ) {
    insights.push({
      kind: "sparse",
      key: "sparse",
      engagedCount: engaged.length,
      totalPages: sorted.length,
      zeroCount,
    });
  }

  return insights;
}
