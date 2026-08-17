import type { AccessLog } from "@/types";

export interface PageDurationPoint {
  page: number;
  duration: number;
  /** Unique x-axis / tooltip label. Used for bundle charts (doc × page). */
  label?: string;
}

export interface PageDurationDocument {
  id: string;
  title: string;
  pageCount: number;
}

export interface BuildPageDurationDataOptions {
  documents: PageDurationDocument[];
  primaryDocumentId?: string;
  formatBundleLabel?: (title: string, page: number) => string;
}

function keyOf(documentId: string, page: number): string {
  return `${documentId}\0${page}`;
}

function maxLoggedPage(
  groups: Map<string, { total: number; count: number }>,
  documentId: string,
): number {
  let max = 0;
  const prefix = `${documentId}\0`;
  for (const [key] of groups) {
    if (!key.startsWith(prefix)) continue;
    const page = Number(key.slice(prefix.length));
    if (Number.isFinite(page) && page > max) max = page;
  }
  return max;
}

/**
 * Average dwell per page. Bundle shares emit one bar per document×page so
 * colliding page numbers (xlsx p.3 vs PDF p.3) never merge.
 * Logs without documentId are attributed to the primary document only.
 */
export function buildPageDurationData(
  logs: AccessLog[],
  opts: BuildPageDurationDataOptions,
): PageDurationPoint[] {
  const docs = opts.documents.filter((d) => d.id?.trim());
  const primaryId = opts.primaryDocumentId?.trim() || docs[0]?.id;
  const isBundle = docs.length > 1;

  const groups = new Map<string, { total: number; count: number }>();
  for (const log of logs) {
    if (typeof log.pageNumber !== "number" || log.pageNumber <= 0) continue;
    const scopedId = log.documentId?.trim();
    const docId = scopedId || primaryId;
    if (!docId) continue;
    const existing = groups.get(keyOf(docId, log.pageNumber));
    if (existing) {
      existing.total += log.durationSeconds || 0;
      existing.count += 1;
    } else {
      groups.set(keyOf(docId, log.pageNumber), {
        total: log.durationSeconds || 0,
        count: 1,
      });
    }
  }

  const series: PageDurationDocument[] =
    docs.length > 0
      ? docs
      : primaryId
        ? [{ id: primaryId, title: "", pageCount: 0 }]
        : [];

  const known = new Set(series.map((d) => d.id));
  for (const key of groups.keys()) {
    const docId = key.slice(0, key.indexOf("\0"));
    if (!docId || known.has(docId)) continue;
    known.add(docId);
    series.push({ id: docId, title: docId, pageCount: 0 });
  }

  const data: PageDurationPoint[] = [];
  for (const doc of series) {
    const loggedMax = maxLoggedPage(groups, doc.id);
    const pageCount = doc.pageCount > 0 ? doc.pageCount : loggedMax;
    if (pageCount <= 0) continue;
    for (let page = 1; page <= pageCount; page++) {
      const existing = groups.get(keyOf(doc.id, page));
      const point: PageDurationPoint = {
        page,
        duration: existing ? Math.round(existing.total / existing.count) : 0,
      };
      if (isBundle && opts.formatBundleLabel) {
        point.label = opts.formatBundleLabel(doc.title || doc.id, page);
      }
      data.push(point);
    }
  }
  return data;
}

export interface PageDurationMetric {
  documentId?: string;
  pageNumber: number;
  avgDurationSeconds: number;
}

/** Build the share-detail chart from member-excluded page_views aggregates. */
export function buildPageDurationDataFromMetrics(
  pages: PageDurationMetric[],
  opts: BuildPageDurationDataOptions,
): PageDurationPoint[] {
  return buildPageDurationData(
    pages.map((page, index) => ({
      id: `metric-${index}`,
      linkId: "",
      visitorEmail: "",
      timestamp: "",
      pageNumber: page.pageNumber,
      documentId: page.documentId,
      durationSeconds: Math.round(page.avgDurationSeconds),
    })),
    opts,
  );
}
