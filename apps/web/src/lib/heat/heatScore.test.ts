import { describe, it, expect } from "vitest";
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

describe("computeHeatScore", () => {
  it("returns a score between 0 and 100", () => {
    const result = computeHeatScore("founder", baseInput);
    expect(result.score).toBeGreaterThanOrEqual(0);
    expect(result.score).toBeLessThanOrEqual(100);
  });

  it("classifies hot above threshold", () => {
    const input = { ...baseInput, opens: 20, revisits: 10, keyPageViews: 20 };
    const result = computeHeatScore("founder", input);
    expect(result.level).toBe("hot");
  });

  it("classifies cold below warm threshold", () => {
    const input: HeatScoreInput = {
      opens: 0,
      revisits: 0,
      avgDurationMinutes: 0,
      keyPageViews: 0,
      forwardSignals: 0,
      downloads: 0,
      bouncePenalty: 0,
    };
    const result = computeHeatScore("founder", input);
    expect(result.level).toBe("cold");
  });

  it("detects top key pages by title", () => {
    const pages: PageAnalytics[] = [
      { pageNumber: 1, title: "Cover", viewCount: 10, avgDurationSeconds: 10, exitRate: 0 },
      { pageNumber: 2, title: "Financial Projections", viewCount: 20, avgDurationSeconds: 30, exitRate: 0 },
      { pageNumber: 3, title: "Team", viewCount: 15, avgDurationSeconds: 20, exitRate: 0 },
    ];
    const result = computeHeatScore("founder", baseInput, pages);
    expect(result.topKeyPages).toContain("Financial Projections");
    expect(result.topKeyPages).toContain("Team");
  });

  it("limits top key pages to 3", () => {
    const pages: PageAnalytics[] = Array.from({ length: 10 }).map((_, i) => ({
      pageNumber: i + 1,
      title: `Financial ${i + 1}`,
      viewCount: 100 - i,
      avgDurationSeconds: 10,
      exitRate: 0,
    }));
    const result = computeHeatScore("founder", baseInput, pages);
    expect(result.topKeyPages.length).toBeLessThanOrEqual(3);
  });
});

describe("summarizeLinkHeat", () => {
  it("calculates a score from link access count", () => {
    const link = {
      id: "l1",
      documentId: "d1",
      documentTitle: "Deck",
      shortUrl: "https://example.com/x",
      accessCount: 10,
      avgDurationSeconds: 120,
      heatLevel: "warm" as const,
      createdAt: new Date().toISOString(),
    };
    const result = summarizeLinkHeat(link);
    expect(result.score).toBeGreaterThan(0);
  });
});
