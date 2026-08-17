import { isDocumentReadyForLibraryShare } from "@/lib/documentsUploadedEvent";

/** Query keys for upload-page → Documents library Share dialog handoff. */
export const LIBRARY_SHARE_HANDOFF = {
  documentId: "shareDocumentId",
  documentTitle: "shareDocumentTitle",
  documentStatus: "shareDocumentStatus",
} as const;

export type LibraryShareHandoff = {
  documentIds: string[];
  documentTitles: string[];
  documentStatus: string;
};

export type LibraryShareHandoffInput = {
  documentId?: string;
  documentIds?: string[];
  documentTitle?: string;
  documentTitles?: string[];
  documentStatus?: string;
};

function normalizeIds(input: LibraryShareHandoffInput): string[] {
  const fromList = (input.documentIds ?? []).map((id) => id.trim()).filter(Boolean);
  if (fromList.length > 0) return [...new Set(fromList)];
  const single = input.documentId?.trim();
  return single ? [single] : [];
}

function normalizeTitles(input: LibraryShareHandoffInput, ids: string[]): string[] {
  const fromList = (input.documentTitles ?? []).map((title) => title.trim());
  if (fromList.length > 0) {
    return ids.map((id, index) => fromList[index]?.trim() || id);
  }
  const single = input.documentTitle?.trim();
  if (single && ids.length === 1) return [single];
  return ids.map((id) => id);
}

/** Normalize status so callers never claim ready until ingestion finishes. */
export function normalizeLibraryShareHandoffStatus(
  status: string | undefined | null,
): string {
  const raw = (status ?? "").trim() || "processing";
  return isDocumentReadyForLibraryShare(raw) ? "ready" : raw;
}

export function buildLibraryShareHandoffParams(
  input: LibraryShareHandoffInput,
): URLSearchParams {
  const documentIds = normalizeIds(input);
  const documentTitles = normalizeTitles(input, documentIds);
  const params = new URLSearchParams();
  for (const id of documentIds) {
    params.append(LIBRARY_SHARE_HANDOFF.documentId, id);
  }
  for (const title of documentTitles) {
    params.append(LIBRARY_SHARE_HANDOFF.documentTitle, title);
  }
  params.set(
    LIBRARY_SHARE_HANDOFF.documentStatus,
    normalizeLibraryShareHandoffStatus(input.documentStatus),
  );
  return params;
}

export function documentsLibraryShareHandoffPath(
  workspaceSlug: string,
  input: LibraryShareHandoffInput,
): string {
  return `/${workspaceSlug}/documents?${buildLibraryShareHandoffParams(input).toString()}`;
}

export function readLibraryShareHandoff(
  searchParams: URLSearchParams,
): LibraryShareHandoff | null {
  const documentIds = [
    ...new Set(
      searchParams
        .getAll(LIBRARY_SHARE_HANDOFF.documentId)
        .flatMap((value) => value.split(","))
        .map((id) => id.trim())
        .filter(Boolean),
    ),
  ];
  if (documentIds.length === 0) return null;
  const rawTitles = searchParams.getAll(LIBRARY_SHARE_HANDOFF.documentTitle);
  return {
    documentIds,
    documentTitles: documentIds.map((id, index) => rawTitles[index]?.trim() || id),
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
