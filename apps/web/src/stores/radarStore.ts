import { create } from "zustand";

interface RadarState {
  openCount: number;
  setOpenCount: (n: number) => void;
}

/** Lightweight cross-layout count for TopNav Deal Radar identity. */
export const useRadarStore = create<RadarState>((set) => ({
  openCount: 0,
  setOpenCount: (n) => set({ openCount: Math.max(0, n) }),
}));
