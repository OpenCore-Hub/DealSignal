import { create } from "zustand";
import type { DealRoomTab } from "@/hooks/useDealRoomTab";

/**
 * Cross-tab signals for deal-room sidebar badges.
 * Documents home may surface a single attention banner that jumps to a tab;
 * deep remediates stay on Share / Q&A / Activity — never duplicated as a pulse drawer.
 */
export interface DealRoomNavSignals {
  roomId: string | null;
  /** Failed access-code deliveries across room links (Share badge). */
  failedDeliveries: number;
  /** Pending visitor questions (Q&A badge). */
  unreadQuestions: number;
  activeLinkCount: number;
  viewCount: number;
}

const emptySignals: DealRoomNavSignals = {
  roomId: null,
  failedDeliveries: 0,
  unreadQuestions: 0,
  activeLinkCount: 0,
  viewCount: 0,
};

interface DealRoomNavState extends DealRoomNavSignals {
  setSignals: (signals: DealRoomNavSignals) => void;
  clear: () => void;
}

export const useDealRoomNavStore = create<DealRoomNavState>((set) => ({
  ...emptySignals,
  setSignals: (signals) => set(signals),
  clear: () => set(emptySignals),
}));

export function badgeCountForTab(
  tab: DealRoomTab,
  signals: Pick<DealRoomNavSignals, "failedDeliveries" | "unreadQuestions" | "activeLinkCount">
): number {
  switch (tab) {
    case "participants":
      if (signals.failedDeliveries > 0) return signals.failedDeliveries;
      // Nudge outbound setup when the room has no active share links yet.
      return signals.activeLinkCount === 0 ? 1 : 0;
    case "qa":
      return signals.unreadQuestions;
    default:
      return 0;
  }
}
