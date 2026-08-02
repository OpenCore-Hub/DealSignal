import { useState, useEffect, useRef, useMemo, useCallback } from "react";
import { useTranslation } from "react-i18next";
import { ChatCenteredDots, PaperPlaneRight, Spinner, User } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import type { VisitorQuestion } from "@/types";

interface UnifiedQAPanelProps {
  token: string;
  sessionToken?: string;
  qaEnabled?: boolean;
}

type Source = "owner" | "you";

interface UIMessage {
  id: string;
  source: Source;
  content: string;
  createdAt: string;
  pendingReply?: boolean;
}

const creds = (token?: string) => (token ? { sessionToken: token } : undefined);

export function UnifiedQAPanel({
  token,
  sessionToken,
  qaEnabled,
}: UnifiedQAPanelProps) {
  const { t } = useTranslation("documents");
  const [questions, setQuestions] = useState<VisitorQuestion[]>([]);
  const [loadingQuestions, setLoadingQuestions] = useState(() => Boolean(qaEnabled));
  const [questionError, setQuestionError] = useState<string | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);
  const [input, setInput] = useState("");
  const [ownerSubmitting, setOwnerSubmitting] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const sessionTokenRef = useRef(sessionToken);
  sessionTokenRef.current = sessionToken;

  useEffect(() => {
    let cancelled = false;
    if (!qaEnabled) return;
    (async () => {
      setQuestionError(null);
      setLoadingQuestions(true);
      try {
        const res = await api.listPublicQuestions(token, creds(sessionTokenRef.current));
        if (!cancelled) setQuestions(res.data ?? []);
      } catch {
        if (!cancelled) setQuestionError(t("viewer.qaLoadError"));
      } finally {
        if (!cancelled) setLoadingQuestions(false);
      }
    })();
    return () => { cancelled = true; };
  }, [token, qaEnabled, t, refreshKey]);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [questions, ownerSubmitting]);

  const allMessages = useMemo<UIMessage[]>(() => {
    const list: UIMessage[] = [];
    if (qaEnabled) {
      questions.forEach((q) => {
        list.push({
          id: `q_${q.id}`,
          source: "you",
          content: q.question,
          createdAt: q.created_at,
          pendingReply: q.status === "pending",
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
  }, [qaEnabled, questions]);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      const text = input.trim();
      if (!text) return;

      if (text.length > 500) {
        setQuestionError(t("viewer.qaLengthError"));
        return;
      }
      setOwnerSubmitting(true);
      setQuestionError(null);
      setInput("");
      try {
        await api.createPublicQuestion(token, text, creds(sessionTokenRef.current));
        setRefreshKey((k) => k + 1);
      } catch (e: unknown) {
        if (e instanceof ApiError) {
          if (e.code === "qa_disabled") {
            setQuestionError(t("viewer.qaDisabled"));
          } else if (e.code === "rate_limit_exceeded") {
            setQuestionError(t("viewer.qaRateLimited"));
          } else if (e.code === "limiter_unavailable") {
            setQuestionError(t("viewer.qaLimiterUnavailable"));
          } else {
            setQuestionError(t("viewer.qaError"));
          }
        } else {
          setQuestionError(t("viewer.qaError"));
        }
      } finally {
        setOwnerSubmitting(false);
      }
    },
    [input, t, token]
  );

  const busy = ownerSubmitting;

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
            <p className="text-sm font-medium">{t("viewer.qaEmptyUnified")}</p>
          </div>
        ) : (
          allMessages.map((msg) => {
            const isUser = msg.source === "you";
            return (
              <div key={msg.id} className={`flex ${isUser ? "justify-end" : "justify-start"}`}>
                <div className={`max-w-[90%] ${isUser ? "items-end" : "items-start"} flex flex-col gap-1`}>
                  {!isUser && (
                    <span className="inline-flex items-center gap-1 rounded-full bg-warm-100 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-warm-700 dark:bg-warm-900 dark:text-warm-300">
                      <User size={10} />
                      {t("viewer.qaSourceOwner")}
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
                  </div>
                  {msg.pendingReply && (
                    <span className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
                      {t("viewer.qaPendingReply")}
                    </span>
                  )}
                </div>
              </div>
            );
          })
        )}
      </div>

      {questionError && (
        <p className="px-3 pt-2 text-center text-xs text-destructive">{questionError}</p>
      )}

      <div className="space-y-2 border-t border-border p-3">
        <form onSubmit={handleSubmit} className="flex gap-2">
          <Textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder={t("viewer.qaOwnerPlaceholder")}
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
            aria-label={t("viewer.qaSubmit")}
          >
            {busy ? <Spinner size={16} className="animate-spin" /> : <PaperPlaneRight size={16} weight="bold" />}
          </Button>
        </form>
      </div>
    </div>
  );
}
