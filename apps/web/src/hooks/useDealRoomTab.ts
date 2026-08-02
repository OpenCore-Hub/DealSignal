import { useEffect, useMemo } from "react";
import { useSearchParams } from "react-router";

export type DealRoomTab =
  | "documents"
  | "participants"
  | "qa"
  | "activity"
  | "analytics"
  | "settings";

export function useDealRoomTab(): { tab: DealRoomTab; setTab: (tab: DealRoomTab) => void } {
  const [searchParams, setSearchParams] = useSearchParams();

  // Migrate legacy "permissions" tab to the new "participants" section.
  useEffect(() => {
    if (searchParams.get("tab") === "permissions") {
      const next = new URLSearchParams(searchParams);
      next.set("tab", "participants");
      setSearchParams(next, { replace: true });
    }
  }, [searchParams, setSearchParams]);

  const tab = useMemo<DealRoomTab>(() => {
    const value = searchParams.get("tab") as DealRoomTab | null;
    const valid: DealRoomTab[] = [
      "documents",
      "participants",
      "qa",
      "activity",
      "analytics",
      "settings",
    ];
    return value && valid.includes(value) ? value : "documents";
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
