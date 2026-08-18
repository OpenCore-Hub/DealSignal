import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import {
  createKnowledgeTurn,
  reduceKnowledgeStream,
  type KnowledgeTurn,
} from "@/lib/knowledge/streamEvents";
import {
  publicAskTurnToKnowledgeTurn,
  turnNeedsAIStream,
} from "@/lib/visitorAsk/turnModel";
import type { PublicAskTurn } from "@/types";

type Creds = { sessionToken?: string } | undefined;

const creds = (token?: string): Creds => (token ? { sessionToken: token } : undefined);

export function useVisitorAskPanel(opts: {
  token: string;
  sessionToken?: string;
  qaEnabled?: boolean;
}) {
  const { token, sessionToken, qaEnabled = true } = opts;
  const { t } = useTranslation("documents");
  const sessionTokenRef = useRef(sessionToken);
  sessionTokenRef.current = sessionToken;

  const [turns, setTurns] = useState<PublicAskTurn[]>([]);
  const [loading, setLoading] = useState(() => Boolean(qaEnabled));
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [escalatingId, setEscalatingId] = useState<string | null>(null);
  const [liveByTurnId, setLiveByTurnId] = useState<Record<string, KnowledgeTurn>>({});
  const [stoppedTurnIds, setStoppedTurnIds] = useState<Set<string>>(() => new Set());
  const [refreshKey, setRefreshKey] = useState(0);
  const streamAbortRef = useRef<Record<string, AbortController>>({});

  const reloadTurns = useCallback(async () => {
    if (!qaEnabled) return;
    setError(null);
    setLoading(true);
    try {
      const res = await api.listPublicAskTurns(token, creds(sessionTokenRef.current));
      setTurns(res.data ?? []);
    } catch {
      setError(t("viewer.askLoadError"));
    } finally {
      setLoading(false);
    }
  }, [qaEnabled, t, token]);

  useEffect(() => {
    void reloadTurns();
  }, [reloadTurns, refreshKey]);

  const streamTurn = useCallback(
    async (turn: PublicAskTurn) => {
      if (!turnNeedsAIStream(turn)) return;
      streamAbortRef.current[turn.id]?.abort();
      const ac = new AbortController();
      streamAbortRef.current[turn.id] = ac;
      let live = createKnowledgeTurn(turn.question, turn.id);
      setLiveByTurnId((prev) => ({ ...prev, [turn.id]: live }));

      try {
        await api.streamPublicAskTurn(token, turn.id, {
          creds: creds(sessionTokenRef.current),
          signal: ac.signal,
          onEvent: (event) => {
            live = reduceKnowledgeStream(live, event);
            setLiveByTurnId((prev) => ({ ...prev, [turn.id]: live }));
          },
        });
        setRefreshKey((k) => k + 1);
      } catch (e) {
        if (e instanceof DOMException && e.name === "AbortError") return;
        if (e instanceof ApiError && (e.code === "ai_unavailable" || e.code === "ai_not_enabled")) {
          setError(t("viewer.askAiUnavailable"));
        } else if (e instanceof ApiError && e.code === "knowledge_corpus_not_ready") {
          setError(t("viewer.askAiCorpusNotReady"));
        } else if (e instanceof ApiError && e.code === "rate_limit_exceeded") {
          setError(t("viewer.askAiRateLimited"));
        } else if (!(e instanceof ApiError && e.code === "stream_incomplete")) {
          setError(t("viewer.askError"));
        }
      } finally {
        delete streamAbortRef.current[turn.id];
        setLiveByTurnId((prev) => {
          const next = { ...prev };
          delete next[turn.id];
          return next;
        });
      }
    },
    [t, token],
  );

  useEffect(() => {
    if (loading) return;
    for (const turn of turns) {
      if (stoppedTurnIds.has(turn.id)) continue;
      if (turnNeedsAIStream(turn) && !liveByTurnId[turn.id] && !streamAbortRef.current[turn.id]) {
        void streamTurn(turn);
      }
    }
  }, [turns, loading, liveByTurnId, streamTurn, stoppedTurnIds]);

  const stopStream = useCallback((turnId: string) => {
    streamAbortRef.current[turnId]?.abort();
    setStoppedTurnIds((prev) => {
      if (prev.has(turnId)) return prev;
      const next = new Set(prev);
      next.add(turnId);
      return next;
    });
  }, []);

  useEffect(
    () => () => {
      Object.values(streamAbortRef.current).forEach((ac) => ac.abort());
    },
    [],
  );

  const submitQuestion = useCallback(
    async (text: string) => {
      if (text.length > 500) {
        setError(t("viewer.askLengthError"));
        return;
      }
      setSubmitting(true);
      setError(null);
      try {
        const res = await api.createPublicAsk(token, text, creds(sessionTokenRef.current));
        const created = res.data;
        if (turnNeedsAIStream(created)) {
          setTurns((prev) => [...prev, created]);
          void streamTurn(created);
        } else {
          setRefreshKey((k) => k + 1);
        }
      } catch (e: unknown) {
        if (e instanceof ApiError) {
          if (e.code === "qa_disabled") setError(t("viewer.askDisabled"));
          else if (e.code === "rate_limit_exceeded") setError(t("viewer.askRateLimited"));
          else if (e.code === "limiter_unavailable") setError(t("viewer.askLimiterUnavailable"));
          else if (e.code === "formal_not_entitled") setError(t("viewer.askFormalNotEntitled"));
          else setError(t("viewer.askError"));
        } else {
          setError(t("viewer.askError"));
        }
      } finally {
        setSubmitting(false);
      }
    },
    [streamTurn, t, token],
  );

  const escalateToHost = useCallback(
    async (turn: PublicAskTurn) => {
      setEscalatingId(turn.id);
      setError(null);
      try {
        await api.escalatePublicAskTurn(token, turn.id, creds(sessionTokenRef.current));
        setRefreshKey((k) => k + 1);
      } catch {
        setError(t("viewer.askError"));
      } finally {
        setEscalatingId(null);
      }
    },
    [t, token],
  );

  const resolveKnowledgeTurn = useCallback(
    (turn: PublicAskTurn): KnowledgeTurn | null => {
      return liveByTurnId[turn.id] ?? publicAskTurnToKnowledgeTurn(turn);
    },
    [liveByTurnId],
  );

  return {
    turns,
    loading,
    error,
    submitting,
    escalatingId,
    stoppedTurnIds,
    submitQuestion,
    escalateToHost,
    resolveKnowledgeTurn,
    stopStream,
  };
}
