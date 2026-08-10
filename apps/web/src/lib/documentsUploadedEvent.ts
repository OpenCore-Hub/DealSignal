/** Cross-surface signal after a library (or agreement) document upload finishes. */
export const DOCUMENTS_UPLOADED_EVENT = "documents:uploaded";

export type DocumentsUploadedDetail = {
  documentId: string;
  documentTitle: string;
  status: string;
  category?: string;
};

export function dispatchDocumentsUploaded(detail?: DocumentsUploadedDetail): void {
  window.dispatchEvent(
    new CustomEvent<DocumentsUploadedDetail | undefined>(DOCUMENTS_UPLOADED_EVENT, {
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
