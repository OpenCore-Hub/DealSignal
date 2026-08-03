import type { DealRoomKnowledgeQueryHit } from "@/types";

/** Locale-aware formatters for page/sheet locus labels (no hard-coded UI language). */
export interface LocusFormatters {
  sheetPrefix: string;
  pageSingle: (page: number) => string;
  pageRange: (from: number, to: number) => string;
  /** Join non-contiguous pages, e.g. "1、3" or "1, 3". */
  pageListSep: string;
  pageList: (joinedPages: string) => string;
}

/** Format page numbers without implying missing pages exist in the span. */
export function formatPagesLabel(pages: number[], fmt: LocusFormatters): string {
  const sorted = [...new Set(pages.filter((p) => p > 0))].sort((a, b) => a - b);
  if (sorted.length === 0) return "";
  if (sorted.length === 1) return fmt.pageSingle(sorted[0]);
  const lo = sorted[0];
  const hi = sorted[sorted.length - 1];
  const contiguous = hi - lo + 1 === sorted.length;
  if (contiguous) return fmt.pageRange(lo, hi);
  return fmt.pageList(sorted.join(fmt.pageListSep));
}

/** Human-readable citation locus: file · pages|sheet. Never invents missing pages. */
export function formatHitLocusLabel(
  hit: DealRoomKnowledgeQueryHit,
  fmt: LocusFormatters,
): string | null {
  const parts: string[] = [];
  if (hit.sourceName) parts.push(hit.sourceName);
  if (hit.pages && hit.pages.length > 0) {
    const pagesLabel = formatPagesLabel(hit.pages, fmt);
    if (pagesLabel) parts.push(pagesLabel);
  } else if (hit.sheet) {
    const prefix = fmt.sheetPrefix.trim();
    // Never fall back to a hard-coded locale word — caller must pass i18n sheetPrefix.
    parts.push(prefix ? `${prefix} ${hit.sheet}` : hit.sheet);
  }
  return parts.length ? parts.join(" · ") : null;
}

/** Same-tab viewer path (keeps in-memory workspace slug for authenticated APIs). */
export function viewerPath(documentId: string, page?: number): string {
  const qs = page && page > 0 ? `?page=${page}` : "";
  return `/viewer/${documentId}${qs}`;
}

/** Split answer text so `[n]` citations become clickable markers. */
export function renderAnswerWithCitations(
  answer: string,
  onCite: (n: number) => void,
) {
  const parts = answer.split(/(\[\d+\])/g);
  return parts.map((part, i) => {
    const m = /^\[(\d+)\]$/.exec(part);
    if (!m) return <span key={i}>{part}</span>;
    const n = Number(m[1]);
    return (
      <button
        key={i}
        type="button"
        className="mx-0.5 inline-flex h-5 min-w-5 items-center justify-center rounded-sm bg-foreground/[0.06] px-1 align-baseline font-mono text-[11px] font-semibold text-foreground transition-colors hover:bg-foreground/10"
        data-testid={`knowledge-cite-${n}`}
        onClick={() => onCite(n)}
      >
        {n}
      </button>
    );
  });
}
