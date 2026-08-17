import { describe, expect, it } from "vitest";
import {
  DEAL_ROOM_PAGE_TABS,
  orderDealRoomPageTabs,
  visibleDealRoomPageTabs,
} from "./useDealRoomTab";

describe("orderDealRoomPageTabs", () => {
  it("keeps canonical order when documents is active", () => {
    expect(orderDealRoomPageTabs("documents")).toEqual(DEAL_ROOM_PAGE_TABS);
  });

  it("moves the active page tab to the front", () => {
    expect(orderDealRoomPageTabs("knowledge")).toEqual([
      "knowledge",
      "documents",
      "members",
      "access",
      "links",
      "analytics",
    ]);
    expect(orderDealRoomPageTabs("links")).toEqual([
      "links",
      "documents",
      "members",
      "access",
      "analytics",
      "knowledge",
    ]);
  });

  it("falls back to canonical order for non-page tabs", () => {
    expect(orderDealRoomPageTabs("settings")).toEqual(DEAL_ROOM_PAGE_TABS);
  });

  it("hides access for non-managers and keeps knowledge for room viewers", () => {
    const viewOnly = visibleDealRoomPageTabs({ canManage: false, canViewKnowledge: true });
    expect(viewOnly).toEqual(["documents", "members", "links", "analytics", "knowledge"]);
    expect(orderDealRoomPageTabs("knowledge", viewOnly)).toEqual([
      "knowledge",
      "documents",
      "members",
      "links",
      "analytics",
    ]);
    expect(orderDealRoomPageTabs("links", viewOnly)).toEqual([
      "links",
      "documents",
      "members",
      "analytics",
      "knowledge",
    ]);
  });

  it("shows access read-only for oversight", () => {
    expect(
      visibleDealRoomPageTabs({
        canManage: false,
        canViewKnowledge: true,
        canViewAccess: true,
      }),
    ).toEqual(["documents", "members", "access", "links", "analytics", "knowledge"]);
  });

  it("always includes members for anyone who can open the room", () => {
    expect(
      visibleDealRoomPageTabs({ canManage: false, canViewKnowledge: false }),
    ).toContain("members");
  });

  it("hides knowledge when the caller cannot view the desk", () => {
    expect(visibleDealRoomPageTabs({ canManage: false, canViewKnowledge: false })).toEqual([
      "documents",
      "members",
      "links",
      "analytics",
    ]);
  });
});
