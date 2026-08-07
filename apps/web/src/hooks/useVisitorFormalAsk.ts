import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import type { PublicFormalAsk } from "@/types";

type Creds = { sessionToken?: string } | undefined;

const creds = (token?: string): Creds => (token ? { sessionToken: token } : undefined);

export const VISITOR_FORMAL_ASK_POLL_MS = 30_000;

export function formalAskListFingerprint(entries: PublicFormalAsk[]): string {
  return entries
    .map((entry) => `${entry.id}:${entry.published_at}:${entry.question}:${entry.answer}`)
    .join("\n");
}

export function useVisitorFormalAsk(opts: {
  token: string;
  sessionToken?: string;
  qaEnabled?: boolean;
  pollIntervalMs?: number;
}) {
  const { token, sessionToken, qaEnabled = true, pollIntervalMs = VISITOR_FORMAL_ASK_POLL_MS } = opts;
  const { t } = useTranslation("documents");
  const sessionTokenRef = useRef(sessionToken);
  sessionTokenRef.current = sessionToken;
  const fingerprintRef = useRef("");

  const [entries, setEntries] = useState<PublicFormalAsk[]>([]);
  const [loading, setLoading] = useState(() => Boolean(qaEnabled));
  const [error, setError] = useState<string | null>(null);

  const applyEntries = useCallback((next: PublicFormalAsk[]) => {
    const fp = formalAskListFingerprint(next);
    if (fp === fingerprintRef.current) return false;
    fingerprintRef.current = fp;
    setEntries(next);
    return true;
  }, []);

  const fetchEntries = useCallback(
    async (background = false) => {
      if (!qaEnabled) return;
      if (!background) {
        setError(null);
        setLoading(true);
      }
      try {
        const res = await api.listPublicFormalAsk(token, creds(sessionTokenRef.current));
        applyEntries(res.data ?? []);
        if (!background) setError(null);
      } catch {
        if (!background) {
          setError(t("viewer.askFormalLoadError"));
          fingerprintRef.current = "";
          setEntries([]);
        }
      } finally {
        if (!background) setLoading(false);
      }
    },
    [applyEntries, qaEnabled, t, token],
  );

  useEffect(() => {
    void fetchEntries(false);
  }, [fetchEntries]);

  useEffect(() => {
    if (!qaEnabled || pollIntervalMs <= 0) return;
    const onVisibility = () => {
      if (document.visibilityState === "visible") void fetchEntries(true);
    };
    document.addEventListener("visibilitychange", onVisibility);
    const id = window.setInterval(() => {
      if (document.visibilityState === "visible") void fetchEntries(true);
    }, pollIntervalMs);
    return () => {
      document.removeEventListener("visibilitychange", onVisibility);
      window.clearInterval(id);
    };
  }, [fetchEntries, pollIntervalMs, qaEnabled]);

  return { entries, loading, error, refetch: () => fetchEntries(false) };
}
