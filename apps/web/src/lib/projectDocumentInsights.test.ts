import { describe, expect, it } from "vitest";
import { projectDocumentInsights } from "./projectDocumentInsights";
import type { PageAnalytics } from "@/types";

function pages(
  rows: Array<Partial<PageAnalytics> & { pageNumber: number }>,
): PageAnalytics[] {
  return rows.map((row) => ({
    pageNumber: row.pageNumber,
    viewCount: row.viewCount ?? 1,
    avgDurationSeconds: row.avgDurationSeconds ?? 0,
    exitRate: row.exitRate ?? 0,
  }));
}

describe("projectDocumentInsights", () => {
  it("returns empty when there is no page data", () => {
    expect(projectDocumentInsights([])).toEqual([]);
  });

  it("surfaces top dwell and high-exit pages", () => {
    const insights = projectDocumentInsights(
      pages([
        { pageNumber: 1, avgDurationSeconds: 10, exitRate: 0.05 },
        { pageNumber: 2, avgDurationSeconds: 40, exitRate: 0.1 },
        { pageNumber: 3, avgDurationSeconds: 8, exitRate: 0.4, viewCount: 5 },
      ]),
    );
    expect(insights[0]).toMatchObject({ kind: "top_dwell", pageNumber: 2 });
    expect(insights.some((i) => i.kind === "exit_risk" && i.pageNumber === 3)).toBe(
      true,
    );
  });

  it("adds sparse insight when most pages have no engagement", () => {
    const many = Array.from({ length: 20 }, (_, i) => ({
      pageNumber: i + 1,
      avgDurationSeconds: i < 2 ? 12 : 0,
      viewCount: i < 2 ? 2 : 0,
      exitRate: 0.05,
    }));
    const insights = projectDocumentInsights(pages(many));
    expect(insights.some((i) => i.kind === "sparse")).toBe(true);
  });
});
