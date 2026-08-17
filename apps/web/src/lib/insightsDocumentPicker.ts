import type { DealRoom, DealRoomFolderDocs, Document } from "@/types";

export type InsightDocScope = "library" | "deal_room";
export type InsightRoomPickerMode = "hidden" | "rail" | "browse";

export const INSIGHT_ROOM_RAIL_MAX = 5;

export function mergeInsightDocuments(general: Document[], dealRoom: Document[]): Document[] {
  const byId = new Map<string, Document>();
  for (const d of [...general, ...dealRoom]) {
    byId.set(d.id, d);
  }
  return Array.from(byId.values()).sort((a, b) =>
    a.title.localeCompare(b.title, undefined, { sensitivity: "base" }),
  );
}

export function insightDocScope(doc: Pick<Document, "category">): InsightDocScope {
  return doc.category === "deal_room" ? "deal_room" : "library";
}

export function documentTitle(doc: Pick<Document, "title">, untitled: string): string {
  return doc.title.trim() || untitled;
}

export function collectDealRoomDocumentIds(folders: DealRoomFolderDocs[]): Set<string> {
  const ids = new Set<string>();
  for (const folder of folders) {
    for (const item of folder.documents ?? []) {
      if (item.document_id) ids.add(item.document_id);
    }
  }
  return ids;
}

export function activeInsightRooms(rooms: DealRoom[]): DealRoom[] {
  return rooms.filter((room) => room.status !== "archived");
}

export function insightRoomPickerMode(roomCount: number): InsightRoomPickerMode {
  if (roomCount <= 1) return "hidden";
  if (roomCount <= INSIGHT_ROOM_RAIL_MAX) return "rail";
  return "browse";
}

export function filterInsightRooms(rooms: DealRoom[], query: string): DealRoom[] {
  const q = query.trim().toLowerCase();
  if (!q) return rooms;
  return rooms.filter((room) => room.name.toLowerCase().includes(q));
}

export function filterInsightDocuments(
  documents: Document[],
  scope: InsightDocScope,
  query: string,
  roomDocIds?: ReadonlySet<string> | null,
): Document[] {
  const q = query.trim().toLowerCase();
  return documents.filter((doc) => {
    if (insightDocScope(doc) !== scope) return false;
    if (scope === "deal_room" && roomDocIds && !roomDocIds.has(doc.id)) return false;
    if (!q) return true;
    return documentTitle(doc, "").toLowerCase().includes(q);
  });
}
