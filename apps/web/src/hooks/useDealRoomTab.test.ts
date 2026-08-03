import { describe, expect, it } from "vitest";
import {
  DEAL_ROOM_PAGE_TABS,
  orderDealRoomPageTabs,
} from "./useDealRoomTab";

describe("orderDealRoomPageTabs", () => {
  it("keeps canonical order when documents is active", () => {
    expect(orderDealRoomPageTabs("documents")).toEqual(DEAL_ROOM_PAGE_TABS);
  });

  it("moves the active page tab to the front", () => {
    expect(orderDealRoomPageTabs("knowledge")).toEqual([
      "knowledge",
      "documents",
      "access",
      "links",
      "analytics",
    ]);
    expect(orderDealRoomPageTabs("links")).toEqual([
      "links",
      "documents",
      "access",
      "analytics",
      "knowledge",
    ]);
  });

  it("falls back to canonical order for non-page tabs", () => {
    expect(orderDealRoomPageTabs("settings")).toEqual(DEAL_ROOM_PAGE_TABS);
  });
});
