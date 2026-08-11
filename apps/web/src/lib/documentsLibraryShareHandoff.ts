import { isDocumentReadyForLibraryShare } from "@/lib/documentsUploadedEvent";

/** Query keys for upload-page → Documents library Share dialog handoff. */
export const LIBRARY_SHARE_HANDOFF = {
  documentId: "shareDocumentId",
  documentTitle: "shareDocumentTitle",
  documentStatus: "shareDocumentStatus",
} as const;

export type LibraryShareHandoff = {
  documentId: string;
  documentTitle: string;
  documentStatus: string;
};

/** Normalize status so callers never claim ready until ingestion finishes. */
export function normalizeLibraryShareHandoffStatus(
  status: string | undefined | null,
): string {
  const raw = (status ?? "").trim() || "processing";
  return isDocumentReadyForLibraryShare(raw) ? "ready" : raw;
}

export function buildLibraryShareHandoffParams(input: {
  documentId: string;
  documentTitle?: string;
  documentStatus?: string;
}): URLSearchParams {
  return new URLSearchParams({
    [LIBRARY_SHARE_HANDOFF.documentId]: input.documentId,
    [LIBRARY_SHARE_HANDOFF.documentTitle]:
      input.documentTitle?.trim() || input.documentId,
    [LIBRARY_SHARE_HANDOFF.documentStatus]: normalizeLibraryShareHandoffStatus(
      input.documentStatus,
    ),
  });
}

export function documentsLibraryShareHandoffPath(
  workspaceSlug: string,
  input: {
    documentId: string;
    documentTitle?: string;
    documentStatus?: string;
  },
): string {
  return `/${workspaceSlug}/documents?${buildLibraryShareHandoffParams(input).toString()}`;
}

export function readLibraryShareHandoff(
  searchParams: URLSearchParams,
): LibraryShareHandoff | null {
  const documentId = searchParams.get(LIBRARY_SHARE_HANDOFF.documentId)?.trim();
  if (!documentId) return null;
  return {
    documentId,
    documentTitle:
      searchParams.get(LIBRARY_SHARE_HANDOFF.documentTitle)?.trim() || documentId,
    documentStatus: normalizeLibraryShareHandoffStatus(
      searchParams.get(LIBRARY_SHARE_HANDOFF.documentStatus),
    ),
  };
}

/** Returns a cleared clone, or null when nothing to clear. */
export function clearLibraryShareHandoff(
  searchParams: URLSearchParams,
): URLSearchParams | null {
  if (
    !searchParams.has(LIBRARY_SHARE_HANDOFF.documentId) &&
    !searchParams.has(LIBRARY_SHARE_HANDOFF.documentTitle) &&
    !searchParams.has(LIBRARY_SHARE_HANDOFF.documentStatus)
  ) {
    return null;
  }
  const next = new URLSearchParams(searchParams);
  next.delete(LIBRARY_SHARE_HANDOFF.documentId);
  next.delete(LIBRARY_SHARE_HANDOFF.documentTitle);
  next.delete(LIBRARY_SHARE_HANDOFF.documentStatus);
  return next;
}
