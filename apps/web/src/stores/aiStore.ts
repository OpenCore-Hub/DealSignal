import { create } from "zustand";
import { api } from "@/lib/api";
import type { ChatMessage, Evidence } from "@/types";

// i18n keys stored in the store; the AIAssistant component resolves them via t().
// The raw server answers (assistantChat) are passed through directly.
const I18N_KEYS = {
  searchResults: "ai:search.results",
  searchNoResults: "ai:search.noResults",
  searchError: "ai:search.error",
} as const;

interface ChatContext {
  documentId?: string;
  pageNumber?: number;
  publicToken?: string;
  publicSessionToken?: string;
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

      if (context?.publicSessionToken && context.publicToken) {
        const res = await api.publicAssistantChat(
          context.publicToken,
          { message: content, session_id: get().sessionId ?? undefined },
          context.publicSessionToken
        );
        assistantMessage = {
          id: `a_${Date.now()}`,
          role: "assistant",
          content: res.answer,
          evidences: res.evidence,
          createdAt: new Date().toISOString(),
        };
        set({ sessionId: res.session_id });
      } else if (context?.documentId) {
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
          // Store i18n key so the component can resolve it with interpolation.
          content:
            evidences.length > 0
              ? I18N_KEYS.searchResults
              : I18N_KEYS.searchNoResults,
          evidences,
          createdAt: new Date().toISOString(),
        };
        if (evidences.length > 0) {
          // Embed the query in metadata so the component can interpolate it.
        (assistantMessage as unknown as Record<string, unknown>)._query = res.query ?? content;
      } else {
        (assistantMessage as unknown as Record<string, unknown>)._query = res.query ?? content;
        }
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
    } catch {
      const errorMessage: ChatMessage = {
        id: `a_${Date.now()}`,
        role: "assistant",
        content: I18N_KEYS.searchError,
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
