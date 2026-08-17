import { describe, expect, it } from "vitest";
import { sortDocumentsByNewestUpload } from "./sortDocumentsByUploadTime";

describe("sortDocumentsByNewestUpload", () => {
  it("orders by createdAt descending", () => {
    const sorted = sortDocumentsByNewestUpload([
      { id: "old", createdAt: "2026-01-01T00:00:00Z" },
      { id: "new", createdAt: "2026-08-16T00:00:00Z" },
      { id: "mid", createdAt: "2026-06-01T00:00:00Z" },
    ]);
    expect(sorted.map((d) => d.id)).toEqual(["new", "mid", "old"]);
  });

  it("keeps original order when timestamps match", () => {
    const sorted = sortDocumentsByNewestUpload([
      { id: "a", createdAt: "2026-08-16T00:00:00Z" },
      { id: "b", createdAt: "2026-08-16T00:00:00Z" },
    ]);
    expect(sorted.map((d) => d.id)).toEqual(["a", "b"]);
  });
});
