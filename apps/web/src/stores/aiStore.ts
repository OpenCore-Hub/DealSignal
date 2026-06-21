import { create } from "zustand";
import { api } from "@/lib/api";
import type { ChatMessage, Evidence } from "@/types";

interface ChatContext {
  documentId?: string;
  pageNumber?: number;
}

interface AIState {
  open: boolean;
  messages: ChatMessage[];
  pending: boolean;
  sessionId: string | null;
  highlightedEvidence: Evidence | null;
  highlightedPage: number | null;
  toggle: () => void;
  setOpen: (open: boolean) => void;
  sendMessage: (content: string, context?: ChatContext) => Promise<void>;
  reset: () => void;
  setHighlight: (evidence: Evidence | null, page?: number) => void;
  clearHighlight: () => void;
}

const WELCOME_MESSAGE: ChatMessage = {
  id: "welcome",
  role: "assistant",
  content: "ai:welcomeMessage",
  createdAt: new Date().toISOString(),
};

function getInitialMessages(): ChatMessage[] {
  return [WELCOME_MESSAGE];
}

function buildHistory(messages: ChatMessage[]): { role: "user" | "assistant"; content: string }[] {
  return messages
    .filter((m) => m.id !== "welcome")
    .slice(-10)
    .map((m) => ({ role: m.role, content: m.content }));
}

export const useAIStore = create<AIState>((set, get) => ({
  open: false,
  messages: getInitialMessages(),
  pending: false,
  sessionId: null,
  highlightedEvidence: null,
  highlightedPage: null,

  toggle: () => set((state) => ({ open: !state.open })),
  setOpen: (open) => set({ open }),

  sendMessage: async (content, context) => {
    const userMessage: ChatMessage = {
      id: `u_${Date.now()}`,
      role: "user",
      content,
      createdAt: new Date().toISOString(),
    };
    set((state) => ({ messages: [...state.messages, userMessage], pending: true }));

    try {
      const history = buildHistory(get().messages);
      const payload: {
        query: string;
        document_id?: string;
        session_id?: string;
        history?: { role: "user" | "assistant"; content: string }[];
      } = {
        query: content,
        session_id: get().sessionId ?? undefined,
      };
      if (context?.documentId) payload.document_id = context.documentId;
      if (history.length > 0) payload.history = history;

      const res = await api.assistantChat(payload);

      const assistantMessage: ChatMessage = {
        id: `a_${Date.now()}`,
        role: "assistant",
        content: res.answer,
        evidences: res.evidence,
        createdAt: new Date().toISOString(),
      };
      set((state) => ({
        messages: [...state.messages, assistantMessage],
        pending: false,
        sessionId: res.session_id,
      }));
    } catch (e) {
      const errorMessage: ChatMessage = {
        id: `a_${Date.now()}`,
        role: "assistant",
        content: e instanceof Error ? e.message : "Sorry, something went wrong.",
        createdAt: new Date().toISOString(),
      };
      set((state) => ({
        messages: [...state.messages, errorMessage],
        pending: false,
      }));
    }
  },

  reset: () => set({ open: false, pending: false, messages: getInitialMessages(), sessionId: null, highlightedEvidence: null, highlightedPage: null }),

  setHighlight: (evidence, page) =>
    set({ highlightedEvidence: evidence, highlightedPage: page ?? evidence?.page_number ?? null }),

  clearHighlight: () => set({ highlightedEvidence: null, highlightedPage: null }),
}));
