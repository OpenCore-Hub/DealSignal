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

function mapSearchResult(result: unknown): Evidence {
  const r = result as {
    chunk_id: string;
    quote?: string;
    normalized_text?: string;
    page_number: number;
    boxes: Evidence["boxes"];
    score: number;
  };
  return {
    chunk_id: r.chunk_id,
    quote: r.quote ?? r.normalized_text ?? "",
    page_number: r.page_number,
    boxes: r.boxes,
    score: r.score,
  };
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
      let assistantMessage: ChatMessage;

      if (context?.documentId) {
        const res = await api.searchDocument({
          query: content,
          document_id: context.documentId,
          mode: "hybrid",
          top_k: 5,
        });
        const evidences = res.results.map(mapSearchResult);
        assistantMessage = {
          id: `a_${Date.now()}`,
          role: "assistant",
          content:
            evidences.length > 0
              ? `Here are the most relevant passages for "${res.query}":`
              : `No relevant passages found for "${res.query}" in this document.`,
          evidences,
          createdAt: new Date().toISOString(),
        };
      } else {
        const res = await api.assistantChat({
          message: content,
          session_id: get().sessionId ?? undefined,
        });
        assistantMessage = {
          id: `a_${Date.now()}`,
          role: "assistant",
          content: res.answer,
          evidences: res.evidence,
          createdAt: new Date().toISOString(),
        };
        set({ sessionId: res.session_id });
      }

      set((state) => ({
        messages: [...state.messages, assistantMessage],
        pending: false,
      }));
    } catch (e) {
      const errorMessage: ChatMessage = {
        id: `a_${Date.now()}`,
        role: "assistant",
        content: e instanceof Error ? e.message : "Sorry, the search failed. Please try again later.",
        createdAt: new Date().toISOString(),
      };
      set((state) => ({
        messages: [...state.messages, errorMessage],
        pending: false,
      }));
    }
  },

  reset: () =>
    set({
      open: false,
      pending: false,
      messages: getInitialMessages(),
      sessionId: null,
      highlightedEvidence: null,
      highlightedPage: null,
    }),

  setHighlight: (evidence, page) =>
    set({ highlightedEvidence: evidence, highlightedPage: page ?? evidence?.page_number ?? null }),

  clearHighlight: () => set({ highlightedEvidence: null, highlightedPage: null }),
}));
