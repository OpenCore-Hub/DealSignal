import { create } from "zustand";
import type { ChatMessage, Evidence } from "@/types";

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
      "我是 DealSignal AI 助手。我可以帮你解读文档热度、推荐下一步行动、解释访客行为。请随时提问。",
    createdAt: new Date().toISOString(),
  },
];

function generateEvidences(documentId?: string): Evidence[] | undefined {
  if (!documentId) return undefined;
  return [
    {
      id: "ev_1",
      pageNumber: 12,
      text: "财务预测显示 2026 年收入 1,200 万美元，毛利率 72%。",
      bbox: { x: 0.1, y: 0.2, w: 0.8, h: 0.1 },
    },
  ];
}

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

    // Simulate AI response
    await new Promise((resolve) => setTimeout(resolve, 1200));

    let reply: string;
    const docContext = context?.documentId ? `（基于当前文档 ID: ${context.documentId}）` : "";
    if (content.includes("热度") || content.includes("信号")) {
      reply = `Sarah Chen 的热度评分最高（92/100），她在财务页停留 5 分钟，并在 24 小时内多次回访。建议 24 小时内跟进。${docContext}`;
    } else if (content.includes("安全") || content.includes("权限")) {
      reply = "当前链接启用了邮箱验证，但未启用白名单。如果材料更敏感，可以提升至中强度权限。";
    } else if (content.includes("下一步") || content.includes("行动")) {
      reply = "建议优先跟进高热度访客：1）Sarah Chen — 发送融资条款摘要；2）Marcus Johnson — 发送团队背景资料。";
    } else {
      reply = `收到你的问题。我正在分析相关数据，稍后可以为你生成可解释的报告。${docContext}`;
    }

    const assistantMessage: ChatMessage = {
      id: `a_${Date.now()}`,
      role: "assistant",
      content: reply,
      evidences: generateEvidences(context?.documentId),
      createdAt: new Date().toISOString(),
    };
    set((state) => ({
      messages: [...state.messages, assistantMessage],
      pending: false,
    }));
  },

  reset: () => set({ messages: initialMessages }),
}));
