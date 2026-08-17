import {
  isLinkCreatedWithin,
  SHARE_CREATED_WITHIN_PARAM,
  SHARE_SEARCH_PARAM,
} from "@/lib/shareLinksFilter";

const LIBRARY_TABS = new Set(["shared", "archived"]);

/** Share-tab list filters — only valid with tab=shared. */
const SHARE_LIST_FILTER_PARAMS = [SHARE_SEARCH_PARAM, SHARE_CREATED_WITHIN_PARAM] as const;

/** Canonical path for the Document Library → Share (links) tab. */
export function documentsSharePath(
  workspaceSlug: string,
  opts?: { documentId?: string; documentTitle?: string; linkId?: string },
): string {
  const params = new URLSearchParams();
  params.set("tab", "shared");
  if (opts?.documentId) {
    params.set("documentId", opts.documentId);
  }
  if (opts?.documentTitle) {
    params.set("documentTitle", opts.documentTitle);
  }
  if (opts?.linkId) {
    params.set("linkId", opts.linkId);
  }
  return `/${workspaceSlug}/documents?${params.toString()}`;
}

/** Create-link pipeline entry (kept under /links/new). */
export function documentsCreateLinkPath(
  workspaceSlug: string,
  opts?: { documentId?: string; documentIds?: string[] },
): string {
  const ids = [
    ...new Set(
      [...(opts?.documentIds ?? []), opts?.documentId ?? ""]
        .map((id) => id.trim())
        .filter(Boolean),
    ),
  ];
  const base = `/${workspaceSlug}/links/new`;
  if (ids.length === 0) return base;
  const params = new URLSearchParams();
  for (const id of ids) params.append("documentId", id);
  return `${base}?${params.toString()}`;
}

/**
 * Normalize Document Library query params.
 * Returns a new URLSearchParams when cleanup is needed; otherwise null.
 */
export function sanitizeDocumentsLibrarySearchParams(
  input: URLSearchParams,
): URLSearchParams | null {
  const next = new URLSearchParams(input);
  let changed = false;

  const tab = next.get("tab");
  if (tab !== null && tab !== "" && !LIBRARY_TABS.has(tab)) {
    next.delete("tab");
    changed = true;
  }

  if (next.get("tab") !== "shared") {
    if (next.has("documentId")) {
      next.delete("documentId");
      changed = true;
    }
    if (next.has("documentTitle")) {
      next.delete("documentTitle");
      changed = true;
    }
    if (next.has("linkId")) {
      next.delete("linkId");
      changed = true;
    }
    for (const key of SHARE_LIST_FILTER_PARAMS) {
      if (next.has(key)) {
        next.delete(key);
        changed = true;
      }
    }
  } else {
    const within = next.get(SHARE_CREATED_WITHIN_PARAM);
    if (within !== null && within !== "" && !isLinkCreatedWithin(within)) {
      next.delete(SHARE_CREATED_WITHIN_PARAM);
      changed = true;
    }
    if (within === "all") {
      next.delete(SHARE_CREATED_WITHIN_PARAM);
      changed = true;
    }
  }

  return changed ? next : null;
}
