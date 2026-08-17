export type ShareKind = "room" | "bundle" | "document";

export function shareKindFromLink(link: {
  dealRoomId?: string | null;
  hasDocumentScope?: boolean;
  isBundle?: boolean;
  shareKind?: string | null;
}): ShareKind {
  if (link.shareKind === "room" || link.shareKind === "bundle" || link.shareKind === "document") {
    return link.shareKind;
  }
  if (link.dealRoomId) return "room";
  if (link.hasDocumentScope || link.isBundle) return "bundle";
  return "document";
}
