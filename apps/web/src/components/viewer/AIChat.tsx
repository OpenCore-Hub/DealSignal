import { useRef, useEffect } from "react";
import { useParams } from "react-router";
import { motion, AnimatePresence } from "motion/react";
import { Robot, X, PaperPlaneRight, Spinner } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useReducedMotion } from "@/hooks/useReducedMotion";
import { useAIStore } from "@/stores/aiStore";
import { useTranslation } from "react-i18next";
import type { Evidence } from "@/types";

function EvidenceCard({ evidence }: { evidence: Evidence }) {
  const { t } = useTranslation("ai");
  return (
    <button
      type="button"
      className="mt-2 w-full rounded-md border border-border bg-muted/50 p-2 text-left text-sm transition-colors hover:bg-muted"
      onClick={() => {
        alert(t("evidence.jumpAlert", { pageNumber: evidence.pageNumber }));
      }}
    >
      <span className="text-caption text-muted-foreground">{t("evidence.page", { pageNumber: evidence.pageNumber })}</span>
      <p className="mt-0.5 line-clamp-2">{evidence.text}</p>
    </button>
  );
}

export function AIChat() {
  const { documentId } = useParams<{ documentId: string }>();
  const { t } = useTranslation("ai");
  const reducedMotion = useReducedMotion();
  const { open, messages, pending, toggle, setOpen, sendMessage } = useAIStore();
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages, open]);

  const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const form = e.currentTarget;
    const formData = new FormData(form);
    const content = String(formData.get("message") || "").trim();
    if (!content) return;
    sendMessage(content, { documentId });
    form.reset();
  };

  return (
    <>
      <AnimatePresence>
        {!open && (
          <motion.button
            initial={reducedMotion ? undefined : { scale: 0.8, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            exit={reducedMotion ? undefined : { scale: 0.8, opacity: 0 }}
            transition={{ duration: 0.2 }}
            onClick={toggle}
            className="fixed right-6 bottom-6 z-40 flex h-14 w-14 items-center justify-center rounded-full bg-primary text-primary-foreground shadow-lg transition-transform hover:scale-105 focus-visible:ring-3 focus-visible:ring-ring/50"
            aria-label={t("viewer.open")}
          >
            <Robot size={24} weight="fill" />
          </motion.button>
        )}
      </AnimatePresence>

      <AnimatePresence>
        {open && (
          <motion.div
            initial={reducedMotion ? undefined : { opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            exit={reducedMotion ? undefined : { opacity: 0, y: 20 }}
            transition={{ duration: 0.25, ease: [0.16, 1, 0.3, 1] }}
            className="fixed right-4 bottom-4 z-40 flex h-[520px] w-full max-w-[calc(100vw-2rem)] flex-col rounded-xl border border-border bg-card shadow-xl sm:max-w-[360px]"
          >
            <div className="flex h-12 items-center justify-between border-b border-border px-4">
              <div className="flex items-center gap-2">
                <Robot size={18} weight="fill" className="text-primary" />
                <span className="text-sm font-medium">{t("viewer.title")}</span>
              </div>
              <Button
                size="icon"
                variant="ghost"
                onClick={() => setOpen(false)}
                aria-label={t("viewer.close")}
              >
                <X size={18} />
              </Button>
            </div>

            <div
              ref={scrollRef}
              className="flex-1 space-y-4 overflow-y-auto p-4"
              aria-live="polite"
              aria-busy={pending}
            >
              {messages.length === 0 && (
                <div className="py-8 text-center text-muted-foreground">
                  <Robot size={32} className="mx-auto mb-2 opacity-40" />
                  <p className="text-sm">{t("viewer.emptyTitle")}</p>
                  <p className="text-caption">{t("viewer.emptyExample")}</p>
                </div>
              )}
              {messages.map((msg) => (
                <div key={msg.id} className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}>
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
              {pending && (
                <div className="flex justify-start">
                  <div className="flex max-w-[85%] items-center gap-2 rounded-lg bg-muted px-3 py-2 text-sm text-muted-foreground">
                    <Spinner size={16} className="animate-spin" />
                    {t("viewer.thinking")}
                  </div>
                </div>
              )}
            </div>

            <div className="border-t border-border p-3">
              <form onSubmit={handleSubmit} className="flex gap-2">
                <Input
                  name="message"
                  placeholder={t("viewer.placeholder")}
                  className="flex-1"
                  autoComplete="off"
                />
                <Button type="submit" size="icon" disabled={pending}>
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
