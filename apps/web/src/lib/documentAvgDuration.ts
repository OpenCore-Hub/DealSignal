import type { Link, PageAnalytics } from "@/types";

function isBundleShare(link: Pick<Link, "isBundle" | "documents">): boolean {
  return Boolean(link.isBundle && (link.documents?.length ?? 0) > 1);
}

function meanLinkAvgDuration(links: Array<Pick<Link, "avgDurationSeconds">>): number {
  if (links.length === 0) return 0;
  return Math.round(
    links.reduce((sum, link) => sum + (link.avgDurationSeconds || 0), 0) / links.length,
  );
}

function weightedPageAvgDuration(
  pages: Array<Pick<PageAnalytics, "viewCount" | "avgDurationSeconds">>,
): number | null {
  let weighted = 0;
  let views = 0;
  for (const page of pages) {
    const n = page.viewCount ?? 0;
    if (n <= 0) continue;
    weighted += (page.avgDurationSeconds || 0) * n;
    views += n;
  }
  if (views <= 0) return null;
  return Math.round(weighted / views);
}

/**
 * Document-detail avg duration.
 * Prefer attributed page analytics (library + room + bundle) so deal-room
 * reads are not dropped when this file has no library share.
 * Fall back to library-link averages when pages have no views.
 */
export function documentAvgDurationSeconds(
  links: Array<Pick<Link, "avgDurationSeconds" | "isBundle" | "documents">>,
  pages?: Array<Pick<PageAnalytics, "viewCount" | "avgDurationSeconds">>,
): number {
  if (pages) {
    const scoped = weightedPageAvgDuration(pages);
    if (scoped != null) return scoped;
  }
  if (links.some(isBundleShare)) {
    return 0;
  }
  return meanLinkAvgDuration(links);
}
