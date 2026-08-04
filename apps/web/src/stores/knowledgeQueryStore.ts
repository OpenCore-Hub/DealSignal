import { create } from "zustand";
import type {
  DealRoomKnowledgeQATurn,
  DealRoomKnowledgeSessionState,
} from "@/types";

/** Room-scoped Q&A draft: composer + active session + turn timeline. */
export interface KnowledgeQueryDraft {
  query: string;
  activeSessionId: string | null;
  turns: DealRoomKnowledgeQATurn[];
  activeCite: number | null;
  /** Auditable session.state for the research desk rail (Phase L). */
  sessionState?: DealRoomKnowledgeSessionState | null;
}

const emptyDraft = (): KnowledgeQueryDraft => ({
  query: "",
  activeSessionId: null,
  turns: [],
  activeCite: null,
  sessionState: null,
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
