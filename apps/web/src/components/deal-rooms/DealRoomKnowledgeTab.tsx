import { useEffect, useMemo, useRef, useState } from "react";
import { ArrowLeft, LockKey, SealCheck } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  CorpusIntegrityRail,
  resolveCorpusAttentionStage,
  type KnowledgeRoomMetrics,
} from "@/components/deal-rooms/knowledge/CorpusIntegrityRail";
import { GroundedChatShell } from "@/components/deal-rooms/knowledge/GroundedChatShell";
import { KnowledgeAskEntryCard } from "@/components/deal-rooms/knowledge/KnowledgeAskEntryCard";
import { KnowledgeSessionHistoryMenu } from "@/components/deal-rooms/knowledge/KnowledgeSessionHistoryMenu";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import { useAsyncData } from "@/hooks/useAsyncData";
import {
  formatHitLocusLabel,
  formatPagesLabel,
  renderAnswerWithCitations,
  viewerPath,
} from "@/lib/knowledge/citations";
import { knowledgeErrorMessage } from "@/lib/knowledge/errors";
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

interface DealRoomKnowledgeTabProps {
  roomId: string;
}

const EMPTY_TURNS: DealRoomKnowledgeQATurn[] = [];

export { knowledgeErrorMessage };

function isKnowledgeBusy(status?: string, jobStatus?: string) {
  if (status === "syncing" || status === "provisioning") return true;
  return jobStatus === "pending" || jobStatus === "running";
}

/** docling-rag grounded-answer refusals — retrieval may still return low-score hits. */
export function isUngroundedKnowledgeAnswer(answer?: string | null): boolean {
  const text = (answer ?? "").trim().toLowerCase();
  if (!text) return false;
  const needles = [
    "does not contain an answer",
    "do not contain an answer",
    "no relevant information",
    "cannot answer based on the",
    "can't answer based on the",
    "未找到相关",
    "没有匹配",
    "无法从提供的",
    "上下文中没有",
    "资料中没有",
  ];
  return needles.some((n) => text.includes(n));
}

// Re-export citation helpers for existing tests / callers.
export {
  formatHitLocusLabel,
  formatPagesLabel,
  renderAnswerWithCitations,
  viewerPath,
};

export function DealRoomKnowledgeTab({ roomId }: DealRoomKnowledgeTabProps) {
  const { t } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");
  const navigate = useNavigate();
  const { data, loading, error, refetch } = useAsyncData(
    () => api.getDealRoomKnowledge(roomId),
    [roomId],
  );
  const { data: roomMetrics } = useAsyncData(async () => {
    const [analytics, questionsRes, linksRes] = await Promise.all([
      api.getDealRoomAnalytics(roomId),
      api.listRoomQuestions(roomId),
      api.getDealRoomLinks(roomId, { page_size: 100 }),
    ]);
    const questions = questionsRes.data ?? [];
    const links = linksRes.data ?? [];
    const askKeys = new Set<string>();
    for (const q of questions) {
      const key = (q.visitor_id || q.visitor_email || "").trim();
      if (key) askKeys.add(key);
    }
    const visitedLinkIds = new Set(
      links.filter((l) => (l.accessCount ?? 0) > 0).map((l) => l.id),
    );
    return {
      documentCount: analytics.documentCount,
      askUniqueVisitors: askKeys.size,
      visitedLinkCount: visitedLinkIds.size,
    } satisfies KnowledgeRoomMetrics;
  }, [roomId]);

  const [syncing, setSyncing] = useState(false);
  const [asking, setAsking] = useState(false);
  const [sessionHydrated, setSessionHydrated] = useState(false);
  const [liveTurn, setLiveTurn] = useState<KnowledgeTurn | null>(null);
  const askAbortRef = useRef<AbortController | null>(null);
  /** Sync guard — React state `asking` alone cannot block rapid double Ask/Enter. */
  const askingRef = useRef(false);

  // Single store snapshot — avoid `?? []` selectors that allocate each read.
  const draft = useKnowledgeQueryStore((s) => s.byRoom[roomId]);
  const query = draft?.query ?? "";
  const activeSessionId = draft?.activeSessionId ?? null;
  const turns = draft?.turns ?? EMPTY_TURNS;
  const activeCite = draft?.activeCite ?? null;

  const openViewer = (documentId: string, page?: number) => {
    // Prefer in-flight live turn; else last persisted audit turn.
    let turnOutcome: "grounded" | "refused" | "unknown" = "unknown";
    if (liveTurn) {
      turnOutcome = liveTurn.refused
        ? "refused"
        : liveTurn.results.length > 0
          ? "grounded"
          : "unknown";
    } else {
      const last = turns[turns.length - 1];
      if (last?.refused) turnOutcome = "refused";
      else if ((last?.hits?.length ?? 0) > 0) turnOutcome = "grounded";
    }
    void api
      .recordDealRoomKnowledgeDeskEvent(roomId, { type: "cite_open", turnOutcome })
      .catch(() => {
        /* metrics must not block navigation */
      });
    navigate(viewerPath(documentId, page));
  };

  const setQuery = (value: string) =>
    useKnowledgeQueryStore.getState().setDraft(roomId, { query: value });
  const setActiveCite = (value: number | null) =>
    useKnowledgeQueryStore.getState().setDraft(roomId, { activeCite: value });

  // Landing shows corpus + ask-entry; restore desk when store already has turns
  // (viewer → Back) or after active-session hydrate.
  const [chatOpen, setChatOpen] = useState(
    () => (useKnowledgeQueryStore.getState().byRoom[roomId]?.turns.length ?? 0) > 0,
  );

  const viewTurns = useMemo(() => {
    const lastIdx = turns.length - 1;
    const mapped = turns.map((row, idx) =>
      turnFromQATurn(row, idx === lastIdx && !liveTurn ? activeCite : null),
    );
    return liveTurn ? [...mapped, liveTurn] : mapped;
  }, [turns, activeCite, liveTurn]);

  useEffect(() => {
    if (!isKnowledgeBusy(data?.status, data?.progress?.jobStatus)) return;
    const timer = window.setInterval(() => {
      void refetch();
    }, 2500);
    return () => window.clearInterval(timer);
  }, [data?.status, data?.progress?.jobStatus, refetch]);

  // Hydrate newest active session from server (refresh recovery).
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
          });
          setChatOpen(true);
        }
      } catch {
        // Non-fatal: desk still works for a fresh session on first ask.
      } finally {
        if (!cancelled) setSessionHydrated(true);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [roomId]);

  const onSync = async () => {
    setSyncing(true);
    try {
      await api.syncDealRoomKnowledge(roomId);
      toast.success(t("knowledge.syncQueued"));
      await refetch();
    } catch (e) {
      if (e instanceof ApiError && e.status === 503) {
        toast.error(t("knowledge.unavailable"));
      } else {
        toast.error(t("knowledge.syncFailed"));
      }
    } finally {
      setSyncing(false);
    }
  };

  const onStopAsk = () => {
    askAbortRef.current?.abort();
  };

  /** Poll active session until turn count grows past baseline (server may still be writing). */
  const hydrateAfterAbort = async (baselineTurnCount: number) => {
    const deadline = Date.now() + 4000;
    let delayMs = 120;
    while (Date.now() < deadline) {
      try {
        const detail = await api.getActiveDealRoomKnowledgeSession(roomId);
        const serverTurns = detail.turns ?? [];
        const sessionId = detail.session?.id ?? null;
        if (sessionId && serverTurns.length > baselineTurnCount) {
          useKnowledgeQueryStore.getState().setDraft(roomId, {
            activeSessionId: sessionId,
            turns: serverTurns,
            activeCite: null,
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

  const onQuery = async () => {
    const q = query.trim();
    if (!q || askingRef.current) return;
    askingRef.current = true;
    askAbortRef.current?.abort();
    const ac = new AbortController();
    askAbortRef.current = ac;
    setAsking(true);
    setActiveCite(null);
    setLiveTurn(createKnowledgeTurn(q));
    const baselineTurnCount =
      useKnowledgeQueryStore.getState().byRoom[roomId]?.turns?.length ?? 0;
    try {
      const res = await api.streamDealRoomKnowledgeSession(
        roomId,
        {
          sessionId: activeSessionId ?? undefined,
          query: q,
          answer: true,
          top_k: 8,
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
      useKnowledgeQueryStore.getState().setDraft(roomId, {
        activeSessionId: res.sessionId,
        turns: merged,
        query: "",
        activeCite: null,
      });
      // Clear optimistic stream turn only after audit row is in the store.
      setLiveTurn(null);
      if (res.turn.resultStatus === "error") {
        toast.error(knowledgeErrorMessage(t, res.turn.errorSummary));
      }
    } catch (e) {
      if (ac.signal.aborted) {
        // Server may still finish QueryWithSession and persist the turn — hydrate so
        // the audit timeline does not lose a completed answer after Stop (P5).
        setLiveTurn(null);
        await hydrateAfterAbort(baselineTurnCount);
        return;
      }
      if (e instanceof ApiError) {
        toast.error(knowledgeErrorMessage(t, e.code));
      } else {
        toast.error(t("knowledge.queryFailed"));
      }
      setLiveTurn(null);
    } finally {
      if (askAbortRef.current === ac) askAbortRef.current = null;
      askingRef.current = false;
      setAsking(false);
    }
  };

  const onNewSession = async () => {
    if (activeSessionId) {
      try {
        await api.closeDealRoomKnowledgeSession(roomId, activeSessionId);
      } catch {
        toast.error(t("knowledge.sessionCloseFailed"));
        return;
      }
    }
    useKnowledgeQueryStore.getState().setDraft(roomId, {
      activeSessionId: null,
      turns: [],
      query: "",
      activeCite: null,
    });
  };

  const onOpenSession = async (sessionId: string) => {
    const detail = await api.getDealRoomKnowledgeSession(roomId, sessionId);
    const serverTurns = detail.turns ?? [];
    const id = detail.session?.id ?? sessionId;
    useKnowledgeQueryStore.getState().setDraft(roomId, {
      activeSessionId: id,
      turns: serverTurns,
      query: "",
      activeCite: null,
    });
    setChatOpen(true);
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

  if (loading && !data) {
    return (
      <div className="rounded-xl border border-border/80 bg-muted/20 px-4 py-14 text-center text-sm text-muted-foreground">
        {tc("loading")}
      </div>
    );
  }

  if (error && !data) {
    return (
      <div
        className="rounded-xl border border-destructive/25 bg-destructive/[0.04] px-4 py-8 text-center"
        role="alert"
      >
        <p className="text-sm text-destructive">{t("knowledge.loadFailed")}</p>
        <Button size="sm" variant="outline" className="mt-3" onClick={() => void refetch()}>
          {tc("retry")}
        </Button>
      </div>
    );
  }

  const corpus = data!;
  if (!corpus.enabled) {
    return (
      <div
        className="relative overflow-hidden rounded-2xl border border-border/70 bg-[linear-gradient(180deg,#f8fafc_0%,#ffffff_55%)] px-6 py-16 text-center"
        data-testid="deal-room-knowledge-tab"
      >
        <div
          aria-hidden
          className="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-foreground/15 to-transparent"
        />
        <span className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl border border-border/80 bg-background text-foreground shadow-[0_1px_0_rgba(15,23,42,0.04)]">
          <LockKey size={22} weight="duotone" />
        </span>
        <div className="mx-auto mt-4 max-w-md space-y-2">
          <p className="text-[15px] font-semibold tracking-tight text-foreground">
            {t("knowledge.disabledTitle")}
          </p>
          <p className="text-sm leading-relaxed text-muted-foreground">
            {t("knowledge.disabledDescription")}
          </p>
        </div>
      </div>
    );
  }

  const metrics: KnowledgeRoomMetrics = {
    documentCount: roomMetrics?.documentCount ?? corpus.documents.length,
    askUniqueVisitors: roomMetrics?.askUniqueVisitors ?? 0,
    visitedLinkCount: roomMetrics?.visitedLinkCount ?? 0,
  };
  const corpusReady = resolveCorpusAttentionStage(corpus) === "ready";

  if (!chatOpen) {
    return (
      <div className="relative space-y-6" data-testid="deal-room-knowledge-tab">
        <div className="grid max-w-4xl gap-4 sm:grid-cols-2">
          <CorpusIntegrityRail
            corpus={corpus}
            metrics={metrics}
            syncing={syncing}
            onSync={() => {
              void onSync();
            }}
          />
          <KnowledgeAskEntryCard
            ready={corpusReady && sessionHydrated}
            onStartAsk={() => setChatOpen(true)}
          />
        </div>
        <div className="flex max-w-4xl justify-end">
          <KnowledgeSessionHistoryMenu
            roomId={roomId}
            activeSessionId={activeSessionId}
            onOpenSession={onOpenSession}
          />
        </div>
      </div>
    );
  }

  return (
    <div className="relative space-y-6" data-testid="deal-room-knowledge-tab">
      <section
        className="relative overflow-hidden rounded-2xl border border-border/70 bg-[linear-gradient(165deg,#f8fafc_0%,#ffffff_42%,#ffffff_100%)]"
        data-testid="deal-room-knowledge-desk"
      >
        <div
          aria-hidden
          className="pointer-events-none absolute -right-16 -top-20 h-56 w-56 rounded-full bg-[radial-gradient(circle_at_center,rgba(15,23,42,0.06),transparent_68%)]"
        />
        <div
          aria-hidden
          className="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-foreground/12 to-transparent"
        />

        <div className="relative space-y-5 px-5 py-6 sm:px-7 sm:py-7">
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div className="min-w-0 max-w-2xl space-y-2.5">
              <p className="font-mono text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground">
                {t("knowledge.heroEyebrow")}
              </p>
              <h2 className="text-[1.65rem] font-semibold tracking-tight text-foreground sm:text-[1.85rem]">
                {t("knowledge.heroTitle")}
              </h2>
              <p className="max-w-xl text-sm leading-relaxed text-muted-foreground">
                {t("knowledge.heroDescription")}
              </p>
            </div>
            <div className="flex flex-wrap items-center justify-end gap-2">
              <button
                type="button"
                onClick={() => setChatOpen(false)}
                data-testid="deal-room-knowledge-back-to-corpus"
                className="inline-flex items-center gap-1.5 rounded-full border border-border/80 bg-background/80 px-2.5 py-1 text-[11px] font-medium text-foreground/80 backdrop-blur-sm transition-colors hover:bg-muted/50 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                <ArrowLeft size={12} weight="bold" className="text-foreground/55" />
                {t("knowledge.backToCorpus")}
              </button>
              {turns.length > 0 ? (
                <span
                  className="inline-flex items-center gap-1.5 rounded-full border border-border/80 bg-background/80 px-2.5 py-1 text-[11px] font-medium text-foreground/80 backdrop-blur-sm"
                  data-testid="deal-room-knowledge-session-meta"
                >
                  {t("knowledge.sessionTurns", { count: turns.length })}
                </span>
              ) : null}
              <KnowledgeSessionHistoryMenu
                roomId={roomId}
                activeSessionId={activeSessionId}
                onOpenSession={onOpenSession}
              />
              <button
                type="button"
                onClick={() => {
                  void onNewSession();
                }}
                data-testid="deal-room-knowledge-new-session"
                className="inline-flex items-center gap-1.5 rounded-full border border-border/80 bg-background/80 px-2.5 py-1 text-[11px] font-medium text-foreground/80 backdrop-blur-sm transition-colors hover:bg-muted/50 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                {t("knowledge.newSession")}
              </button>
              <span className="inline-flex items-center gap-1.5 rounded-full border border-border/80 bg-background/80 px-2.5 py-1 text-[11px] font-medium text-foreground/80 backdrop-blur-sm">
                <LockKey size={12} weight="bold" className="text-foreground/55" />
                {t("knowledge.trustScoped")}
              </span>
              <span className="inline-flex items-center gap-1.5 rounded-full border border-border/80 bg-background/80 px-2.5 py-1 text-[11px] font-medium text-foreground/80 backdrop-blur-sm">
                <SealCheck size={12} weight="bold" className="text-foreground/55" />
                {t("knowledge.trustGrounded")}
              </span>
            </div>
          </div>

          <GroundedChatShell
            query={query}
            onQueryChange={setQuery}
            turns={viewTurns}
            asking={asking}
            onAsk={() => {
              void onQuery();
            }}
            onStop={asking ? onStopAsk : undefined}
            onActiveCite={setActiveCite}
            onOpenViewer={openViewer}
            onFeedback={onFeedback}
          />
        </div>
      </section>
    </div>
  );
}
