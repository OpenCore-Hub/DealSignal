import { describe, expect, it } from "vitest";
import { viewerPath } from "./citations";

describe("viewerPath", () => {
  it("builds a bare document path", () => {
    expect(viewerPath("doc_1")).toBe("/viewer/doc_1");
  });

  it("includes page, roomId, and ws for owner knowledge continuity", () => {
    expect(
      viewerPath("doc_1", 3, { roomId: "room_1", workspaceSlug: "acme-capital" }),
    ).toBe("/viewer/doc_1?page=3&roomId=room_1&ws=acme-capital");
  });

  it("omits invalid page but keeps roomId and ws", () => {
    expect(
      viewerPath("doc_1", 0, { roomId: "room_1", workspaceSlug: "acme-capital" }),
    ).toBe("/viewer/doc_1?roomId=room_1&ws=acme-capital");
  });
});
