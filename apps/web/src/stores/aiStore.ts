import { create } from "zustand";
import i18next from "i18next";
import type { ChatMessage } from "@/types";

interface AIState {
  open: boolean;
  messages: ChatMessage[];
  pending: boolean;
  toggle: () => void;
  setOpen: (open: boolean) => void;
  sendMessage: (content: string, context?: { documentId?: string; pageNumber?: number }) => Promise<void>;
  reset: () => void;
}

function getInitialMessages(): ChatMessage[] {
  return [
    {
      id: "welcome",
      role: "assistant",
      content: i18next.t("ai:welcomeMessage"),
      createdAt: new Date().toISOString(),
    },
  ];
}

function buildReply(content: string, context?: { documentId?: string }): string {
  const docContext = context?.documentId ? i18next.t("ai:replies.documentContext") : "";
  const lower = content.toLowerCase();

  if (lower.includes("heat") || lower.includes("signal")) {
    return `${i18next.t("ai:replies.heatAnalysis")}${docContext}`;
  }
  if (lower.includes("security") || lower.includes("permission")) {
    return i18next.t("ai:replies.securityAdvice");
  }
  if (lower.includes("next") || lower.includes("action")) {
    return i18next.t("ai:replies.followUpAction");
  }
  return `${i18next.t("ai:replies.default")}${docContext}`;
}

export const useAIStore = create<AIState>((set) => ({
  open: false,
  messages: getInitialMessages(),
  pending: false,

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

    await new Promise((resolve) => setTimeout(resolve, 800));

    const reply = buildReply(content, context);

    const assistantMessage: ChatMessage = {
      id: `a_${Date.now()}`,
      role: "assistant",
      content: reply,
      createdAt: new Date().toISOString(),
    };
    set((state) => ({
      messages: [...state.messages, assistantMessage],
      pending: false,
    }));
  },

  reset: () => set({ messages: getInitialMessages() }),
}));
