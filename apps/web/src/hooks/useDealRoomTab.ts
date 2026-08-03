import { useEffect, useMemo } from "react";
import { useSearchParams } from "react-router";

export type DealRoomTab =
  | "documents"
  | "access"
  | "links"
  | "knowledge"
  | "qa"
  | "activity"
  | "analytics"
  | "settings";

/** Horizontal page tabs under the deal-room header (not left-nav-only sections). */
export const DEAL_ROOM_PAGE_TABS: DealRoomTab[] = [
  "documents",
  "access",
  "links",
  "analytics",
  "knowledge",
];

const VALID_TABS: DealRoomTab[] = [
  "documents",
  "access",
  "links",
  "knowledge",
  "qa",
  "activity",
  "analytics",
  "settings",
];

export function isDealRoomPageTab(tab: DealRoomTab): boolean {
  return DEAL_ROOM_PAGE_TABS.includes(tab);
}

export function useDealRoomTab(): { tab: DealRoomTab; setTab: (tab: DealRoomTab) => void } {
  const [searchParams, setSearchParams] = useSearchParams();

  // Migrate legacy tab query values.
  useEffect(() => {
    const raw = searchParams.get("tab");
    if (raw === "permissions" || raw === "participants") {
      const next = new URLSearchParams(searchParams);
      next.set("tab", "links");
      setSearchParams(next, { replace: true });
    }
  }, [searchParams, setSearchParams]);

  const tab = useMemo<DealRoomTab>(() => {
    const value = searchParams.get("tab") as DealRoomTab | null;
    if (value === "permissions" || value === "participants") {
      return "links";
    }
    return value && VALID_TABS.includes(value) ? value : "documents";
  }, [searchParams]);

  const setTab = (value: DealRoomTab) => {
    const next = new URLSearchParams(searchParams);
    if (value === "documents") {
      next.delete("tab");
    } else {
      next.set("tab", value);
    }
    setSearchParams(next, { replace: true });
  };

  return { tab, setTab };
}
