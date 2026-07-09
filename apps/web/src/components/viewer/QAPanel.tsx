import { useState, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { ChatCenteredDots, Spinner } from "@phosphor-icons/react";
import type { VisitorQuestion } from "@/types";
import { api } from "@/lib/api";

interface QAPanelProps {
  token: string;
  sessionToken?: string;
}

const creds = (token?: string) =>
  token ? { sessionToken: token } : undefined;

export function QAPanel({ token, sessionToken }: QAPanelProps) {
  const { t } = useTranslation(["documents"]);
  const [question, setQuestion] = useState("");
  const [questions, setQuestions] = useState<VisitorQuestion[]>([]);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        setError(null);
        const res = await api.listPublicQuestions(token, creds(sessionToken));
        if (!cancelled) setQuestions(res.data ?? []);
      } catch {
        if (!cancelled) setError(t("documents:viewer.qaLoadError", "Could not load questions"));
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, [token, sessionToken, t, refreshKey]);

  const handleSubmit = async () => {
    const trimmed = question.trim();
    if (!trimmed || trimmed.length > 500) {
      setError(t("documents:viewer.qaLengthError", "Question must be 1–500 characters"));
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await api.createPublicQuestion(token, trimmed, creds(sessionToken));
      setQuestion("");
      setRefreshKey((k) => k + 1);
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      setError(msg.includes("disabled") ? t("documents:viewer.qaDisabled", "Q&A is not available for this link") : t("documents:viewer.qaError", "Failed to submit question"));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="flex h-full flex-col">
      <div className="flex-1 overflow-y-auto px-4 py-3 space-y-3">
        {loading ? (
          <div className="flex items-center justify-center py-8">
            <Spinner size={20} className="animate-spin text-muted-foreground" />
          </div>
        ) : questions.length === 0 ? (
          <p className="py-8 text-center text-xs text-muted-foreground">
            {t("documents:viewer.qaEmpty", "No questions yet. Ask the owner about this document.")}
          </p>
        ) : (
          questions.map((q) => (
            <div key={q.id} className="rounded-lg border border-border bg-card p-3 space-y-2">
              <div className="space-y-1">
                <div className="rounded-md bg-muted px-3 py-2">
                  <p className="text-sm text-foreground whitespace-pre-wrap break-words">{q.question}</p>
                </div>
                {q.answer ? (
                  <div className="rounded-md bg-primary/5 border border-primary/10 px-3 py-2">
                    <p className="text-xs font-medium text-primary mb-0.5">
                      {t("documents:viewer.qaOwnerReply", "Owner reply")}
                    </p>
                    <p className="text-sm text-foreground whitespace-pre-wrap break-words">{q.answer}</p>
                  </div>
                ) : null}
              </div>
              <div className="flex items-center justify-between">
                <span
                  className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${
                    q.status === "answered"
                      ? "bg-success-100 text-success-700 dark:bg-success-900 dark:text-success-300"
                      : "bg-warm-100 text-warm-700 dark:bg-warm-900 dark:text-warm-300"
                  }`}
                >
                  {q.status === "answered"
                    ? t("documents:viewer.qaAnswered", "Answered")
                    : t("documents:viewer.qaPending", "Awaiting answer")}
                </span>
                <time className="text-xs text-muted-foreground" dateTime={q.created_at}>
                  {new Date(q.created_at).toLocaleDateString()}
                </time>
              </div>
            </div>
          ))
        )}
        {error && (
          <p className="text-xs text-destructive text-center">{error}</p>
        )}
      </div>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          handleSubmit();
        }}
        className="shrink-0 border-t border-border p-3 space-y-2"
      >
        <textarea
          value={question}
          onChange={(e) => setQuestion(e.target.value)}
          placeholder={t("documents:viewer.qaPlaceholder", "Ask a question...")}
          maxLength={500}
          rows={2}
          className="w-full resize-none rounded-md border border-input bg-background px-3 py-2 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
          disabled={submitting}
        />
        <div className="flex items-center justify-between">
          <span className="text-xs text-muted-foreground">{question.length}/500</span>
          <button
            type="submit"
            disabled={submitting || !question.trim()}
            className="inline-flex items-center gap-1.5 rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50 transition-opacity"
          >
            {submitting ? (
              <Spinner size={14} className="animate-spin" />
            ) : (
              <ChatCenteredDots size={14} />
            )}
            {t("documents:viewer.qaSubmit", "Ask")}
          </button>
        </div>
      </form>
    </div>
  );
}
