import { describe, expect, it } from "vitest";
import {
  computeHeatScore,
  filterKeywordsByLang,
  keyPageRulesForCircle,
  keywordLangFromI18n,
} from "./heatScore";
import type { PageAnalytics } from "@/types";

const baseInput = {
  opens: 3,
  revisits: 1,
  avgDurationMinutes: 2,
  keyPageViews: 1,
  forwardSignals: 0,
  downloads: 0,
  bouncePenalty: 0,
};

describe("computeHeatScore", () => {
  it("scores within 0–100 and returns a level", () => {
    const result = computeHeatScore("founder", baseInput);
    expect(result.score).toBeGreaterThanOrEqual(0);
    expect(result.score).toBeLessThanOrEqual(100);
    expect(["hot", "warm", "cold"]).toContain(result.level);
  });

  it("applies bounce penalty as a negative component", () => {
    const input = { ...baseInput, bouncePenalty: 3 };
    const result = computeHeatScore("founder", input);
    expect(result.breakdown.bouncePenalty).toBeLessThan(0);
  });

  it("surfaces matching key pages by title keywords", () => {
    const pages: PageAnalytics[] = [
      { pageNumber: 1, title: "Cover", viewCount: 10, avgDurationSeconds: 0, exitRate: 0 },
      { pageNumber: 2, title: "Financial projections", viewCount: 5, avgDurationSeconds: 0, exitRate: 0 },
      { pageNumber: 3, title: "Team & founders", viewCount: 8, avgDurationSeconds: 0, exitRate: 0 },
    ];
    const result = computeHeatScore("founder", baseInput, pages);
    expect(result.topKeyPages).toContain("Financial projections");
    expect(result.topKeyPages).toContain("Team & founders");
  });

  it("matches Chinese key-page titles (same rules as API heat)", () => {
    const pages: PageAnalytics[] = [
      { pageNumber: 1, title: "封面", viewCount: 3, avgDurationSeconds: 0, exitRate: 0 },
      { pageNumber: 2, title: "财务模型与营收", viewCount: 5, avgDurationSeconds: 0, exitRate: 0 },
      { pageNumber: 3, title: "核心团队", viewCount: 4, avgDurationSeconds: 0, exitRate: 0 },
    ];
    const result = computeHeatScore("founder", baseInput, pages);
    expect(result.topKeyPages).toContain("财务模型与营收");
    expect(result.topKeyPages).toContain("核心团队");
    expect(result.topKeyPages).not.toContain("封面");
  });

  it("ranks key pages by relevance × views", () => {
    const pages: PageAnalytics[] = [
      { pageNumber: 1, title: "Market opportunity", viewCount: 1, avgDurationSeconds: 0, exitRate: 0 },
      { pageNumber: 2, title: "Market opportunity TAM", viewCount: 20, avgDurationSeconds: 0, exitRate: 0 },
    ];
    const result = computeHeatScore("founder", baseInput, pages);
    expect(result.topKeyPages[0]).toBe("Market opportunity TAM");
  });

  it("ignores pages without keyword hits", () => {
    const pages: PageAnalytics[] = [
      { pageNumber: 1, title: "Appendix", viewCount: 99, avgDurationSeconds: 0, exitRate: 0 },
    ];
    const result = computeHeatScore("founder", baseInput, pages);
    expect(result.topKeyPages).toEqual([]);
  });

  it("returns empty topKeyPages without page analytics", () => {
    const result = computeHeatScore("founder", baseInput);
    expect(result.topKeyPages).toEqual([]);
  });

  it("never emits hard-coded Chinese page labels in topKeyPages", () => {
    const pages: PageAnalytics[] = [
      { pageNumber: 7, title: "Financial projections", viewCount: 1, avgDurationSeconds: 0, exitRate: 0 },
      { pageNumber: 8, title: undefined, viewCount: 1, avgDurationSeconds: 0, exitRate: 0 },
    ];
    const result = computeHeatScore("founder", baseInput, pages);
    expect(result.topKeyPages).toContain("Financial projections");
    expect(result.topKeyPages.join(" ")).not.toMatch(/第\s*\d+\s*页/);
  });
});

describe("keyword language filter", () => {
  it("maps i18n locales", () => {
    expect(keywordLangFromI18n("en")).toBe("en");
    expect(keywordLangFromI18n("zh-CN")).toBe("zh");
    expect(keywordLangFromI18n("")).toBe("any");
  });

  it("filters built-in rules by language", () => {
    const en = keyPageRulesForCircle("founder", "en").find((r) => r.category === "financials")!;
    expect(en.keywords).toContain("financial");
    expect(en.keywords.some(isCJK)).toBe(false);

    const zh = keyPageRulesForCircle("founder", "zh").find((r) => r.category === "financials")!;
    expect(zh.keywords).toContain("财务");
    expect(zh.keywords.every(isCJK)).toBe(true);

    expect(filterKeywordsByLang(["financial", "财务"], "en")).toEqual(["financial"]);
  });
});

function isCJK(kw: string) {
  return /[\u3400-\u9fff]/.test(kw);
}
