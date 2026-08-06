import { describe, expect, it } from "vitest";
import { actionNavigatePath } from "./actionNavigation";

describe("actionNavigatePath", () => {
  it("routes document share access requests to Document Library Share only", () => {
    expect(
      actionNavigatePath("acme", {
        sourceType: "link_access_request",
        sourceId: "link-doc",
      }),
    ).toBe("/acme/documents?tab=shared&linkId=link-doc");
  });

  it("routes deal-room share access requests to Deal Room Access only", () => {
    expect(
      actionNavigatePath("acme", {
        sourceType: "deal_room_link_access_request",
        sourceId: "link-room",
        targetId: "room-1",
      }),
    ).toBe("/acme/deal-rooms/room-1?tab=access&linkId=link-room");
  });

  it("refuses deal-room share navigation without targetId (no document fallback)", () => {
    expect(
      actionNavigatePath("acme", {
        sourceType: "deal_room_link_access_request",
        sourceId: "link-room",
      }),
    ).toBeNull();
  });

  it("routes room membership / NDA todos by room id to Access", () => {
    expect(
      actionNavigatePath("acme", {
        sourceType: "room_access_request",
        sourceId: "room-1",
      }),
    ).toBe("/acme/deal-rooms/room-1?tab=access");

    expect(
      actionNavigatePath("acme", {
        sourceType: "room_nda",
        sourceId: "room-1",
      }),
    ).toBe("/acme/deal-rooms/room-1?tab=access");
  });

  it("does not send document share todos into deal rooms", () => {
    const path = actionNavigatePath("acme", {
      sourceType: "link_access_request",
      sourceId: "link-doc",
      targetId: "room-should-be-ignored",
    });
    expect(path).toContain("/documents?");
    expect(path).not.toContain("/deal-rooms/");
  });
});
