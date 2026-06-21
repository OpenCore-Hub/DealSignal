import { useState, useRef, useEffect } from "react";
import { motion, AnimatePresence } from "motion/react";
import { Robot, X, PaperPlaneRight, Spinner } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useReducedMotion } from "@/hooks/useReducedMotion";
import type { ChatMessage, Evidence } from "@/types";

const mockResponse: ChatMessage = {
  id: "msg_2",
  role: "assistant",
  content:
    "根据文档内容，Acme 在 2026 年预计收入为 $4.2M，毛利率 72%。这一数据出现在财务预测第 7 页。",
  evidences: [
    {
      id: "ev_1",
      pageNumber: 7,
      text: "2026 年预计收入 $4.2M，毛利率 72%",
      bbox: { x: 120, y: 340, w: 420, h: 40 },
    },
  ],
  createdAt: new Date().toISOString(),
};

function EvidenceCard({ evidence }: { evidence: Evidence }) {
  return (
    <button
      className="mt-2 w-full rounded-md border border-border bg-muted/50 p-2 text-left text-sm transition-colors hover:bg-muted"
      onClick={() => {
        // Placeholder: would jump to page and highlight bbox
        alert(`跳转到第 ${evidence.pageNumber} 页并高亮引用`);
      }}
    >
      <span className="text-caption text-muted-foreground">第 {evidence.pageNumber} 页</span>
      <p className="mt-0.5 line-clamp-2">{evidence.text}</p>
    </button>
  );
}

export function AIChat() {
  const [open, setOpen] = useState(false);
  const [input, setInput] = useState("");
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [loading, setLoading] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const reducedMotion = useReducedMotion();

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages, open]);

  const sendMessage = () => {
    if (!input.trim()) return;

    const userMessage: ChatMessage = {
      id: Math.random().toString(36).slice(2),
      role: "user",
      content: input,
      createdAt: new Date().toISOString(),
    };

    setMessages((prev) => [...prev, userMessage]);
    setInput("");
    setLoading(true);

    setTimeout(() => {
      setMessages((prev) => [...prev, mockResponse]);
      setLoading(false);
    }, 1200);
  };

  return (
    <>
      {/* Floating toggle button */}
      <AnimatePresence>
        {!open && (
          <motion.button
            initial={reducedMotion ? undefined : { scale: 0.8, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            exit={reducedMotion ? undefined : { scale: 0.8, opacity: 0 }}
            transition={{ duration: 0.2 }}
            onClick={() => setOpen(true)}
            className="fixed right-6 bottom-6 z-40 flex h-14 w-14 items-center justify-center rounded-full bg-primary text-primary-foreground shadow-lg transition-transform hover:scale-105 focus-visible:ring-2 focus-visible:ring-ring"
            aria-label="打开 AI 助手"
          >
            <Robot size={24} weight="fill" />
          </motion.button>
        )}
      </AnimatePresence>

      {/* Chat panel */}
      <AnimatePresence>
        {open && (
          <motion.div
            initial={reducedMotion ? undefined : { opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            exit={reducedMotion ? undefined : { opacity: 0, y: 20 }}
            transition={{ duration: 0.25, ease: [0.16, 1, 0.3, 1] }}
            className="fixed right-4 bottom-4 z-40 flex h-[520px] w-[360px] flex-col rounded-xl border border-border bg-card shadow-xl"
          >
            {/* Header */}
            <div className="flex h-12 items-center justify-between border-b border-border px-4">
              <div className="flex items-center gap-2">
                <Robot size={18} weight="fill" className="text-primary" />
                <span className="text-sm font-medium">AI 助手</span>
              </div>
              <button
                onClick={() => setOpen(false)}
                className="text-muted-foreground hover:text-foreground"
                aria-label="关闭 AI 助手"
              >
                <X size={18} />
              </button>
            </div>

            {/* Messages */}
            <div
              ref={scrollRef}
              className="flex-1 space-y-4 overflow-y-auto p-4"
            >
              {messages.length === 0 && (
                <div className="py-8 text-center text-muted-foreground">
                  <Robot size={32} className="mx-auto mb-2 opacity-40" />
                  <p className="text-sm">向 AI 提问关于文档的内容</p>
                  <p className="text-caption">例如："这家公司的毛利率是多少？"</p>
                </div>
              )}
              {messages.map((msg) => (
                <div
                  key={msg.id}
                  className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}
                >
                  <div
                    className={`max-w-[85%] rounded-lg px-3 py-2 text-sm ${
                      msg.role === "user"
                        ? "bg-primary text-primary-foreground"
                        : "bg-muted text-foreground"
                    }`}
                  >
                    <p>{msg.content}</p>
                    {msg.evidences?.map((ev) => (
                      <EvidenceCard key={ev.id} evidence={ev} />
                    ))}
                  </div>
                </div>
              ))}
              {loading && (
                <div className="flex justify-start">
                  <div className="flex max-w-[85%] items-center gap-2 rounded-lg bg-muted px-3 py-2 text-sm text-muted-foreground">
                    <Spinner size={16} className="animate-spin" />
                    思考中...
                  </div>
                </div>
              )}
            </div>

            {/* Input */}
            <div className="border-t border-border p-3">
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  sendMessage();
                }}
                className="flex gap-2"
              >
                <Input
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  placeholder="输入问题..."
                  className="flex-1"
                />
                <Button type="submit" size="icon" disabled={!input.trim() || loading}>
                  <PaperPlaneRight size={16} weight="bold" />
                </Button>
              </form>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </>
  );
}
