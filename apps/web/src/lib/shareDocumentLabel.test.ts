import { describe, expect, it } from "vitest";
import {
  accessLogDocumentTitle,
  formatShareDocumentLabel,
  libraryShareDocumentIDs,
  linkIncludesDocument,
  linkIncludesLibraryDocument,
  shareDocumentLabel,
  shareDocumentSearchText,
} from "./shareDocumentLabel";
import type { Link } from "@/types";

function link(partial: Partial<Link> = {}): Link {
  return {
    id: "link_1",
    documentId: "doc_xlsx",
    documentIds: ["doc_xlsx", "doc_pdf"],
    folderPaths: [],
    documentTitle: "Model.xlsx",
    shortUrl: "https://example.com/l/x",
    accessCount: 0,
    heatLevel: "cold",
    createdAt: "2026-08-01T00:00:00Z",
    isBundle: true,
    documents: [
      { id: "doc_xlsx", title: "Model.xlsx", sourceType: "xlsx", pageCount: 3, status: "ready" },
      { id: "doc_pdf", title: "Memo.pdf", sourceType: "pdf", pageCount: 16, status: "ready" },
    ],
    ...partial,
  };
}

describe("shareDocumentLabel", () => {
  it("uses the primary title and extra count for a bundle", () => {
    expect(shareDocumentLabel(link())).toEqual({ title: "Model.xlsx", extraCount: 1 });
  });

  it("prefers the focused member when viewing a document's shares", () => {
    expect(shareDocumentLabel(link(), { focusDocumentId: "doc_pdf" })).toEqual({
      title: "Memo.pdf",
      extraCount: 1,
    });
  });

  it("uses the first live member when the stored primary is archived", () => {
    expect(
      shareDocumentLabel(
        link({
          documents: [
            { id: "doc_xlsx", title: "Model.xlsx", sourceType: "xlsx", pageCount: 3, status: "archived" },
            { id: "doc_pdf", title: "Memo.pdf", sourceType: "pdf", pageCount: 16, status: "ready" },
          ],
        }),
      ),
    ).toEqual({ title: "Memo.pdf", extraCount: 1 });
  });

  it("keeps a single-document share as the plain title", () => {
    expect(
      shareDocumentLabel(
        link({
          isBundle: false,
          documentIds: ["doc_xlsx"],
          documents: [
            { id: "doc_xlsx", title: "Model.xlsx", sourceType: "xlsx", pageCount: 3, status: "ready" },
          ],
        }),
      ),
    ).toEqual({ title: "Model.xlsx", extraCount: 0 });
  });

  it("formats the bundle label through the caller i18n seam", () => {
    expect(formatShareDocumentLabel(link(), (title, count) => `${title} +${count}`)).toBe(
      "Model.xlsx +1",
    );
    expect(
      formatShareDocumentLabel(link(), (title, count) => `${title} +${count}`, "doc_pdf"),
    ).toBe("Memo.pdf +1");
  });
});

describe("shareDocumentSearchText", () => {
  it("includes secondary document titles", () => {
    expect(shareDocumentSearchText(link()).toLowerCase()).toContain("memo.pdf");
  });
});

describe("libraryShareDocumentIDs", () => {
  it("unions primary, documentIds, and documents without duplicates", () => {
    expect(libraryShareDocumentIDs(link()).sort()).toEqual(["doc_pdf", "doc_xlsx"]);
  });
});

describe("linkIncludesDocument", () => {
  it("matches primary and bundle members, including deal-room shares", () => {
    expect(linkIncludesDocument(link(), "doc_xlsx")).toBe(true);
    expect(linkIncludesDocument(link(), "doc_pdf")).toBe(true);
    expect(linkIncludesDocument(link(), "doc_other")).toBe(false);
    expect(linkIncludesDocument(link({ dealRoomId: "room_1" }), "doc_pdf")).toBe(true);
  });
});

describe("linkIncludesLibraryDocument", () => {
  it("skips deal-room shares and matches bundle members", () => {
    expect(linkIncludesLibraryDocument(link(), "doc_pdf")).toBe(true);
    expect(linkIncludesLibraryDocument(link({ dealRoomId: "room_1" }), "doc_pdf")).toBe(false);
    expect(linkIncludesLibraryDocument(link(), "  ")).toBe(false);
  });
});

describe("accessLogDocumentTitle", () => {
  const docs = [
    { id: "doc_xlsx", title: "Model.xlsx" },
    { id: "doc_pdf", title: "Memo.pdf" },
  ];

  it("uses the log documentId when present", () => {
    expect(accessLogDocumentTitle({ documentId: "doc_pdf" }, docs, "doc_xlsx")).toBe("Memo.pdf");
  });

  it("falls back to the primary document for legacy logs", () => {
    expect(accessLogDocumentTitle({}, docs, "doc_xlsx")).toBe("Model.xlsx");
  });
});
