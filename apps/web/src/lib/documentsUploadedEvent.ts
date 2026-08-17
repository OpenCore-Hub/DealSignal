/** Cross-surface signal after a library (or agreement) document upload finishes. */
export const DOCUMENTS_UPLOADED_EVENT = "documents:uploaded";

export type DocumentsUploadedDetail = {
  documentId: string;
  documentTitle: string;
  status: string;
  category?: string;
  createdAt?: string;
};

export type DocumentsUploadedEventDetail =
  | DocumentsUploadedDetail
  | DocumentsUploadedDetail[]
  | { documents: DocumentsUploadedDetail[] }
  | undefined;

export function parseDocumentsUploadedDetail(
  detail: DocumentsUploadedEventDetail,
): DocumentsUploadedDetail[] {
  if (!detail) return [];
  if (Array.isArray(detail)) return detail.filter((item) => item?.documentId);
  if ("documents" in detail && Array.isArray(detail.documents)) {
    return detail.documents.filter((item) => item?.documentId);
  }
  if ("documentId" in detail && detail.documentId) return [detail];
  return [];
}

export function dispatchDocumentsUploaded(
  detail?: DocumentsUploadedDetail | DocumentsUploadedDetail[],
): void {
  window.dispatchEvent(
    new CustomEvent<DocumentsUploadedEventDetail>(DOCUMENTS_UPLOADED_EVENT, {
      detail,
    }),
  );
}

/** General library uploads may offer post-upload Share; agreements / deal_room do not. */
export function isLibraryShareableUpload(
  detail: DocumentsUploadedDetail | null | undefined,
): boolean {
  if (!detail?.documentId) return false;
  const category = (detail.category ?? "general").trim().toLowerCase();
  return category === "general" || category === "";
}

/** Share dialog must not open until ingestion finishes (upload HTTP ≠ ready). */
export function isDocumentReadyForLibraryShare(
  status: string | undefined | null,
): boolean {
  return (status ?? "").trim().toLowerCase() === "ready";
}
