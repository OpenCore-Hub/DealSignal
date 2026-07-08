import { useMemo } from "react";
import { useSearchParams } from "react-router";

export type DealRoomTab = "documents" | "permissions" | "analytics" | "qa";

export function useDealRoomTab(): { tab: DealRoomTab; setTab: (tab: DealRoomTab) => void } {
  const [searchParams, setSearchParams] = useSearchParams();
  const tab = useMemo<DealRoomTab>(() => {
    const value = searchParams.get("tab") as DealRoomTab | null;
    const valid: DealRoomTab[] = ["documents", "permissions", "analytics", "qa"];
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
