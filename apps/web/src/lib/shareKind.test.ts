import { describe, expect, it } from "vitest";
import { shareKindFromLink } from "./shareKind";

describe("shareKindFromLink", () => {
  it("prefers an explicit shareKind", () => {
    expect(shareKindFromLink({ shareKind: "room", hasDocumentScope: true })).toBe("room");
  });

  it("labels deal-room shares as room", () => {
    expect(shareKindFromLink({ dealRoomId: "room-1" })).toBe("room");
  });

  it("labels scoped bundles as bundle", () => {
    expect(shareKindFromLink({ hasDocumentScope: true })).toBe("bundle");
    expect(shareKindFromLink({ isBundle: true })).toBe("bundle");
  });

  it("defaults to document", () => {
    expect(shareKindFromLink({})).toBe("document");
  });
});
