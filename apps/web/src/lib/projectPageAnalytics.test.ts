import { describe, expect, it } from "vitest";
import {
  PAGE_ANALYTICS_DETAIL_SCROLL_CAP,
  PAGE_ANALYTICS_MAX_BARS,
  defaultPageAnalyticsFilter,
  projectPageAnalytics,
} from "./projectPageAnalytics";
import type { PageAnalytics } from "@/types";

function pages(count: number, engaged: (n: number) => boolean = () => true): PageAnalytics[] {
  return Array.from({ length: count }, (_, i) => {
    const pageNumber = i + 1;
    const on = engaged(pageNumber);
    return {
      pageNumber,
      viewCount: on ? 2 : 0,
      avgDurationSeconds: on ? pageNumber : 0,
      exitRate: 0,
    };
  });
}

describe("projectPageAnalytics", () => {
  it("overview keeps all bars when page count is small", () => {
    const projection = projectPageAnalytics(pages(10), { variant: "overview" });
    expect(projection.strategy).toBe("all");
    expect(projection.bars).toHaveLength(10);
    expect(projection.hiddenZeroCount).toBe(0);
    expect(projection.canExpandPerPage).toBe(false);
  });

  it("overview hides zeros when most pages have no dwell", () => {
    const projection = projectPageAnalytics(
      pages(100, (n) => n <= 5),
      { variant: "overview" },
    );
    expect(projection.strategy).toBe("engaged");
    expect(projection.bars).toHaveLength(5);
    expect(projection.hiddenZeroCount).toBe(95);
    expect(projection.topPages[0]?.pageNumber).toBe(5);
  });

  it("overview buckets dense long documents", () => {
    const projection = projectPageAnalytics(pages(120), { variant: "overview" });
    expect(projection.strategy).toBe("bucketed");
    expect(projection.bars.length).toBeLessThanOrEqual(PAGE_ANALYTICS_MAX_BARS);
    expect(projection.bars[0]?.kind).toBe("bucket");
  });

  it("detail scrolls between maxBars and scrollCap", () => {
    const projection = projectPageAnalytics(pages(80), {
      variant: "detail",
      filter: "all",
    });
    expect(projection.strategy).toBe("scroll");
    expect(projection.bars).toHaveLength(80);
    expect(projection.canExpandPerPage).toBe(false);
  });

  it("detail buckets above scrollCap until expanded", () => {
    const count = PAGE_ANALYTICS_DETAIL_SCROLL_CAP + 40;
    const collapsed = projectPageAnalytics(pages(count), {
      variant: "detail",
      filter: "all",
    });
    expect(collapsed.strategy).toBe("bucketed");
    expect(collapsed.canExpandPerPage).toBe(true);
    expect(collapsed.bars.length).toBeLessThanOrEqual(PAGE_ANALYTICS_MAX_BARS);

    const expanded = projectPageAnalytics(pages(count), {
      variant: "detail",
      filter: "all",
      expandPerPage: true,
    });
    expect(expanded.strategy).toBe("scroll");
    expect(expanded.bars).toHaveLength(count);
    expect(expanded.canExpandPerPage).toBe(true);
  });

  it("detail engaged filter drops zero pages", () => {
    const projection = projectPageAnalytics(pages(40, (n) => n % 2 === 0), {
      variant: "detail",
      filter: "engaged",
    });
    expect(projection.strategy).toBe("engaged");
    expect(projection.bars).toHaveLength(20);
    expect(projection.hiddenZeroCount).toBe(20);
  });

  it("defaultPageAnalyticsFilter prefers engaged when sparse", () => {
    expect(defaultPageAnalyticsFilter(pages(100, (n) => n <= 3), "detail")).toBe("engaged");
    expect(defaultPageAnalyticsFilter(pages(20), "detail")).toBe("all");
    expect(defaultPageAnalyticsFilter(pages(100, (n) => n <= 3), "overview")).toBe("all");
  });

  it("focusRange drills into a bucket window as per-page bars", () => {
    const projection = projectPageAnalytics(pages(160), {
      variant: "detail",
      filter: "all",
      focusRange: { startPage: 41, endPage: 52 },
    });
    expect(projection.focusRange).toEqual({ startPage: 41, endPage: 52 });
    expect(projection.strategy).toBe("all");
    expect(projection.bars).toHaveLength(12);
    expect(projection.bars.every((b) => b.kind === "page")).toBe(true);
    expect(projection.bars[0]).toMatchObject({ kind: "page", pageNumber: 41 });
    expect(projection.bars.at(-1)).toMatchObject({ kind: "page", pageNumber: 52 });
    expect(projection.topPages.every((p) => p.pageNumber >= 41 && p.pageNumber <= 52)).toBe(true);
  });

  it("focusRange spanning the full document is treated as no focus", () => {
    const projection = projectPageAnalytics(pages(40), {
      variant: "detail",
      filter: "all",
      focusRange: { startPage: 1, endPage: 40 },
    });
    expect(projection.focusRange).toBeNull();
    expect(projection.bars).toHaveLength(40);
  });

  it("overview focusRange still buckets oversized windows", () => {
    const projection = projectPageAnalytics(pages(200), {
      variant: "overview",
      focusRange: { startPage: 1, endPage: 160 },
    });
    expect(projection.focusRange).toEqual({ startPage: 1, endPage: 160 });
    expect(projection.strategy).toBe("bucketed");
    expect(projection.bars.length).toBeLessThanOrEqual(PAGE_ANALYTICS_MAX_BARS);
  });
});
