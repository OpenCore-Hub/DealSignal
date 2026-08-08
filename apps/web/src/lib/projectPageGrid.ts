import type { PageAnalytics } from "@/types";

/** Above this, content tab uses a virtualized row grid (avoids N signed-URL mounts). */
export const PAGE_GRID_VIRTUALIZE_THRESHOLD = 48;
export const PAGE_GRID_OVERSCAN_ROWS = 2;
export const PAGE_GRID_GAP_PX = 16;

/** Portrait letter-page fallback when page dimensions are unknown. */
export const DEFAULT_PAGE_ASPECT_RATIO = 3 / 4;
export const DEFAULT_PAGE_ASPECT_RATIO_CSS = "3 / 4";

/** Numeric width/height ratio for layout math (grid estimate, canvas fit). */
export function pageAspectRatio(
  width?: number | null,
  height?: number | null,
  fallback = DEFAULT_PAGE_ASPECT_RATIO,
): number {
  if (width != null && height != null && width > 0 && height > 0) {
    return width / height;
  }
  return fallback;
}

/** CSS `aspect-ratio` value preserving integer dimensions when possible. */
export function pageAspectRatioCSS(
  width?: number | null,
  height?: number | null,
  fallback = DEFAULT_PAGE_ASPECT_RATIO_CSS,
): string {
  if (width != null && height != null && width > 0 && height > 0) {
    return `${width} / ${height}`;
  }
  return fallback;
}

export type PageGridItem = {
  pageNumber: number;
  viewCount: number;
  avgDurationSeconds: number;
  exitRate: number;
};

/** Match Tailwind breakpoints used by the page grid (2/3/4/5/6). */
export function columnCountForWidth(width: number): number {
  if (width >= 1280) return 6;
  if (width >= 1024) return 5;
  if (width >= 768) return 4;
  if (width >= 640) return 3;
  return 2;
}

export function shouldVirtualizePageGrid(
  pageCount: number,
  threshold = PAGE_GRID_VIRTUALIZE_THRESHOLD,
): boolean {
  return pageCount > threshold;
}

export function buildPageGridItems(
  pageCount: number,
  analytics: PageAnalytics[],
): PageGridItem[] {
  const count = Math.max(0, Math.floor(pageCount));
  if (count === 0) return [];

  const byPage = new Map<number, PageAnalytics>();
  for (const row of analytics) {
    byPage.set(row.pageNumber, row);
  }

  const items: PageGridItem[] = new Array(count);
  for (let pageNumber = 1; pageNumber <= count; pageNumber++) {
    const analytic = byPage.get(pageNumber);
    items[pageNumber - 1] = {
      pageNumber,
      viewCount: analytic?.viewCount ?? 0,
      avgDurationSeconds: analytic?.avgDurationSeconds ?? 0,
      exitRate: analytic?.exitRate ?? 0,
    };
  }
  return items;
}

/** Card height is driven by the page aspect ratio; row height ≈ card + gap. */
export function estimatePageCardRowHeight(
  containerWidth: number,
  columns: number,
  aspectRatio = DEFAULT_PAGE_ASPECT_RATIO,
  gapPx = PAGE_GRID_GAP_PX,
): number {
  const cols = Math.max(1, columns);
  const width = Math.max(120, containerWidth);
  const colWidth = Math.max(72, (width - gapPx * (cols - 1)) / cols);
  const ratio = aspectRatio > 0 ? aspectRatio : DEFAULT_PAGE_ASPECT_RATIO;
  return Math.round(colWidth / ratio + gapPx);
}

export function pageGridRowCount(pageCount: number, columns: number): number {
  if (pageCount <= 0 || columns <= 0) return 0;
  return Math.ceil(pageCount / columns);
}
