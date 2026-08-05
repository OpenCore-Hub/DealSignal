const LIBRARY_TABS = new Set(["shared", "archived"]);

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
  opts?: { documentId?: string },
): string {
  const base = `/${workspaceSlug}/links/new`;
  if (!opts?.documentId) return base;
  const params = new URLSearchParams({ documentId: opts.documentId });
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
  }

  return changed ? next : null;
}
