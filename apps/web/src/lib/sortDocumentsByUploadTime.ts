/** Newest upload first. Stable for equal/missing timestamps. */
export function sortDocumentsByNewestUpload<T extends { createdAt?: string }>(
  documents: readonly T[],
): T[] {
  return documents
    .map((doc, index) => ({ doc, index }))
    .sort((a, b) => {
      const tb = Date.parse(b.doc.createdAt ?? "") || 0;
      const ta = Date.parse(a.doc.createdAt ?? "") || 0;
      if (tb !== ta) return tb - ta;
      return a.index - b.index;
    })
    .map(({ doc }) => doc);
}
