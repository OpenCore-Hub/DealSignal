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
export const DEAL_ROOM_PAGE_TABS = [
  "documents",
  "access",
  "links",
  "analytics",
  "knowledge",
] as const satisfies readonly DealRoomTab[];

export type DealRoomPageTab = (typeof DEAL_ROOM_PAGE_TABS)[number];

/** i18n keys under `dealRooms` for each horizontal page tab. */
export const DEAL_ROOM_PAGE_TAB_LABEL_KEY: Record<DealRoomPageTab, string> = {
  documents: "pageTabs.resources",
  access: "pageTabs.access",
  links: "pageTabs.links",
  analytics: "pageTabs.analytics",
  knowledge: "pageTabs.knowledge",
};

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

export function isDealRoomPageTab(tab: DealRoomTab): tab is DealRoomPageTab {
  return (DEAL_ROOM_PAGE_TABS as readonly DealRoomTab[]).includes(tab);
}

/**
 * Active page tab is always first; remaining tabs keep canonical order.
 * Non-page tabs fall back to the canonical list unchanged.
 */
export function orderDealRoomPageTabs(active: DealRoomTab): DealRoomPageTab[] {
  if (!isDealRoomPageTab(active)) return [...DEAL_ROOM_PAGE_TABS];
  return [active, ...DEAL_ROOM_PAGE_TABS.filter((t) => t !== active)];
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
    if ((value as string) === "permissions" || (value as string) === "participants") {
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
