import type { DealRoomTab } from "@/hooks/useDealRoomTab";

/**
 * Deal-room left-nav IA (B′): one responsibility per tab.
 * Cross-tab: pass signals (badges / one attention banner) only — do not copy pages.
 */
export const DEAL_ROOM_TAB_ROLE: Record<DealRoomTab, string> = {
  documents: "prepare", // content readiness, upload, folder tree
  participants: "outbound", // links, access codes, delivery remediates
  qa: "clarify", // buyer questions + answers
  activity: "audit", // who viewed what, when
  analytics: "insight", // room-level aggregates (deep dive)
  settings: "govern", // room policy (NDA, approval, status)
};

export const DEAL_ROOM_TAB_ORDER: DealRoomTab[] = [
  "documents",
  "participants",
  "qa",
  "activity",
  "analytics",
  "settings",
];

/** Derived stage for P0 — no formal state machine. */
export function deriveRoomStage(activeLinkCount: number): "preparing" | "open" {
  return activeLinkCount > 0 ? "open" : "preparing";
}
