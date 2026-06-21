import { useRef, useEffect, useState } from "react";
import { useLocation, useNavigate } from "react-router";
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
import { useTranslation } from "react-i18next";
import type { Evidence } from "@/types";

function EvidenceCard({ evidence, documentId }: { evidence: Evidence; documentId?: string }) {
  const { t } = useTranslation("ai");
  const navigate = useNavigate();
  const { setHighlight } = useAIStore();

  return (
    <button
      type="button"
      disabled={!documentId}
      className="mt-2 w-full rounded-md border border-border bg-muted/50 p-2 text-left text-sm transition-colors hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
      onClick={() => {
        if (!documentId) return;
        setHighlight(evidence, evidence.page_number);
        navigate(`/viewer/${documentId}?page=${evidence.page_number}`);
      }}
    >
      <span className="text-caption text-muted-foreground">
        {t("evidence.page", { pageNumber: evidence.page_number })}
      </span>
      <p className="mt-0.5 line-clamp-2">{evidence.quote}</p>
    </button>
  );
}

function EvidencePanel({ evidences, documentId }: { evidences?: Evidence[]; documentId?: string }) {
  const { t } = useTranslation("ai");
  if (!evidences || evidences.length === 0) return null;
  return (
    <div className="mt-2 rounded-md border border-border bg-muted/50 p-2">
      <p className="text-caption mb-1 flex items-center gap-1 text-muted-foreground">
        <FileText size={12} /> {t("evidence.title")}
      </p>
      {evidences.map((ev) => (
        <EvidenceCard key={ev.chunk_id} evidence={ev} documentId={documentId} />
      ))}
    </div>
  );
}

export function AIAssistant() {
  const location = useLocation();
  const { t } = useTranslation("ai");
  const { open, messages, pending, toggle, setOpen, sendMessage, reset } = useAIStore();
  const [input, setInput] = useState("");
  const [resetDialogOpen, setResetDialogOpen] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);

  const documentIdMatch = location.pathname.match(/\/(?:documents|viewer)\/([^/]+)/);
  const documentId = documentIdMatch?.[1];

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, open]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim() || pending) return;
    const text = input.trim();
    setInput("");
    await sendMessage(text, documentId ? { documentId } : undefined);
  };

  const handleReset = () => {
    reset();
    setResetDialogOpen(false);
  };

  return (
    <div className="fixed right-4 bottom-4 z-40 flex flex-col items-end">
      <Dialog open={resetDialogOpen} onOpenChange={setResetDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("resetDialog.title")}</DialogTitle>
            <DialogDescription>{t("resetDialog.description")}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setResetDialogOpen(false)}>
              {t("resetDialog.cancel")}
            </Button>
            <Button onClick={handleReset}>{t("resetDialog.confirm")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AnimatePresence>
        {open && (
          <motion.div
            initial={{ opacity: 0, y: 20, scale: 0.95 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: 20, scale: 0.95 }}
            transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
            className="mb-4 flex h-[520px] w-full max-w-[calc(100vw-2rem)] flex-col rounded-xl border border-border bg-card shadow-xl sm:w-[360px]"
          >
            <div className="flex h-12 items-center justify-between border-b border-border px-4">
              <div className="flex items-center gap-2">
                <Sparkle size={18} weight="fill" className="text-primary" />
                <span className="text-sm font-medium">{t("viewer.title")}</span>
              </div>
              <div className="flex items-center gap-1">
                <Button
                  size="icon"
                  variant="ghost"
                  onClick={() => setResetDialogOpen(true)}
                  aria-label={t("aria.reset")}
                >
                  <ArrowCounterClockwise size={18} />
                </Button>
                <Button size="icon" variant="ghost" onClick={() => setOpen(false)} aria-label={t("aria.close")}>
                  <X size={18} />
                </Button>
              </div>
            </div>

            <div className="flex-1 space-y-4 overflow-y-auto p-4">
              {messages.length === 0 && (
                <div className="py-8 text-center text-muted-foreground">
                  <ChatTeardropText size={32} className="mx-auto mb-2 opacity-40" />
                  <p className="text-sm">{t("viewer.emptyTitle")}</p>
                  <p className="text-caption">{t("viewer.emptyExample")}</p>
                </div>
              )}
              {messages.map((msg) => {
                const content = msg.id === "welcome" ? t(msg.content) : msg.content;
                return (
                  <div key={msg.id} className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}>
                    <div
                      className={`max-w-[85%] rounded-lg px-3 py-2 text-sm ${
                        msg.role === "user" ? "bg-primary text-primary-foreground" : "bg-muted text-foreground"
                      }`}
                    >
                      <p>{content}</p>
                      {msg.role === "assistant" && <EvidencePanel evidences={msg.evidences} documentId={documentId} />}
                    </div>
                  </div>
                );
              })}
              {pending && (
                <div className="flex justify-start">
                  <div className="flex max-w-[85%] items-center gap-2 rounded-lg bg-muted px-3 py-2 text-sm text-muted-foreground">
                    <span className="h-4 w-4 animate-pulse rounded-full bg-primary/60" />
                    {t("viewer.thinking")}
                  </div>
                </div>
              )}
              <div ref={bottomRef} />
            </div>

            <form onSubmit={handleSubmit} className="flex gap-2 border-t border-border p-3">
              <Input
                value={input}
                onChange={(e) => setInput(e.target.value)}
                placeholder={t("inputPlaceholder")}
                className="flex-1"
              />
              <Button type="submit" size="icon" disabled={!input.trim() || pending} aria-label={t("aria.send")}>
                <PaperPlaneRight size={16} />
              </Button>
            </form>
          </motion.div>
        )}
      </AnimatePresence>

      <Button
        size="icon"
        className="h-12 w-12 rounded-full shadow-lg"
        onClick={toggle}
        aria-label={open ? t("aria.close") : t("aria.open")}
      >
        {open ? <X size={22} /> : <ChatTeardropText size={22} weight="fill" />}
      </Button>
    </div>
  );
}
