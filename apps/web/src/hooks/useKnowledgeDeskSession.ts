import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import { resolveCiteOpenOutcome } from "@/lib/knowledge/citeOutcome";
import {
  isKnowledgeAskGateReject,
  knowledgeErrorMessage,
} from "@/lib/knowledge/errors";
import { asFeedbackKind } from "@/lib/knowledge/feedback";
import {
  createKnowledgeTurn,
  reduceKnowledgeStream,
  turnFromQATurn,
  type KnowledgeTurn,
} from "@/lib/knowledge/streamEvents";
import { useKnowledgeQueryStore } from "@/stores/knowledgeQueryStore";
import type {
  DealRoomKnowledgeFeedbackKind,
  DealRoomKnowledgeQATurn,
} from "@/types";

const EMPTY_TURNS: DealRoomKnowledgeQATurn[] = [];

export interface UseKnowledgeDeskSessionOptions {
  /** When false, Ask is blocked (e.g. corpus not ready). Default true. */
  allowAsk?: boolean;
  /** Fired once after hydrate finds an active session with turns. */
  onActiveSessionHydrated?: () => void;
}

/**
 * Shared session ask/hydrate/feedback for Knowledge Tab + owner Viewer rail
 * (ceiling Phase T/V). JWT workspace knowledge routes only.
 */
export function useKnowledgeDeskSession(
  roomId: string,
  opts: UseKnowledgeDeskSessionOptions = {},
) {
  const { allowAsk = true, onActiveSessionHydrated } = opts;
  const { t } = useTranslation("dealRooms");

  const draft = useKnowledgeQueryStore((s) => s.byRoom[roomId]);
  const query = draft?.query ?? "";
  const activeSessionId = draft?.activeSessionId ?? null;
  const turns = draft?.turns ?? EMPTY_TURNS;
  const activeCite = draft?.activeCite ?? null;
  const sessionState = draft?.sessionState ?? null;

  const [asking, setAsking] = useState(false);
  const [sessionHydrated, setSessionHydrated] = useState(false);
  const [liveTurn, setLiveTurn] = useState<KnowledgeTurn | null>(null);
  const askAbortRef = useRef<AbortController | null>(null);
  const askingRef = useRef(false);
  const userStoppedRef = useRef(false);
  const pendingAskRef = useRef<{ query: string; clientRequestId: string } | null>(
    null,
  );
  const hydratedCbRef = useRef(onActiveSessionHydrated);
  hydratedCbRef.current = onActiveSessionHydrated;

  const setQuery = (value: string) =>
    useKnowledgeQueryStore.getState().setDraft(roomId, { query: value });
  const setActiveCite = (value: number | null) =>
    useKnowledgeQueryStore.getState().setDraft(roomId, { activeCite: value });

  const viewTurns = useMemo(() => {
    const lastIdx = turns.length - 1;
    const mapped = turns.map((row, idx) =>
      turnFromQATurn(row, idx === lastIdx && !liveTurn ? activeCite : null),
    );
    return liveTurn ? [...mapped, liveTurn] : mapped;
  }, [turns, activeCite, liveTurn]);

  useEffect(() => {
    let cancelled = false;
    setSessionHydrated(false);
    void (async () => {
      try {
        const detail = await api.getActiveDealRoomKnowledgeSession(roomId);
        if (cancelled) return;
        const serverTurns = detail.turns ?? [];
        const sessionId = detail.session?.id ?? null;
        if (sessionId && serverTurns.length > 0) {
          useKnowledgeQueryStore.getState().setDraft(roomId, {
            activeSessionId: sessionId,
            turns: serverTurns,
            activeCite: null,
            sessionState: detail.session?.state ?? null,
          });
          hydratedCbRef.current?.();
        }
      } catch {
        /* Non-fatal: desk still works for a fresh session on first ask. */
      } finally {
        if (!cancelled) setSessionHydrated(true);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [roomId]);

  const hydratePendingTurns = async (
    baselineTurnCount: number,
    question?: string,
  ) => {
    const deadline = Date.now() + 11_000;
    let delayMs = 120;
    while (Date.now() < deadline) {
      try {
        const detail = await api.getActiveDealRoomKnowledgeSession(roomId);
        const serverTurns = detail.turns ?? [];
        const sessionId = detail.session?.id ?? null;
        const matchedQuestion =
          question &&
          serverTurns.some((turn) => turn.question === question);
        if (
          sessionId &&
          (serverTurns.length > baselineTurnCount || matchedQuestion)
        ) {
          useKnowledgeQueryStore.getState().setDraft(roomId, {
            activeSessionId: sessionId,
            turns: serverTurns,
            activeCite: null,
            sessionState: detail.session?.state ?? null,
          });
          return;
        }
      } catch {
        /* retry until deadline */
      }
      await new Promise((r) => setTimeout(r, delayMs));
      delayMs = Math.min(Math.round(delayMs * 1.6), 700);
    }
  };

  const onStop = () => {
    userStoppedRef.current = true;
    askAbortRef.current?.abort();
  };

  const newClientRequestId = () =>
    typeof crypto !== "undefined" && "randomUUID" in crypto
      ? crypto.randomUUID()
      : `kqa_${Date.now()}_${Math.random().toString(36).slice(2)}`;

  const onAsk = async (overrideQuery?: string) => {
    const q = (typeof overrideQuery === "string" ? overrideQuery : query).trim();
    if (!q || askingRef.current) return;
    if (typeof overrideQuery === "string") {
      setQuery(q);
    }
    if (!allowAsk) {
      toast.error(knowledgeErrorMessage(t, "knowledge_corpus_not_ready"));
      return;
    }
    askingRef.current = true;
    askAbortRef.current?.abort();
    const resumingAfterStop = userStoppedRef.current;
    userStoppedRef.current = false;
    const ac = new AbortController();
    askAbortRef.current = ac;
    setAsking(true);
    setActiveCite(null);
    setLiveTurn(createKnowledgeTurn(q));
    const baselineTurnCount =
      useKnowledgeQueryStore.getState().byRoom[roomId]?.turns?.length ?? 0;
    const clientRequestId =
      pendingAskRef.current?.query === q
        ? pendingAskRef.current.clientRequestId
        : newClientRequestId();
    pendingAskRef.current = { query: q, clientRequestId };
    try {
      const res = await api.streamDealRoomKnowledgeSession(
        roomId,
        {
          sessionId: activeSessionId ?? undefined,
          query: q,
          answer: true,
          top_k: 8,
          clientRequestId,
        },
        {
          signal: ac.signal,
          onEvent: (event) => {
            setLiveTurn((prev) => (prev ? reduceKnowledgeStream(prev, event) : prev));
          },
        },
      );
      const current =
        useKnowledgeQueryStore.getState().byRoom[roomId]?.turns ?? [];
      const merged = [...current.filter((x) => x.id !== res.turn.id), res.turn].sort(
        (a, b) => a.sequence - b.sequence,
      );
      const cancelledTurn =
        res.turn.resultStatus === "error" &&
        res.turn.errorSummary === "client_cancelled";
      useKnowledgeQueryStore.getState().setDraft(roomId, {
        activeSessionId: res.sessionId,
        turns: merged,
        activeCite: null,
        sessionState: res.sessionState ?? null,
        ...(cancelledTurn ? {} : { query: "" }),
      });
      setLiveTurn(null);
      pendingAskRef.current = null;
      if (
        res.turn.resultStatus === "error" &&
        res.turn.errorSummary !== "client_cancelled"
      ) {
        toast.error(knowledgeErrorMessage(t, res.turn.errorSummary));
      }
    } catch (e) {
      setLiveTurn(null);
      const stopped =
        ac.signal.aborted || userStoppedRef.current;
      if (stopped) {
        await hydratePendingTurns(baselineTurnCount, q);
        return;
      }
      if (e instanceof ApiError) {
        if (
          e.code === "client_cancelled" ||
          (e.code === "stream_incomplete" && userStoppedRef.current)
        ) {
          await hydratePendingTurns(baselineTurnCount, q);
          return;
        }
        if (
          e.code === "knowledge_query_busy" &&
          pendingAskRef.current?.query === q &&
          resumingAfterStop
        ) {
          const existingTurns =
            useKnowledgeQueryStore.getState().byRoom[roomId]?.turns ?? [];
          if (existingTurns.some((turn) => turn.question === q)) {
            pendingAskRef.current = null;
            return;
          }
          await hydratePendingTurns(baselineTurnCount, q);
          const afterTurns =
            useKnowledgeQueryStore.getState().byRoom[roomId]?.turns ?? [];
          if (
            afterTurns.some((turn) => turn.question === q) ||
            afterTurns.length > baselineTurnCount
          ) {
            pendingAskRef.current = null;
            return;
          }
        }
        toast.error(knowledgeErrorMessage(t, e.code));
        if (isKnowledgeAskGateReject(e.status, e.code)) {
          if (e.code !== "knowledge_query_busy") {
            pendingAskRef.current = null;
          }
          return;
        }
        await hydratePendingTurns(baselineTurnCount, q);
      } else {
        toast.error(t("knowledge.queryFailed"));
        await hydratePendingTurns(baselineTurnCount, q);
      }
    } finally {
      if (askAbortRef.current === ac) askAbortRef.current = null;
      askingRef.current = false;
      setAsking(false);
    }
  };

  const onFeedback = async (
    turnId: string,
    body: { kind: DealRoomKnowledgeFeedbackKind; note?: string },
  ) => {
    try {
      const fb = await api.upsertDealRoomKnowledgeTurnFeedback(roomId, turnId, body);
      const kind = asFeedbackKind(fb.kind) ?? body.kind;
      const next = turns.map((row) =>
        row.id === turnId
          ? { ...row, feedback: { kind, note: fb.note } }
          : row,
      );
      useKnowledgeQueryStore.getState().setDraft(roomId, { turns: next });
    } catch {
      toast.error(t("knowledge.feedback.saveFailed"));
      throw new Error("feedback_failed");
    }
  };

  const citeOpenOutcome = () => resolveCiteOpenOutcome(liveTurn, turns);

  const recordCiteOpen = (
    turnOutcome?: "grounded" | "refused" | "unknown",
  ) => {
    const outcome = turnOutcome ?? citeOpenOutcome();
    void api
      .recordDealRoomKnowledgeDeskEvent(roomId, {
        type: "cite_open",
        turnOutcome: outcome,
      })
      .catch(() => {
        /* metrics must not block navigation */
      });
  };

  return {
    query,
    setQuery,
    activeSessionId,
    turns,
    activeCite,
    setActiveCite,
    sessionState,
    asking,
    liveTurn,
    viewTurns,
    sessionHydrated,
    onAsk,
    onStop,
    onFeedback,
    citeOpenOutcome,
    recordCiteOpen,
  };
}
