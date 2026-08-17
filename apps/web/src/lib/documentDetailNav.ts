export type DocumentDetailTab = "overview" | "content" | "analytics" | "insights";

const TABS = new Set<DocumentDetailTab>(["overview", "content", "analytics", "insights"]);

/** Best-effort page number from alert/signal metadata maps. */
export function parsePageFromMetadata(
  metadata?: Record<string, string> | null,
): number | null {
  if (!metadata) return null;
  for (const key of ["page_number", "pageNumber", "page"]) {
    const page = parseDocumentFocusPage(metadata[key]);
    if (page != null) return page;
  }
  return null;
}

/** Attributed document for bundle page deep links; empty when metadata has none. */
export function parseDocumentIdFromMetadata(
  metadata?: Record<string, string> | null,
): string | null {
  if (!metadata) return null;
  for (const key of ["document_id", "documentId"]) {
    const id = metadata[key]?.trim();
    if (id) return id;
  }
  return null;
}

/** Share/signal evidence path: attributed document + page, else primary, else link. */
export function documentEvidencePath(
  workspaceSlug: string,
  opts: {
    documentId?: string;
    linkId?: string;
    metadata?: Record<string, string> | null;
  },
): string | undefined {
  const documentId = parseDocumentIdFromMetadata(opts.metadata) || opts.documentId?.trim();
  if (documentId) {
    const page = parsePageFromMetadata(opts.metadata);
    return documentDetailPath(
      workspaceSlug,
      documentId,
      page ? { tab: "content", page } : { tab: "analytics" },
    );
  }
  const linkId = opts.linkId?.trim();
  if (linkId) return `/${workspaceSlug}/links/${linkId}`;
  return undefined;
}

/** Legacy `ai` tab renamed to reading insights. */
const TAB_ALIASES: Record<string, DocumentDetailTab> = {
  ai: "insights",
};

export function parseDocumentDetailTab(raw: string | null | undefined): DocumentDetailTab {
  if (!raw) return "overview";
  if (TABS.has(raw as DocumentDetailTab)) {
    return raw as DocumentDetailTab;
  }
  return TAB_ALIASES[raw] ?? "overview";
}

/** True when the raw query value is a legacy alias that should be rewritten. */
export function isLegacyDocumentDetailTab(raw: string | null | undefined): boolean {
  return raw != null && raw in TAB_ALIASES;
}

/** Positive integer page, optionally clamped to pageCount when known. */
export function parseDocumentFocusPage(
  raw: string | null | undefined,
  pageCount?: number,
): number | null {
  if (raw == null || raw === "") return null;
  const n = Number(raw);
  if (!Number.isInteger(n) || n < 1) return null;
  if (typeof pageCount === "number" && pageCount > 0 && n > pageCount) return null;
  return n;
}

export function documentDetailPath(
  workspaceSlug: string,
  documentId: string,
  opts?: { tab?: DocumentDetailTab; page?: number | null },
): string {
  const params = new URLSearchParams();
  const tab = opts?.tab ?? "overview";
  if (tab !== "overview") params.set("tab", tab);
  // Only emit page when navigating to the content tab (avoids stale ?page= on other tabs).
  if (tab === "content" && opts?.page != null && opts.page > 0) {
    params.set("page", String(opts.page));
  }
  const qs = params.toString();
  return qs
    ? `/${workspaceSlug}/documents/${documentId}?${qs}`
    : `/${workspaceSlug}/documents/${documentId}`;
}

/** Merge tab/page into existing search params (drops page when null). */
export function patchDocumentDetailSearchParams(
  current: URLSearchParams,
  patch: { tab?: DocumentDetailTab; page?: number | null },
): URLSearchParams {
  const next = new URLSearchParams(current);
  if (patch.tab !== undefined) {
    if (patch.tab === "overview") next.delete("tab");
    else next.set("tab", patch.tab);
  }
  if (patch.page !== undefined) {
    if (patch.page == null || patch.page < 1) next.delete("page");
    else next.set("page", String(patch.page));
  }
  // Leaving content without an explicit page keep/clear still drops stale page.
  const effectiveTab = parseDocumentDetailTab(next.get("tab"));
  if (effectiveTab !== "content" && patch.page === undefined && patch.tab !== undefined) {
    next.delete("page");
  }
  return next;
}
