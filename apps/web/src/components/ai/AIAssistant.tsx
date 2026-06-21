import { useRef, useEffect, useState } from "react";
import { useLocation } from "react-router";
import { motion, AnimatePresence } from "motion/react";
import {
  ChatTeardropText,
  X,
  PaperPlaneRight,
  Sparkle,
  FileText,
  ArrowCounterClockwise,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

import { useAIStore } from "@/stores/aiStore";
import type { Evidence } from "@/types";

function EvidencePanel({ evidences }: { evidences?: Evidence[] }) {
  if (!evidences || evidences.length === 0) return null;
  return (
    <div className="mt-2 rounded-md border border-border bg-muted/50 p-2">
      <p className="text-caption mb-1 flex items-center gap-1 text-muted-foreground">
        <FileText size={12} /> 证据与原文定位
      </p>
      {evidences.map((ev) => (
        <div key={ev.id} className="text-caption">
          <span className="font-medium">第 {ev.pageNumber} 页</span> · {ev.text}
        </div>
      ))}
    </div>
  );
}

export function AIAssistant() {
  const location = useLocation();
  const { open, messages, pending, toggle, setOpen, sendMessage, reset } = useAIStore();
  const [input, setInput] = useState("");
  const [resetDialogOpen, setResetDialogOpen] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);

  const documentIdMatch = location.pathname.match(/\/documents\/([^/]+)/);
  const documentId = documentIdMatch?.[1];

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, open]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim() || pending) return;
    const text = input.trim();
    setInput("");
    await sendMessage(text, { documentId });
  };

  const suggestions = [
    "今天有哪些高热度信号？",
    "Sarah Chen 的行为说明什么？",
    documentId ? "这份文档的关键风险点是什么？" : "如何提升链接安全性？",
  ];

  return (
    <div className="fixed bottom-6 right-6 z-40 flex flex-col items-end gap-3">
      <AnimatePresence>
        {open && (
          <motion.div
            initial={{ opacity: 0, y: 20, scale: 0.95 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: 20, scale: 0.95 }}
            transition={{ duration: 0.25, ease: [0.16, 1, 0.3, 1] }}
            className="flex max-h-[calc(100dvh-6rem)] min-h-[320px] w-full max-w-[calc(100vw-2rem)] flex-col overflow-hidden rounded-xl border border-border bg-card shadow-lg sm:max-w-[420px]"
          >
            <div className="flex items-center justify-between border-b border-border bg-muted/50 px-4 py-3">
              <div className="flex items-center gap-2">
                <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary text-primary-foreground">
                  <Sparkle size={16} weight="fill" />
                </div>
                <div>
                  <p className="text-sm font-medium">DealSignal AI</p>
                  <p className="text-caption text-muted-foreground">
                    {documentId ? "基于当前文档" : "全局助手"}
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-1">
                <Button
                  size="icon"
                  variant="ghost"
                  onClick={() => setResetDialogOpen(true)}
                  aria-label="重置对话"
                >
                  <ArrowCounterClockwise size={16} />
                </Button>
                <Button size="icon" variant="ghost" onClick={() => setOpen(false)} aria-label="关闭">
                  <X size={18} />
                </Button>
              </div>
            </div>

            <div
              className="flex-1 overflow-y-auto px-4 py-4"
              aria-live="polite"
              aria-busy={pending}
            >
              <div className="space-y-4">
                {messages.map((msg) => (
                  <div
                    key={msg.id}
                    className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}
                  >
                    <div
                      className={`max-w-[85%] rounded-2xl px-3 py-2 text-sm ${
                        msg.role === "user"
                          ? "bg-primary text-primary-foreground rounded-br-md"
                          : "bg-muted text-foreground rounded-bl-md"
                      }`}
                    >
                      {msg.content}
                      {msg.role === "assistant" && <EvidencePanel evidences={msg.evidences} />}
                    </div>
                  </div>
                ))}
                {pending && (
                  <div className="flex justify-start">
                    <div className="rounded-2xl rounded-bl-md bg-muted px-3 py-2">
                      <div className="flex gap-1">
                        <span className="h-2 w-2 animate-bounce rounded-full bg-muted-foreground" />
                        <span className="h-2 w-2 animate-bounce rounded-full bg-muted-foreground [animation-delay:0.1s]" />
                        <span className="h-2 w-2 animate-bounce rounded-full bg-muted-foreground [animation-delay:0.2s]" />
                      </div>
                    </div>
                  </div>
                )}
                <div ref={bottomRef} />
              </div>
            </div>

            <Dialog open={resetDialogOpen} onOpenChange={setResetDialogOpen}>
              <DialogContent showCloseButton={false}>
                <DialogHeader>
                  <DialogTitle>重置对话</DialogTitle>
                  <DialogDescription>
                    确定要清空当前对话记录吗？此操作无法撤销。
                  </DialogDescription>
                </DialogHeader>
                <DialogFooter>
                  <Button variant="outline" onClick={() => setResetDialogOpen(false)}>
                    取消
                  </Button>
                  <Button
                    variant="destructive"
                    onClick={() => {
                      reset();
                      setResetDialogOpen(false);
                    }}
                  >
                    重置
                  </Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>

            <div className="border-t border-border p-3">
              {!pending && messages.length <= 2 && (
                <div className="mb-2 flex flex-wrap gap-2">
                  {suggestions.map((s) => (
                    <button
                      key={s}
                      onClick={() => sendMessage(s, { documentId })}
                      className="text-caption rounded-full border border-border bg-muted px-2 py-1 text-left transition-colors hover:bg-muted/80"
                    >
                      {s}
                    </button>
                  ))}
                </div>
              )}
              <form onSubmit={handleSubmit} className="flex gap-2">
                <Input
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  placeholder="询问信号、行动建议或安全设置..."
                  className="flex-1"
                />
                <Button type="submit" size="icon" disabled={!input.trim() || pending}>
                  <PaperPlaneRight size={16} />
                </Button>
              </form>
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      <Button
        size="icon"
        className="h-12 w-12 rounded-full shadow-lg"
        onClick={toggle}
        aria-label={open ? "关闭 AI 助手" : "打开 AI 助手"}
      >
        {open ? <X size={22} /> : <ChatTeardropText size={22} weight="fill" />}
      </Button>
    </div>
  );
}
