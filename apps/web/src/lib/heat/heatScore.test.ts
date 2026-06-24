import { describe, expect, it } from "vitest";
import { computeHeatScore, summarizeLinkHeat } from "./heatScore";
import type { HeatScoreInput } from "./heatScore";
import type { PageAnalytics } from "@/types";

const baseInput: HeatScoreInput = {
  opens: 5,
  revisits: 2,
  avgDurationMinutes: 3,
  keyPageViews: 4,
  forwardSignals: 1,
  downloads: 0,
  bouncePenalty: 0,
};

function makePage(
  pageNumber: number,
  title: string,
  viewCount: number
): PageAnalytics {
  return {
    pageNumber,
    title,
    viewCount,
    avgDurationSeconds: 60,
    exitRate: 0.1,
  };
}

describe("computeHeatScore", () => {
  it("returns a score between 0 and 100", () => {
    const result = computeHeatScore("founder", baseInput);
    expect(result.score).toBeGreaterThanOrEqual(0);
    expect(result.score).toBeLessThanOrEqual(100);
    expect(["hot", "warm", "cold"]).toContain(result.level);
  });

  it("caps opens and bounce penalty components", () => {
    const input: HeatScoreInput = {
      ...baseInput,
      opens: 100,
      bouncePenalty: 100,
    };
    const result = computeHeatScore("founder", input);
    expect(result.breakdown.opens).toBe(10 * 3);
    expect(result.breakdown.bouncePenalty).toBe(-5 * 10);
  });

  it("matches top key pages by title keywords, not page number", () => {
    const pages: PageAnalytics[] = [
      makePage(1, "Cover page", 100),
      makePage(2, "Financial projections and revenue", 20),
      makePage(3, "Team and founders", 10),
    ];
    const result = computeHeatScore("founder", baseInput, pages);
    expect(result.topKeyPages).toContain("Financial projections and revenue");
    expect(result.topKeyPages).toContain("Team and founders");
    expect(result.topKeyPages).not.toContain("Cover page");
  });

  it("ranks key pages by relevance weighted by view count", () => {
    const pages: PageAnalytics[] = [
      makePage(1, "Financial projections", 5),
      makePage(2, "Team and founders", 100),
      makePage(3, "Market opportunity", 1),
    ];
    const result = computeHeatScore("founder", baseInput, pages);
    // "Team" (1 keyword match) * 100 views should outrank "Financial" (1 match) * 5 views.
    expect(result.topKeyPages[0]).toBe("Team and founders");
    expect(result.topKeyPages[1]).toBe("Financial projections");
  });

  it("limits top key pages to 3", () => {
    const pages: PageAnalytics[] = [
      makePage(1, "Financial projections", 1),
      makePage(2, "Team and founders", 1),
      makePage(3, "Market opportunity", 1),
      makePage(4, "Traction and growth", 1),
    ];
    const result = computeHeatScore("founder", baseInput, pages);
    expect(result.topKeyPages.length).toBe(3);
  });

  it("returns empty top key pages when no analytics provided", () => {
    const result = computeHeatScore("founder", baseInput);
    expect(result.topKeyPages).toEqual([]);
  });

  it("falls back to localized page number label when title is missing", () => {
    const pages: PageAnalytics[] = [
      { pageNumber: 7, title: "Financial projections", viewCount: 1, avgDurationSeconds: 0, exitRate: 0 },
      { pageNumber: 8, title: undefined, viewCount: 1, avgDurationSeconds: 0, exitRate: 0 },
    ];
    const result = computeHeatScore("founder", baseInput, pages);
    expect(result.topKeyPages).toContain("Financial projections");
    expect(result.topKeyPages).not.toContain("第 8 页");
  });
});

describe("summarizeLinkHeat", () => {
  it("derives input from link access metrics", () => {
    const link = {
      id: "link-1",
      accessCount: 10,
      avgDurationSeconds: 120,
    } as Parameters<typeof summarizeLinkHeat>[0];
    const result = summarizeLinkHeat(link, "sales");
    expect(result.score).toBeGreaterThanOrEqual(0);
    expect(result.topKeyPages).toEqual([]);
  });
});
