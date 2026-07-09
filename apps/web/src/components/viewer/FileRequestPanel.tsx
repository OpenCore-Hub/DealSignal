import { useState, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { PaperPlaneTilt, Spinner } from "@phosphor-icons/react";
import type { FileRequest } from "@/types";
import { api } from "@/lib/api";

interface FileRequestPanelProps {
  token: string;
  sessionToken?: string;
}

const creds = (token?: string) =>
  token ? { sessionToken: token } : undefined;

export function FileRequestPanel({ token, sessionToken }: FileRequestPanelProps) {
  const { t } = useTranslation(["documents"]);
  const [message, setMessage] = useState("");
  const [requests, setRequests] = useState<FileRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        setError(null);
        const res = await api.listPublicFileRequests(token, creds(sessionToken));
        if (!cancelled) setRequests(res.data ?? []);
      } catch {
        if (!cancelled) setError(t("documents:viewer.fileRequestLoadError", "Could not load requests"));
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, [token, sessionToken, t, refreshKey]);

  const handleSubmit = async () => {
    const trimmed = message.trim();
    if (!trimmed || trimmed.length > 500) {
      setError(t("documents:viewer.fileRequestLengthError", "Message must be 1–500 characters"));
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await api.createPublicFileRequest(token, trimmed, creds(sessionToken));
      setMessage("");
      setRefreshKey((k) => k + 1);
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      if (msg.includes("too many")) {
        setError(t("documents:viewer.fileRequestTooMany", "Too many pending requests"));
      } else {
        setError(t("documents:viewer.fileRequestError", "Failed to submit request"));
      }
    } finally {
      setSubmitting(false);
    }
  };

  const statusLabel = (s: FileRequest["status"]) => {
    switch (s) {
      case "pending":
        return t("documents:viewer.fileRequestStatusPending", "Pending");
      case "approved":
        return t("documents:viewer.fileRequestStatusApproved", "Approved");
      case "rejected":
        return t("documents:viewer.fileRequestStatusRejected", "Rejected");
      case "fulfilled":
        return t("documents:viewer.fileRequestStatusFulfilled", "Fulfilled");
    }
  };

  const statusColor = (s: FileRequest["status"]) => {
    switch (s) {
      case "pending":
        return "bg-warm-100 text-warm-700 dark:bg-warm-900 dark:text-warm-300";
      case "approved":
        return "bg-success-100 text-success-700 dark:bg-success-900 dark:text-success-300";
      case "rejected":
        return "bg-destructive/10 text-destructive";
      case "fulfilled":
        return "bg-primary/10 text-primary";
    }
  };

  return (
    <div className="flex h-full flex-col">
      <div className="flex-1 overflow-y-auto px-4 py-3 space-y-3">
        {loading ? (
          <div className="flex items-center justify-center py-8">
            <Spinner size={20} className="animate-spin text-muted-foreground" />
          </div>
        ) : requests.length === 0 ? (
          <p className="py-8 text-center text-xs text-muted-foreground">
            {t("documents:viewer.fileRequestEmpty", "No file requests yet. Ask the owner to add missing documents.")}
          </p>
        ) : (
          requests.map((r) => (
            <div key={r.id} className="rounded-lg border border-border bg-card p-3 space-y-1.5">
              <p className="text-sm text-foreground whitespace-pre-wrap break-words">{r.message}</p>
              <div className="flex items-center justify-between">
                <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${statusColor(r.status)}`}>
                  {statusLabel(r.status)}
                </span>
                <time className="text-xs text-muted-foreground" dateTime={r.created_at}>
                  {new Date(r.created_at).toLocaleDateString()}
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
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          placeholder={t("documents:viewer.fileRequestPlaceholder", "Describe what you need...")}
          maxLength={500}
          rows={2}
          className="w-full resize-none rounded-md border border-input bg-background px-3 py-2 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
          disabled={submitting}
        />
        <div className="flex items-center justify-between">
          <span className="text-xs text-muted-foreground">{message.length}/500</span>
          <button
            type="submit"
            disabled={submitting || !message.trim()}
            className="inline-flex items-center gap-1.5 rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50 transition-opacity"
          >
            {submitting ? (
              <Spinner size={14} className="animate-spin" />
            ) : (
              <PaperPlaneTilt size={14} />
            )}
            {t("documents:viewer.fileRequestSubmit", "Send")}
          </button>
        </div>
      </form>
    </div>
  );
}
