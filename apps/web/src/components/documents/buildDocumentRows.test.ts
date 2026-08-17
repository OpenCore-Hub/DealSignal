import { describe, expect, it } from "vitest";
import { buildDocumentRows } from "./DocumentsColumns";
import type { Document, Link } from "@/types";

function document(partial: Partial<Document> & Pick<Document, "id" | "title">): Document {
  return {
    sourceType: "pdf",
    fileName: `${partial.title}.pdf`,
    fileType: "pdf",
    fileSize: 1000,
    pageCount: 10,
    status: "ready",
    createdAt: "2026-08-01T00:00:00Z",
    updatedAt: "2026-08-01T00:00:00Z",
    ...partial,
  };
}

function link(partial: Partial<Link> & Pick<Link, "id">): Link {
  return {
    documentIds: [],
    folderPaths: [],
    documentTitle: "Share",
    shortUrl: "https://example.com/l/x",
    accessCount: 0,
    heatLevel: "cold",
    createdAt: "2026-08-01T00:00:00Z",
    isBundle: false,
    documents: [],
    ...partial,
  };
}

describe("buildDocumentRows", () => {
  it("attributes a bundle share to every member document once", () => {
    const primary = document({ id: "doc_xlsx", title: "Model.xlsx" });
    const secondary = document({ id: "doc_pdf", title: "Memo.pdf" });
    const other = document({ id: "doc_other", title: "Other.pdf" });
    const bundle = link({
      id: "link_bundle",
      documentId: "doc_xlsx",
      documentIds: ["doc_xlsx", "doc_pdf"],
      isBundle: true,
      accessCount: 7,
      documents: [
        { id: "doc_xlsx", title: "Model.xlsx", sourceType: "xlsx", pageCount: 3, status: "ready" },
        { id: "doc_pdf", title: "Memo.pdf", sourceType: "pdf", pageCount: 16, status: "ready" },
      ],
    });

    const rows = buildDocumentRows([primary, secondary, other], [bundle]);
    expect(rows.find((r) => r.id === "doc_xlsx")?.links.map((l) => l.id)).toEqual(["link_bundle"]);
    expect(rows.find((r) => r.id === "doc_pdf")?.links.map((l) => l.id)).toEqual(["link_bundle"]);
    expect(rows.find((r) => r.id === "doc_other")?.links).toEqual([]);
    expect(rows.find((r) => r.id === "doc_xlsx")?.totalViews).toBe(7);
    expect(rows.find((r) => r.id === "doc_pdf")?.totalViews).toBe(7);
  });

  it("does not attach deal-room shares to the library", () => {
    const doc = document({ id: "doc_1", title: "Deck.pdf" });
    const roomShare = link({
      id: "link_room",
      documentId: "doc_1",
      documentIds: ["doc_1"],
      dealRoomId: "room_1",
      accessCount: 99,
    });

    const [row] = buildDocumentRows([doc], [roomShare]);
    expect(row.links).toEqual([]);
    expect(row.totalViews).toBe(0);
  });

  it("keeps a single-document share on the primary only", () => {
    const primary = document({ id: "doc_1", title: "Deck.pdf" });
    const other = document({ id: "doc_2", title: "Other.pdf" });
    const share = link({
      id: "link_1",
      documentId: "doc_1",
      documentIds: ["doc_1"],
      accessCount: 4,
    });

    const rows = buildDocumentRows([primary, other], [share]);
    expect(rows.find((r) => r.id === "doc_1")?.totalViews).toBe(4);
    expect(rows.find((r) => r.id === "doc_2")?.links).toEqual([]);
  });
});
