import { describe, expect, it } from "vitest";
import {
  documentDetailPath,
  documentEvidencePath,
  isLegacyDocumentDetailTab,
  parseDocumentDetailTab,
  parseDocumentFocusPage,
  parseDocumentIdFromMetadata,
  parsePageFromMetadata,
  patchDocumentDetailSearchParams,
} from "./documentDetailNav";

describe("documentDetailNav", () => {
  it("parses known tabs and defaults unknown to overview", () => {
    expect(parseDocumentDetailTab("content")).toBe("content");
    expect(parseDocumentDetailTab("analytics")).toBe("analytics");
    expect(parseDocumentDetailTab("insights")).toBe("insights");
    expect(parseDocumentDetailTab("ai")).toBe("insights");
    expect(parseDocumentDetailTab("nope")).toBe("overview");
    expect(parseDocumentDetailTab(null)).toBe("overview");
    expect(isLegacyDocumentDetailTab("ai")).toBe(true);
    expect(isLegacyDocumentDetailTab("insights")).toBe(false);
  });

  it("parses page numbers from metadata maps", () => {
    expect(parsePageFromMetadata({ page_number: "9" })).toBe(9);
    expect(parsePageFromMetadata({ pageNumber: "3" })).toBe(3);
    expect(parsePageFromMetadata({ page: "0" })).toBeNull();
    expect(parsePageFromMetadata(undefined)).toBeNull();
  });

  it("parses attributed document ids from metadata maps", () => {
    expect(parseDocumentIdFromMetadata({ document_id: "doc-pdf" })).toBe("doc-pdf");
    expect(parseDocumentIdFromMetadata({ documentId: "doc-xlsx" })).toBe("doc-xlsx");
    expect(parseDocumentIdFromMetadata({ document_id: "  " })).toBeNull();
    expect(parseDocumentIdFromMetadata(undefined)).toBeNull();
  });

  it("builds evidence paths from attributed metadata then primary then link", () => {
    expect(
      documentEvidencePath("acme", {
        documentId: "doc-xlsx",
        metadata: { page_number: "8", document_id: "doc-pdf" },
      }),
    ).toBe("/acme/documents/doc-pdf?tab=content&page=8");
    expect(documentEvidencePath("acme", { documentId: "doc-1" })).toBe(
      "/acme/documents/doc-1?tab=analytics",
    );
    expect(documentEvidencePath("acme", { linkId: "link-1" })).toBe("/acme/links/link-1");
  });

  it("parses focus pages and clamps to pageCount when provided", () => {
    expect(parseDocumentFocusPage("12")).toBe(12);
    expect(parseDocumentFocusPage("0")).toBeNull();
    expect(parseDocumentFocusPage("1.5")).toBeNull();
    expect(parseDocumentFocusPage("99", 40)).toBeNull();
    expect(parseDocumentFocusPage("40", 40)).toBe(40);
  });

  it("builds document detail paths with optional tab/page", () => {
    expect(documentDetailPath("acme", "doc-1")).toBe("/acme/documents/doc-1");
    expect(documentDetailPath("acme", "doc-1", { tab: "content", page: 7 })).toBe(
      "/acme/documents/doc-1?tab=content&page=7",
    );
    // page is content-scoped; ignore it on other tabs
    expect(documentDetailPath("acme", "doc-1", { tab: "analytics", page: 7 })).toBe(
      "/acme/documents/doc-1?tab=analytics",
    );
  });

  it("patches search params without dropping unrelated keys", () => {
    const current = new URLSearchParams("foo=1&tab=analytics&page=3");
    const next = patchDocumentDetailSearchParams(current, {
      tab: "content",
      page: 9,
    });
    expect(next.get("foo")).toBe("1");
    expect(next.get("tab")).toBe("content");
    expect(next.get("page")).toBe("9");

    const cleared = patchDocumentDetailSearchParams(next, {
      tab: "overview",
      page: null,
    });
    expect(cleared.get("tab")).toBeNull();
    expect(cleared.get("page")).toBeNull();
    expect(cleared.get("foo")).toBe("1");
  });

  it("drops stale page when switching away from content", () => {
    const current = new URLSearchParams("tab=content&page=4");
    const next = patchDocumentDetailSearchParams(current, { tab: "insights" });
    expect(next.get("tab")).toBe("insights");
    expect(next.get("page")).toBeNull();
  });
});
