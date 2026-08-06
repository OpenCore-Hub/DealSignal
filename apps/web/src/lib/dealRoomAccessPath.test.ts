import { describe, expect, it } from "vitest";
import { dealRoomAccessPath } from "./dealRoomAccessPath";

describe("dealRoomAccessPath", () => {
  it("opens the access tab for a room", () => {
    expect(dealRoomAccessPath("acme", "room-1")).toBe(
      "/acme/deal-rooms/room-1?tab=access",
    );
  });

  it("deep-links a share link on the access tab", () => {
    expect(dealRoomAccessPath("acme", "room-1", { linkId: "link-9" })).toBe(
      "/acme/deal-rooms/room-1?tab=access&linkId=link-9",
    );
  });
});
