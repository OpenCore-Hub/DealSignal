import { create } from "zustand";
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

const initialMessages: ChatMessage[] = [
  {
    id: "welcome",
    role: "assistant",
    content:
      "我是 DealSignal AI 助手。接入模型后，我可以帮你解读文档热度、推荐下一步行动、解释访客行为。当前回复为演示逻辑，不引用真实姓名或数字。",
    createdAt: new Date().toISOString(),
  },
];

export const useAIStore = create<AIState>((set) => ({
  open: false,
  messages: initialMessages,
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

    // Simulate network latency until a real AI endpoint is available
    await new Promise((resolve) => setTimeout(resolve, 800));

    const docContext = context?.documentId ? "（基于当前文档上下文）" : "";
    let reply: string;
    if (content.includes("热度") || content.includes("信号")) {
      reply = `我会分析当前文档的访问数据，找出停留时间长、回访频繁的访客，并标注为热度信号。${docContext}`;
    } else if (content.includes("安全") || content.includes("权限")) {
      reply = "我可以检查当前链接的权限配置，并在需要敏感材料时建议提升安全强度。";
    } else if (content.includes("下一步") || content.includes("行动")) {
      reply = "基于高热度访客与文档互动记录，我可以推荐后续跟进动作，例如发送补充材料或安排会议。";
    } else {
      reply = `收到你的问题。接入模型后，我将基于真实文档与访问数据生成可解释的回答。${docContext}`;
    }

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

  reset: () => set({ messages: initialMessages }),
}));
