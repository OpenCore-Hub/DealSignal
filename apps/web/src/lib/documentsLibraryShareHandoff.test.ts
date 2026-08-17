import { describe, expect, it } from "vitest";
import {
  buildLibraryShareHandoffParams,
  clearLibraryShareHandoff,
  documentsLibraryShareHandoffPath,
  normalizeLibraryShareHandoffStatus,
  readLibraryShareHandoff,
} from "./documentsLibraryShareHandoff";

describe("documentsLibraryShareHandoff", () => {
  it("never elevates non-ready status to ready", () => {
    expect(normalizeLibraryShareHandoffStatus("processing")).toBe("processing");
    expect(normalizeLibraryShareHandoffStatus("uploading")).toBe("uploading");
    expect(normalizeLibraryShareHandoffStatus("")).toBe("processing");
    expect(normalizeLibraryShareHandoffStatus("ready")).toBe("ready");
    expect(normalizeLibraryShareHandoffStatus(" READY ")).toBe("ready");
  });

  it("builds query params with normalized status", () => {
    const params = buildLibraryShareHandoffParams({
      documentId: "doc-1",
      documentTitle: "Deck",
      documentStatus: "processing",
    });
    expect(params.getAll("shareDocumentId")).toEqual(["doc-1"]);
    expect(params.getAll("shareDocumentTitle")).toEqual(["Deck"]);
    expect(params.get("shareDocumentStatus")).toBe("processing");
  });

  it("builds library path for a single-file upload handoff", () => {
    expect(
      documentsLibraryShareHandoffPath("acme", {
        documentId: "doc-1",
        documentTitle: "Deck",
        documentStatus: "ready",
      }),
    ).toBe(
      "/acme/documents?shareDocumentId=doc-1&shareDocumentTitle=Deck&shareDocumentStatus=ready",
    );
  });

  it("builds library path for a multi-file upload handoff", () => {
    expect(
      documentsLibraryShareHandoffPath("acme", {
        documentIds: ["doc-1", "doc-2"],
        documentTitles: ["Deck", "Model.xlsx"],
        documentStatus: "processing",
      }),
    ).toBe(
      "/acme/documents?shareDocumentId=doc-1&shareDocumentId=doc-2&shareDocumentTitle=Deck&shareDocumentTitle=Model.xlsx&shareDocumentStatus=processing",
    );
  });

  it("reads and clears single-id handoff params", () => {
    const raw = new URLSearchParams(
      "shareDocumentId=doc-1&shareDocumentTitle=Deck&shareDocumentStatus=processing&tab=all",
    );
    expect(readLibraryShareHandoff(raw)).toEqual({
      documentIds: ["doc-1"],
      documentTitles: ["Deck"],
      documentStatus: "processing",
    });
    const cleared = clearLibraryShareHandoff(raw);
    expect(cleared?.toString()).toBe("tab=all");
    expect(clearLibraryShareHandoff(new URLSearchParams("tab=all"))).toBeNull();
  });

  it("reads repeated and comma-separated upload ids", () => {
    const repeated = new URLSearchParams();
    repeated.append("shareDocumentId", "doc-1");
    repeated.append("shareDocumentId", "doc-2");
    repeated.append("shareDocumentTitle", "A");
    repeated.append("shareDocumentTitle", "B");
    expect(readLibraryShareHandoff(repeated)).toEqual({
      documentIds: ["doc-1", "doc-2"],
      documentTitles: ["A", "B"],
      documentStatus: "processing",
    });

    expect(
      readLibraryShareHandoff(
        new URLSearchParams("shareDocumentId=doc-1,doc-2&shareDocumentStatus=ready"),
      ),
    ).toEqual({
      documentIds: ["doc-1", "doc-2"],
      documentTitles: ["doc-1", "doc-2"],
      documentStatus: "ready",
    });
  });
});
