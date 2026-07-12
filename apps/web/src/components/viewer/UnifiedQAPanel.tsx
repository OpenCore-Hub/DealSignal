import { useState, useEffect, useRef, useMemo, useCallback } from "react";
import { useTranslation } from "react-i18next";
import { ChatCenteredDots, PaperPlaneRight, Robot, Spinner, User } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { api } from "@/lib/api";
import { useAIStore } from "@/stores/aiStore";
import type { Evidence, VisitorQuestion, ChatMessage } from "@/types";

interface UnifiedQAPanelProps {
  token: string;
  sessionToken?: string;
  documentId?: string;
  qaEnabled?: boolean;
  aiCopilotEnabled?: boolean;
}

type Source = "ai" | "owner" | "you";

interface UIMessage {
  id: string;
  source: Source;
  content: string;
  createdAt: string;
  evidences?: Evidence[];
}

const creds = (token?: string) => (token ? { sessionToken: token } : undefined);

function EvidenceCard({ evidence }: { evidence: Evidence }) {
  const { t } = useTranslation("ai");
  const { setHighlight } = useAIStore();
  return (
    <button
      type="button"
      className="mt-2 w-full rounded-md border border-border bg-muted/50 p-2 text-left text-xs transition-colors hover:bg-muted"
      onClick={() => setHighlight(evidence, evidence.page_number)}
    >
      <span className="text-caption text-muted-foreground">
        {t("evidence.page", { pageNumber: evidence.page_number })}
      </span>
      <p className="mt-0.5 line-clamp-2">{evidence.quote}</p>
    </button>
  );
}

function resolveAIMessage(msg: ChatMessage, t: (key: string, options?: Record<string, unknown>) => string): string {
  if (typeof msg.content === "string" && msg.content.startsWith("ai:")) {
    const key = msg.content;
    const meta = msg as unknown as Record<string, unknown>;
    const query = (meta._query as string) ?? "";
    if (key === "ai:search.results" || key === "ai:search.noResults" || key === "ai:search.error") {
      return t(key, { query });
    }
    return t(key);
  }
  return msg.content;
}

export function UnifiedQAPanel({
  token,
  sessionToken,
  documentId,
  qaEnabled,
  aiCopilotEnabled,
}: UnifiedQAPanelProps) {
  const { t } = useTranslation(["documents", "ai"]);
  const { messages, pending: aiPending, sendMessage } = useAIStore();
  const [questions, setQuestions] = useState<VisitorQuestion[]>([]);
  const [loadingQuestions, setLoadingQuestions] = useState(() => Boolean(qaEnabled));
  const [questionError, setQuestionError] = useState<string | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);
  const [input, setInput] = useState("");
  const [mode, setMode] = useState<"ai" | "owner">(aiCopilotEnabled ? "ai" : "owner");
  const [ownerSubmitting, setOwnerSubmitting] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let cancelled = false;
    if (!qaEnabled) return;
    (async () => {
      setQuestionError(null);
      setLoadingQuestions(true);
      try {
        const res = await api.listPublicQuestions(token, creds(sessionToken));
        if (!cancelled) setQuestions(res.data ?? []);
      } catch {
        if (!cancelled) setQuestionError(t("documents:viewer.qaLoadError"));
      } finally {
        if (!cancelled) setLoadingQuestions(false);
      }
    })();
    return () => { cancelled = true; };
  }, [token, sessionToken, qaEnabled, t, refreshKey]);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages, questions, aiPending]);

  const allMessages = useMemo<UIMessage[]>(() => {
    const list: UIMessage[] = [];
    if (aiCopilotEnabled) {
      messages.forEach((msg) => {
        if (msg.id === "welcome") return;
        const source: Source = msg.role === "user" ? "you" : "ai";
        list.push({
          id: msg.id,
          source,
          content: resolveAIMessage(msg, t),
          createdAt: msg.createdAt,
          evidences: msg.evidences,
        });
      });
    }
    if (qaEnabled) {
      questions.forEach((q) => {
        list.push({
          id: `q_${q.id}`,
          source: "you",
          content: q.question,
          createdAt: q.created_at,
        });
        if (q.answer && q.status === "answered") {
          list.push({
            id: `a_${q.id}`,
            source: "owner",
            content: q.answer,
            createdAt: q.updated_at,
          });
        }
      });
    }
    list.sort((a, b) => new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime());
    return list;
  }, [aiCopilotEnabled, messages, qaEnabled, questions, t]);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      const text = input.trim();
      if (!text) return;

      if (mode === "ai") {
        setInput("");
        await sendMessage(text, { documentId, publicSessionToken: sessionToken });
        return;
      }

      if (text.length > 500) {
        setQuestionError(t("documents:viewer.qaLengthError"));
        return;
      }
      setOwnerSubmitting(true);
      setQuestionError(null);
      setInput("");
      try {
        await api.createPublicQuestion(token, text, creds(sessionToken));
        setRefreshKey((k) => k + 1);
      } catch (e: unknown) {
        const msg = e instanceof Error ? e.message : String(e);
        setQuestionError(msg.includes("disabled") ? t("documents:viewer.qaDisabled") : t("documents:viewer.qaError"));
      } finally {
        setOwnerSubmitting(false);
      }
    },
    [input, mode, documentId, sessionToken, sendMessage, t, token]
  );

  const showModeToggle = aiCopilotEnabled && qaEnabled;
  const busy = aiPending || ownerSubmitting;
  const placeholder = mode === "ai" ? t("documents:viewer.qaAIPlaceholder") : t("documents:viewer.qaOwnerPlaceholder");

  return (
    <div className="flex h-full flex-col bg-card">
      <div
        ref={scrollRef}
        className="flex-1 space-y-4 overflow-y-auto p-3"
        aria-live="polite"
        aria-busy={busy}
      >
        {loadingQuestions ? (
          <div className="flex items-center justify-center py-8">
            <Spinner size={20} className="animate-spin text-muted-foreground" />
          </div>
        ) : allMessages.length === 0 ? (
          <div className="flex flex-col items-center py-10 text-center text-muted-foreground">
            <ChatCenteredDots size={28} className="mb-3 opacity-25" />
            <p className="text-sm font-medium">{t("documents:viewer.qaEmptyUnified")}</p>
          </div>
        ) : (
          allMessages.map((msg) => {
            const isUser = msg.source === "you";
            return (
              <div key={msg.id} className={`flex ${isUser ? "justify-end" : "justify-start"}`}>
                <div className={`max-w-[90%] ${isUser ? "items-end" : "items-start"} flex flex-col gap-1`}>
                  {!isUser && (
                    <span
                      className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide ${
                        msg.source === "ai"
                          ? "bg-primary/10 text-primary"
                          : "bg-warm-100 text-warm-700 dark:bg-warm-900 dark:text-warm-300"
                      }`}
                    >
                      {msg.source === "ai" ? <Robot size={10} /> : <User size={10} />}
                      {t(msg.source === "ai" ? "documents:viewer.qaSourceAI" : "documents:viewer.qaSourceOwner")}
                    </span>
                  )}
                  <div
                    className={`rounded-lg px-3 py-2 text-sm ${
                      isUser
                        ? "bg-primary text-primary-foreground"
                        : "bg-muted text-foreground"
                    }`}
                  >
                    <p className="whitespace-pre-wrap break-words">{msg.content}</p>
                    {msg.evidences?.map((ev) => (
                      <EvidenceCard key={ev.chunk_id} evidence={ev} />
                    ))}
                  </div>
                </div>
              </div>
            );
          })
        )}
        {aiPending && (
          <div className="flex justify-start">
            <div className="flex items-center gap-2 rounded-lg bg-muted px-3 py-2 text-sm text-muted-foreground">
              <Spinner size={14} className="animate-spin" />
              {t("ai:viewer.thinking")}
            </div>
          </div>
        )}
      </div>

      {questionError && (
        <p className="px-3 pt-2 text-center text-xs text-destructive">{questionError}</p>
      )}

      <div className="border-t border-border p-3 space-y-2">
        {showModeToggle && (
          <div className="flex rounded-md border border-border p-0.5">
            <button
              type="button"
              onClick={() => setMode("ai")}
              className={`flex flex-1 items-center justify-center gap-1 rounded px-2 py-1 text-xs font-medium transition-colors ${
                mode === "ai"
                  ? "bg-primary text-primary-foreground"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              <Robot size={12} />
              {t("documents:viewer.qaModeAI")}
            </button>
            <button
              type="button"
              onClick={() => setMode("owner")}
              className={`flex flex-1 items-center justify-center gap-1 rounded px-2 py-1 text-xs font-medium transition-colors ${
                mode === "owner"
                  ? "bg-primary text-primary-foreground"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              <User size={12} />
              {t("documents:viewer.qaModeOwner")}
            </button>
          </div>
        )}
        <form onSubmit={handleSubmit} className="flex gap-2">
          <Textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder={placeholder}
            maxLength={500}
            rows={2}
            className="min-h-0 flex-1 resize-none text-sm"
            disabled={busy}
          />
          <Button
            type="submit"
            size="icon"
            className="h-auto shrink-0"
            disabled={!input.trim() || busy}
            aria-label={t("documents:viewer.qaSubmit")}
          >
            {busy ? <Spinner size={16} className="animate-spin" /> : <PaperPlaneRight size={16} weight="bold" />}
          </Button>
        </form>
      </div>
    </div>
  );
}
