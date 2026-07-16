import type { RecentActivityItem } from "@/lib/api";

export type DateGroupKey = "today" | "yesterday" | "older";

export interface DateGroup<T> {
  key: DateGroupKey;
  activities: T[];
}

export type DisplayActivity =
  | { kind: "single"; activity: RecentActivityItem }
  | { kind: "combined"; activities: RecentActivityItem[] };

function startOfDay(ts: number): number {
  const d = new Date(ts);
  d.setHours(0, 0, 0, 0);
  return d.getTime();
}

export function groupByDate<T extends { createdAt: string }>(
  items: T[]
): DateGroup<T>[] {
  const now = Date.now();
  const todayStart = startOfDay(now);
  const yesterdayStart = todayStart - 24 * 60 * 60 * 1000;

  const groups: DateGroup<T>[] = [
    { key: "today", activities: [] },
    { key: "yesterday", activities: [] },
    { key: "older", activities: [] },
  ];

  for (const item of items) {
    const itemStart = startOfDay(new Date(item.createdAt).getTime());
    if (itemStart >= todayStart) {
      groups[0].activities.push(item);
    } else if (itemStart >= yesterdayStart) {
      groups[1].activities.push(item);
    } else {
      groups[2].activities.push(item);
    }
  }

  return groups.filter((g) => g.activities.length > 0);
}

function sameActivityBucket(
  a: RecentActivityItem,
  b: RecentActivityItem
): boolean {
  return (
    a.actor === b.actor &&
    a.eventType === b.eventType &&
    a.objectType === b.objectType
  );
}

export function combineAdjacent(
  activities: RecentActivityItem[]
): DisplayActivity[] {
  const result: DisplayActivity[] = [];
  let bucket: RecentActivityItem[] = [];
  let seenObjectIds = new Set<string>();

  const flushBucket = () => {
    if (bucket.length === 0) return;
    result.push(
      bucket.length === 1
        ? { kind: "single", activity: bucket[0] }
        : { kind: "combined", activities: bucket }
    );
    bucket = [];
    seenObjectIds = new Set();
  };

  for (const activity of activities) {
    const fitsBucket =
      bucket.length === 0 || sameActivityBucket(bucket[bucket.length - 1], activity);

    if (!fitsBucket) {
      flushBucket();
    }

    if (!seenObjectIds.has(activity.objectId)) {
      bucket.push(activity);
      seenObjectIds.add(activity.objectId);
    }
  }

  flushBucket();
  return result;
}

export interface ActivityGroup {
  key: DateGroupKey;
  items: DisplayActivity[];
}

export function buildActivityGroups(
  activities: RecentActivityItem[]
): ActivityGroup[] {
  return groupByDate(activities).map((g) => ({
    key: g.key,
    items: combineAdjacent(g.activities),
  }));
}

export function sliceGroups(
  groups: ActivityGroup[],
  limit: number
): ActivityGroup[] {
  let remaining = limit;
  const result: ActivityGroup[] = [];

  for (const group of groups) {
    if (remaining <= 0) break;
    const take = Math.min(group.items.length, remaining);
    result.push({ key: group.key, items: group.items.slice(0, take) });
    remaining -= take;
  }

  return result;
}

export function countDisplayItems(groups: ActivityGroup[]): number {
  return groups.reduce((sum, g) => sum + g.items.length, 0);
}

export function getActorInitials(actor: string): string {
  const first = actor.trim().charAt(0).toUpperCase();
  return first || "?";
}
