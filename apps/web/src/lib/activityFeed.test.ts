import { describe, it, expect } from "vitest";
import {
  groupByDate,
  combineAdjacent,
  buildActivityGroups,
  sliceGroups,
  countDisplayItems,
  getActorInitials,
} from "./activityFeed";
import type { RecentActivityItem } from "@/lib/api";

function make(
  overrides: Partial<RecentActivityItem> & { createdAt?: string } = {}
): RecentActivityItem {
  const now = new Date();
  return {
    id: "act-1",
    eventType: "visit",
    actor: "alice@example.test",
    objectType: "document",
    objectName: "Financial Model",
    objectId: "doc-1",
    createdAt: now.toISOString(),
    ...overrides,
  };
}

describe("groupByDate", () => {
  it("groups today, yesterday and older", () => {
    const now = new Date();
    const today = make({ id: "today", createdAt: now.toISOString() });
    const yesterday = make({
      id: "yesterday",
      createdAt: new Date(now.getTime() - 24 * 60 * 60 * 1000).toISOString(),
    });
    const older = make({
      id: "older",
      createdAt: new Date(now.getTime() - 48 * 60 * 60 * 1000).toISOString(),
    });

    const groups = groupByDate([older, today, yesterday]);
    expect(groups.map((g) => g.key)).toEqual(["today", "yesterday", "older"]);
    expect(groups[0].activities).toEqual([today]);
    expect(groups[1].activities).toEqual([yesterday]);
    expect(groups[2].activities).toEqual([older]);
  });

  it("skips empty groups", () => {
    const today = make();
    expect(groupByDate([today])).toHaveLength(1);
  });
});

describe("combineAdjacent", () => {
  it("keeps different buckets separate", () => {
    const items: RecentActivityItem[] = [
      make({ id: "a", eventType: "visit" }),
      make({ id: "b", eventType: "download" }),
      make({ id: "c", eventType: "visit" }),
    ];
    const result = combineAdjacent(items);
    expect(result).toHaveLength(3);
    expect(result.every((r) => r.kind === "single")).toBe(true);
  });

  it("combines adjacent same actor/event/objectType", () => {
    const items: RecentActivityItem[] = [
      make({ id: "a", objectId: "doc-a", objectName: "Doc A" }),
      make({ id: "b", objectId: "doc-b", objectName: "Doc B" }),
      make({ id: "c", eventType: "download" }),
    ];
    const result = combineAdjacent(items);
    expect(result).toHaveLength(2);
    expect(result[0]).toEqual({
      kind: "combined",
      activities: [items[0], items[1]],
    });
    expect(result[1]).toEqual({ kind: "single", activity: items[2] });
  });

  it("deduplicates repeated visits to the same object", () => {
    const items: RecentActivityItem[] = [
      make({ id: "a", objectId: "doc-1", objectName: "Doc A" }),
      make({ id: "b", objectId: "doc-1", objectName: "Doc A" }),
      make({ id: "c", objectId: "doc-2", objectName: "Doc B" }),
    ];
    const result = combineAdjacent(items);
    expect(result).toHaveLength(1);
    expect(result[0].kind).toBe("combined");
    if (result[0].kind === "combined") {
      expect(result[0].activities).toHaveLength(2);
      expect(result[0].activities.map((a) => a.objectId)).toEqual([
        "doc-1",
        "doc-2",
      ]);
    }
  });
});

describe("buildActivityGroups", () => {
  it("groups by date then combines within each group", () => {
    const now = new Date();
    const a = make({ id: "a", objectId: "doc-a", createdAt: now.toISOString(), objectName: "A" });
    const b = make({ id: "b", objectId: "doc-b", createdAt: now.toISOString(), objectName: "B" });
    const c = make({
      id: "c",
      objectId: "doc-c",
      createdAt: new Date(now.getTime() - 48 * 60 * 60 * 1000).toISOString(),
      objectName: "C",
    });

    const groups = buildActivityGroups([a, b, c]);
    expect(groups).toHaveLength(2);
    expect(groups[0].items).toHaveLength(1);
    expect(groups[0].items[0].kind).toBe("combined");
    expect(groups[1].items).toHaveLength(1);
    expect(groups[1].items[0].kind).toBe("single");
  });
});

describe("sliceGroups", () => {
  it("respects limit across groups", () => {
    const items: RecentActivityItem[] = [
      make({ id: "a", eventType: "visit" }),
      make({ id: "b", eventType: "download" }),
      make({
        id: "c",
        createdAt: new Date(Date.now() - 48 * 60 * 60 * 1000).toISOString(),
        eventType: "upload",
      }),
    ];
    const groups = buildActivityGroups(items);
    const sliced = sliceGroups(groups, 2);
    expect(countDisplayItems(sliced)).toBe(2);
    expect(sliced).toHaveLength(1);
  });
});

describe("getActorInitials", () => {
  it("returns uppercase first character", () => {
    expect(getActorInitials("alice")).toBe("A");
    expect(getActorInitials("  bob")).toBe("B");
    expect(getActorInitials("")).toBe("?");
  });
});
