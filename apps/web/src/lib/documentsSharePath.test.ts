import { describe, expect, it } from "vitest";
import {
  documentsCreateLinkPath,
  documentsSharePath,
  sanitizeDocumentsLibrarySearchParams,
} from "./documentsSharePath";

describe("documentsSharePath", () => {
  it("builds the shared tab path", () => {
    expect(documentsSharePath("acme")).toBe("/acme/documents?tab=shared");
  });

  it("includes document filter params", () => {
    expect(
      documentsSharePath("acme", {
        documentId: "doc-1",
        documentTitle: "Pitch Deck",
      }),
    ).toBe("/acme/documents?tab=shared&documentId=doc-1&documentTitle=Pitch+Deck");
  });

  it("includes linkId for access-request deep links", () => {
    expect(documentsSharePath("acme", { linkId: "link-9" })).toBe(
      "/acme/documents?tab=shared&linkId=link-9",
    );
  });
});

describe("documentsCreateLinkPath", () => {
  it("builds the create-link path", () => {
    expect(documentsCreateLinkPath("acme")).toBe("/acme/links/new");
  });

  it("encodes documentId", () => {
    expect(documentsCreateLinkPath("acme", { documentId: "doc 1" })).toBe(
      "/acme/links/new?documentId=doc+1",
    );
  });
});

describe("sanitizeDocumentsLibrarySearchParams", () => {
  it("returns null when params are already valid", () => {
    expect(sanitizeDocumentsLibrarySearchParams(new URLSearchParams("tab=shared"))).toBeNull();
    expect(sanitizeDocumentsLibrarySearchParams(new URLSearchParams("tab=archived"))).toBeNull();
    expect(sanitizeDocumentsLibrarySearchParams(new URLSearchParams())).toBeNull();
  });

  it("drops unknown tab values", () => {
    const next = sanitizeDocumentsLibrarySearchParams(new URLSearchParams("tab=recent"));
    expect(next?.toString()).toBe("");
  });

  it("keeps shared document filters and strips them off other tabs", () => {
    expect(
      sanitizeDocumentsLibrarySearchParams(
        new URLSearchParams("tab=shared&documentId=doc-1&documentTitle=Deck&linkId=link-1"),
      ),
    ).toBeNull();

    const stripped = sanitizeDocumentsLibrarySearchParams(
      new URLSearchParams("tab=archived&documentId=doc-1&documentTitle=Deck&linkId=link-1"),
    );
    expect(stripped?.toString()).toBe("tab=archived");
  });

  it("preserves unrelated params when dropping invalid tab", () => {
    const next = sanitizeDocumentsLibrarySearchParams(
      new URLSearchParams("tab=bogus&q=deck"),
    );
    expect(next?.toString()).toBe("q=deck");
  });
});
