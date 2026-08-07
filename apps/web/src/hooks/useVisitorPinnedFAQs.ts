import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import type { PublicAskFAQ } from "@/types";

type Creds = { sessionToken?: string } | undefined;

const creds = (token?: string): Creds => (token ? { sessionToken: token } : undefined);

/** Background refresh interval while the viewer tab is visible. */
export const VISITOR_ASK_FAQ_POLL_MS = 30_000;

export function faqListFingerprint(faqs: PublicAskFAQ[]): string {
  return faqs
    .map(
      (faq) =>
        `${faq.id}:${faq.pinned_faq_sort ?? ""}:${faq.pinned_at}:${faq.question}:${faq.answer}`,
    )
    .join("\n");
}

export function useVisitorPinnedFAQs(opts: {
  token: string;
  sessionToken?: string;
  qaEnabled?: boolean;
  pollIntervalMs?: number;
}) {
  const { token, sessionToken, qaEnabled = true, pollIntervalMs = VISITOR_ASK_FAQ_POLL_MS } = opts;
  const { t } = useTranslation("documents");
  const sessionTokenRef = useRef(sessionToken);
  sessionTokenRef.current = sessionToken;
  const fingerprintRef = useRef("");

  const [faqs, setFaqs] = useState<PublicAskFAQ[]>([]);
  const [loading, setLoading] = useState(() => Boolean(qaEnabled));
  const [error, setError] = useState<string | null>(null);

  const applyFaqs = useCallback((next: PublicAskFAQ[]) => {
    const fp = faqListFingerprint(next);
    if (fp === fingerprintRef.current) return false;
    fingerprintRef.current = fp;
    setFaqs(next);
    return true;
  }, []);

  const fetchFaqs = useCallback(
    async (background = false) => {
      if (!qaEnabled) return;
      if (!background) {
        setError(null);
        setLoading(true);
      }
      try {
        const res = await api.listPublicAskFAQs(token, creds(sessionTokenRef.current));
        applyFaqs(res.data ?? []);
        if (!background) setError(null);
      } catch {
        if (!background) {
          setError(t("viewer.askFaqLoadError"));
          fingerprintRef.current = "";
          setFaqs([]);
        }
      } finally {
        if (!background) setLoading(false);
      }
    },
    [applyFaqs, qaEnabled, t, token],
  );

  const reload = useCallback(async () => {
    await fetchFaqs(false);
  }, [fetchFaqs]);

  useEffect(() => {
    void fetchFaqs(false);
  }, [fetchFaqs]);

  useEffect(() => {
    if (!qaEnabled) return;

    let cancelled = false;

    const tick = () => {
      if (cancelled || document.hidden) return;
      void fetchFaqs(true);
    };

    const onVisibility = () => {
      if (!document.hidden) tick();
    };

    const intervalId = window.setInterval(tick, pollIntervalMs);
    document.addEventListener("visibilitychange", onVisibility);

    return () => {
      cancelled = true;
      window.clearInterval(intervalId);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [fetchFaqs, pollIntervalMs, qaEnabled]);

  return { faqs, loading, error, reload };
}
