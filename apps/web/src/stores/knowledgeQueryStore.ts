import { create } from "zustand";
import type { DealRoomKnowledgeQueryResult } from "@/types";

/** In-memory Q&A draft so SPA navigations (viewer → back) keep the last answer. */
export interface KnowledgeQueryDraft {
  query: string;
  result: DealRoomKnowledgeQueryResult | null;
  activeCite: number | null;
}

const emptyDraft = (): KnowledgeQueryDraft => ({
  query: "",
  result: null,
  activeCite: null,
});

interface KnowledgeQueryState {
  byRoom: Record<string, KnowledgeQueryDraft>;
  draftFor: (roomId: string) => KnowledgeQueryDraft;
  setDraft: (roomId: string, patch: Partial<KnowledgeQueryDraft>) => void;
  clearRoom: (roomId: string) => void;
  clear: () => void;
}

export const useKnowledgeQueryStore = create<KnowledgeQueryState>((set, get) => ({
  byRoom: {},
  draftFor: (roomId) => get().byRoom[roomId] ?? emptyDraft(),
  setDraft: (roomId, patch) =>
    set((state) => ({
      byRoom: {
        ...state.byRoom,
        [roomId]: { ...(state.byRoom[roomId] ?? emptyDraft()), ...patch },
      },
    })),
  clearRoom: (roomId) =>
    set((state) => {
      const next = { ...state.byRoom };
      delete next[roomId];
      return { byRoom: next };
    }),
  clear: () => set({ byRoom: {} }),
}));
