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
    expect(params.get("shareDocumentId")).toBe("doc-1");
    expect(params.get("shareDocumentTitle")).toBe("Deck");
    expect(params.get("shareDocumentStatus")).toBe("processing");
  });

  it("builds library path for upload handoff", () => {
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

  it("reads and clears handoff params", () => {
    const raw = new URLSearchParams(
      "shareDocumentId=doc-1&shareDocumentTitle=Deck&shareDocumentStatus=processing&tab=all",
    );
    expect(readLibraryShareHandoff(raw)).toEqual({
      documentId: "doc-1",
      documentTitle: "Deck",
      documentStatus: "processing",
    });
    const cleared = clearLibraryShareHandoff(raw);
    expect(cleared?.toString()).toBe("tab=all");
    expect(clearLibraryShareHandoff(new URLSearchParams("tab=all"))).toBeNull();
  });
});
