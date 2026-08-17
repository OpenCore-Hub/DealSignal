import type { AccessLog, DocumentSummary, Link } from "@/types";

export type ShareDocumentLink = Pick<
  Link,
  "documentId" | "documentTitle" | "documents" | "isBundle" | "shortUrl" | "name"
>;

export function shareDocumentLabel(
  link: ShareDocumentLink,
  opts?: { focusDocumentId?: string },
): { title: string; extraCount: number } {
  const docs = link.documents ?? [];
  const focus = opts?.focusDocumentId?.trim();
  const focused = focus ? docs.find((d) => d.id === focus) : undefined;
  const primary = docs.find((d) => d.id && d.id === link.documentId) ?? docs[0];
  const live = docs.find((d) => d.status !== "archived") ?? primary;
  const fallback = primary?.status === "archived" ? live : primary;
  const title = (focused?.title || fallback?.title || link.documentTitle || "").trim();
  const extraCount = docs.length > 1 ? docs.length - 1 : 0;
  return { title, extraCount };
}

export function formatShareDocumentLabel(
  link: ShareDocumentLink,
  formatBundle: (title: string, extraCount: number) => string,
  focusDocumentId?: string,
): string {
  const { title, extraCount } = shareDocumentLabel(link, { focusDocumentId });
  if (!title) return "";
  return extraCount > 0 ? formatBundle(title, extraCount) : title;
}

export function shareDocumentSearchText(link: ShareDocumentLink): string {
  const titles = (link.documents ?? []).map((d) => d.title).filter(Boolean);
  return [link.shortUrl, link.documentTitle, link.name, ...titles].filter(Boolean).join(" ");
}

/** Document IDs a share contains: primary, documentIds, and documents[]. */
export function libraryShareDocumentIDs(
  link: Pick<Link, "documentId" | "documentIds" | "documents">,
): string[] {
  const ids = new Set<string>();
  const add = (value?: string) => {
    const id = value?.trim();
    if (id) ids.add(id);
  };
  add(link.documentId);
  for (const id of link.documentIds ?? []) add(id);
  for (const doc of link.documents ?? []) add(doc.id);
  return [...ids];
}

/** True when the link contains the document (primary or bundle member). */
export function linkIncludesDocument(
  link: Pick<Link, "documentId" | "documentIds" | "documents">,
  documentId: string,
): boolean {
  const id = documentId.trim();
  return Boolean(id) && libraryShareDocumentIDs(link).includes(id);
}

/** Library share list: same membership as ListLinksByDocument (skip deal-room). */
export function linkIncludesLibraryDocument(
  link: Pick<Link, "dealRoomId" | "documentId" | "documentIds" | "documents">,
  documentId: string,
): boolean {
  if (link.dealRoomId) return false;
  return linkIncludesDocument(link, documentId);
}

export function accessLogDocumentTitle(
  log: Pick<AccessLog, "documentId">,
  documents: Pick<DocumentSummary, "id" | "title">[],
  primaryDocumentId?: string,
): string | undefined {
  const id = log.documentId?.trim() || primaryDocumentId?.trim();
  if (!id) return undefined;
  const title = documents.find((d) => d.id === id)?.title?.trim();
  return title || undefined;
}
