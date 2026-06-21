import { create } from "zustand";
import { api } from "@/lib/api";
import type { ActionItem, Signal } from "@/types";

interface SignalState {
  signals: Signal[];
  actions: ActionItem[];
  loading: boolean;
  error: string | null;
  fetchSignals: () => Promise<void>;
  updateActionStatus: (id: string, status: ActionItem["status"]) => void;
  getSignalById: (id: string) => Signal | undefined;
}

export const useSignalStore = create<SignalState>((set, get) => ({
  signals: [],
  actions: [],
  loading: false,
  error: null,

  fetchSignals: async () => {
    set({ loading: true, error: null });
    try {
      const feed = await api.getSignals();
      set({ signals: feed.signals, actions: feed.actions, loading: false });
    } catch (err) {
      set({ error: err instanceof Error ? err.message : "Unknown error", loading: false });
    }
  },

  updateActionStatus: (id, status) => {
    set((state) => ({
      actions: state.actions.map((a) => (a.id === id ? { ...a, status } : a)),
    }));
  },

  getSignalById: (id) => get().signals.find((s) => s.id === id),
}));
