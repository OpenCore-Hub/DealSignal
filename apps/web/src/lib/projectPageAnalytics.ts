import type { PageAnalytics } from "@/types";

export const PAGE_ANALYTICS_MAX_BARS = 48;
/** Above this, detail defaults to buckets until the user expands per-page. */
export const PAGE_ANALYTICS_DETAIL_SCROLL_CAP = 120;
export const PAGE_ANALYTICS_ZERO_RATIO = 0.6;
export const PAGE_ANALYTICS_TOP_N = 5;

export type PageAnalyticsVariant = "overview" | "detail";
export type PageAnalyticsFilter = "all" | "engaged";

export type PageAnalyticsFocusRange = {
  startPage: number;
  endPage: number;
};

export type PageAnalyticsBar =
  | {
      kind: "page";
      key: string;
      pageNumber: number;
      avgDurationSeconds: number;
      viewCount: number;
      zero: boolean;
    }
  | {
      kind: "bucket";
      key: string;
      startPage: number;
      endPage: number;
      avgDurationSeconds: number;
      maxDurationSeconds: number;
      pageCount: number;
    };

export type PageAnalyticsTopPage = {
  pageNumber: number;
  avgDurationSeconds: number;
  viewCount: number;
};

export type PageAnalyticsProjection = {
  strategy: "all" | "engaged" | "bucketed" | "scroll";
  bars: PageAnalyticsBar[];
  topPages: PageAnalyticsTopPage[];
  totalPages: number;
  engagedCount: number;
  zeroCount: number;
  hiddenZeroCount: number;
  /** Detail can offer "expand to per-page scroll" when bars are bucketed for size. */
  canExpandPerPage: boolean;
  sourceCount: number;
  /** Active drill-down window, or null for the full document. */
  focusRange: PageAnalyticsFocusRange | null;
};

function isEngaged(page: PageAnalytics): boolean {
  return page.avgDurationSeconds > 0 || page.viewCount > 0;
}

function toPageBar(page: PageAnalytics): PageAnalyticsBar {
  return {
    kind: "page",
    key: `p-${page.pageNumber}`,
    pageNumber: page.pageNumber,
    avgDurationSeconds: page.avgDurationSeconds,
    viewCount: page.viewCount,
    zero: !isEngaged(page),
  };
}

function bucketPages(pages: PageAnalytics[], maxBuckets: number): PageAnalyticsBar[] {
  if (pages.length === 0) return [];
  if (pages.length <= maxBuckets) return pages.map(toPageBar);

  const bucketCount = Math.min(maxBuckets, pages.length);
  const size = Math.ceil(pages.length / bucketCount);
  const bars: PageAnalyticsBar[] = [];

  for (let i = 0; i < pages.length; i += size) {
    const slice = pages.slice(i, i + size);
    if (slice.length === 0) continue;
    const durations = slice.map((p) => p.avgDurationSeconds);
    const avg =
      durations.reduce((sum, n) => sum + n, 0) / Math.max(durations.length, 1);
    const max = Math.max(...durations, 0);
    const startPage = slice[0]!.pageNumber;
    const endPage = slice[slice.length - 1]!.pageNumber;
    bars.push({
      kind: "bucket",
      key: `b-${startPage}-${endPage}`,
      startPage,
      endPage,
      avgDurationSeconds: Math.round(avg * 10) / 10,
      maxDurationSeconds: max,
      pageCount: slice.length,
    });
  }

  return bars;
}

function topPages(pages: PageAnalytics[], limit: number): PageAnalyticsTopPage[] {
  return [...pages]
    .filter(isEngaged)
    .sort((a, b) => {
      if (b.avgDurationSeconds !== a.avgDurationSeconds) {
        return b.avgDurationSeconds - a.avgDurationSeconds;
      }
      return b.viewCount - a.viewCount;
    })
    .slice(0, limit)
    .map((p) => ({
      pageNumber: p.pageNumber,
      avgDurationSeconds: p.avgDurationSeconds,
      viewCount: p.viewCount,
    }));
}

function emptyProjection(): PageAnalyticsProjection {
  return {
    strategy: "all",
    bars: [],
    topPages: [],
    totalPages: 0,
    engagedCount: 0,
    zeroCount: 0,
    hiddenZeroCount: 0,
    canExpandPerPage: false,
    sourceCount: 0,
    focusRange: null,
  };
}

function normalizeFocusRange(
  range: PageAnalyticsFocusRange | null | undefined,
  pages: PageAnalytics[],
): PageAnalyticsFocusRange | null {
  if (!range || pages.length === 0) return null;
  const minPage = pages[0]!.pageNumber;
  const maxPage = pages[pages.length - 1]!.pageNumber;
  const startPage = Math.max(minPage, Math.min(range.startPage, range.endPage));
  const endPage = Math.min(maxPage, Math.max(range.startPage, range.endPage));
  if (startPage > endPage) return null;
  // Full-span focus is a no-op (same as clearing).
  if (startPage === minPage && endPage === maxPage) return null;
  return { startPage, endPage };
}

function pagesInRange(
  pages: PageAnalytics[],
  range: PageAnalyticsFocusRange | null,
): PageAnalytics[] {
  if (!range) return pages;
  return pages.filter(
    (p) => p.pageNumber >= range.startPage && p.pageNumber <= range.endPage,
  );
}

/** Prefer "engaged" filter on detail when most pages have no dwell. */
export function defaultPageAnalyticsFilter(
  pages: PageAnalytics[],
  variant: PageAnalyticsVariant,
): PageAnalyticsFilter {
  if (variant !== "detail" || pages.length === 0) return "all";
  const zeroCount = pages.filter((p) => !isEngaged(p)).length;
  return zeroCount / pages.length >= PAGE_ANALYTICS_ZERO_RATIO ? "engaged" : "all";
}

/**
 * Project raw per-page analytics into a chart-friendly shape.
 * overview: glanceable (≤48 visual units, may hide zeros or bucket).
 * detail: exploratory (filter + scroll; buckets above scroll cap until expanded).
 * focusRange: drill into a page window (from clicking a bucket).
 */
export function projectPageAnalytics(
  pages: PageAnalytics[],
  options: {
    variant: PageAnalyticsVariant;
    filter?: PageAnalyticsFilter;
    expandPerPage?: boolean;
    focusRange?: PageAnalyticsFocusRange | null;
    maxBars?: number;
    scrollCap?: number;
    zeroRatioThreshold?: number;
    topN?: number;
  },
): PageAnalyticsProjection {
  const maxBars = options.maxBars ?? PAGE_ANALYTICS_MAX_BARS;
  const scrollCap = options.scrollCap ?? PAGE_ANALYTICS_DETAIL_SCROLL_CAP;
  const zeroRatioThreshold = options.zeroRatioThreshold ?? PAGE_ANALYTICS_ZERO_RATIO;
  const topN = options.topN ?? PAGE_ANALYTICS_TOP_N;
  const filter = options.filter ?? "all";
  const expandPerPage = options.expandPerPage ?? false;

  const sorted = [...pages].sort((a, b) => a.pageNumber - b.pageNumber);
  const totalPages = sorted.length;
  if (totalPages === 0) return emptyProjection();

  const focusRange = normalizeFocusRange(options.focusRange, sorted);
  const scoped = pagesInRange(sorted, focusRange);
  const engaged = scoped.filter(isEngaged);
  const engagedCount = engaged.length;
  const zeroCount = scoped.length - engagedCount;
  const ranking = topPages(scoped, topN);

  if (options.variant === "detail") {
    const source = filter === "engaged" ? engaged : scoped;
    const hiddenZeroCount = filter === "engaged" ? zeroCount : 0;
    const base = {
      topPages: ranking,
      totalPages,
      engagedCount,
      zeroCount,
      hiddenZeroCount,
      sourceCount: source.length,
      focusRange,
    };

    if (source.length === 0) {
      return {
        ...base,
        strategy: "engaged",
        bars: [],
        canExpandPerPage: false,
      };
    }
    if (source.length <= maxBars) {
      return {
        ...base,
        strategy: filter === "engaged" ? "engaged" : "all",
        bars: source.map(toPageBar),
        canExpandPerPage: false,
      };
    }
    if (source.length <= scrollCap || expandPerPage) {
      return {
        ...base,
        strategy: "scroll",
        bars: source.map(toPageBar),
        canExpandPerPage: source.length > scrollCap,
      };
    }
    return {
      ...base,
      strategy: "bucketed",
      bars: bucketPages(source, maxBars),
      canExpandPerPage: true,
    };
  }

  // overview (focusRange still applies when user drills from a bucket)
  const zeroRatio = scoped.length > 0 ? zeroCount / scoped.length : 0;
  const overviewBase = {
    topPages: ranking,
    totalPages,
    engagedCount,
    zeroCount,
    canExpandPerPage: false,
    sourceCount: scoped.length,
    focusRange,
  };

  if (scoped.length <= maxBars) {
    return {
      ...overviewBase,
      strategy: "all",
      bars: scoped.map(toPageBar),
      hiddenZeroCount: 0,
    };
  }

  if (zeroRatio >= zeroRatioThreshold && engagedCount > 0) {
    if (engagedCount <= maxBars) {
      return {
        ...overviewBase,
        strategy: "engaged",
        bars: engaged.map(toPageBar),
        hiddenZeroCount: zeroCount,
        sourceCount: engagedCount,
      };
    }
    return {
      ...overviewBase,
      strategy: "bucketed",
      bars: bucketPages(engaged, maxBars),
      hiddenZeroCount: zeroCount,
      sourceCount: engagedCount,
    };
  }

  return {
    ...overviewBase,
    strategy: "bucketed",
    bars: bucketPages(scoped, maxBars),
    hiddenZeroCount: 0,
  };
}

export function barDuration(bar: PageAnalyticsBar): number {
  return bar.kind === "bucket" ? bar.maxDurationSeconds : bar.avgDurationSeconds;
}
