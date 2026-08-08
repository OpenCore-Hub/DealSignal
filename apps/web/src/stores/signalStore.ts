import { create } from "zustand";
import { api } from "@/lib/api";
import type { ActionItem, Signal } from "@/types";

type SnoozeHours = 24 | 72 | 168;

interface SignalState {
  signals: Signal[];
  actions: ActionItem[];
  loading: boolean;
  error: string | null;
  fetchSignals: () => Promise<void>;
  updateActionStatus: (
    id: string,
    status: ActionItem["status"],
    snoozeHours?: SnoozeHours,
  ) => Promise<void>;
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

  updateActionStatus: async (id, status, snoozeHours) => {
    const previous = get().actions.find((a) => a.id === id);
    set((state) => ({
      actions: state.actions.map((a) =>
        a.id === id
          ? {
              ...a,
              status,
              snoozedUntil:
                status === "snoozed" && snoozeHours
                  ? new Date(Date.now() + snoozeHours * 3600_000).toISOString()
                  : status === "snoozed"
                    ? a.snoozedUntil
                    : undefined,
            }
          : a,
      ),
    }));
    try {
      const updated = await api.updateActionStatus(id, status, snoozeHours);
      set((state) => ({
        actions: state.actions.map((a) => (a.id === id ? updated : a)),
      }));
    } catch (err) {
      if (previous) {
        set((state) => ({
          actions: state.actions.map((a) => (a.id === id ? previous : a)),
        }));
      }
      set({ error: err instanceof Error ? err.message : "Unknown error" });
    }
  },

  getSignalById: (id) => get().signals.find((s) => s.id === id),
}));
