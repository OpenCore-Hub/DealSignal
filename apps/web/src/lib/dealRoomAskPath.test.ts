import { describe, expect, it } from "vitest";
import { dealRoomAskPath, parseDealRoomAskTarget } from "./dealRoomAskPath";

describe("dealRoomAskPath", () => {
  it("opens the QA tab for a room", () => {
    expect(dealRoomAskPath("acme", "room-1")).toBe(
      "/acme/deal-rooms/room-1?tab=qa",
    );
  });

  it("deep-links a share link filter on the QA tab", () => {
    expect(dealRoomAskPath("acme", "room-1", { linkId: "link-9" })).toBe(
      "/acme/deal-rooms/room-1?tab=qa&linkId=link-9",
    );
  });

  it("deep-links the formal queue inbox tab", () => {
    expect(dealRoomAskPath("acme", "room-1", { formalQueue: true })).toBe(
      "/acme/deal-rooms/room-1?tab=qa&askInbox=formal_queue",
    );
  });
});

describe("parseDealRoomAskTarget", () => {
  it("parses room-only targets", () => {
    expect(parseDealRoomAskTarget("room-1")).toEqual({ roomId: "room-1" });
  });

  it("parses room/link composite targets", () => {
    expect(parseDealRoomAskTarget("room-1/link-9")).toEqual({
      roomId: "room-1",
      linkId: "link-9",
    });
  });
});
