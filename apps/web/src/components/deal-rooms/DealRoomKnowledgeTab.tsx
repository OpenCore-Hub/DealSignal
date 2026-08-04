import { useEffect, useState } from "react";
import { ArrowLeft, LockKey, SealCheck } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  CorpusIntegrityRail,
  resolveCorpusAttentionStage,
  type KnowledgeRoomMetrics,
} from "@/components/deal-rooms/knowledge/CorpusIntegrityRail";
import { GroundedChatShell } from "@/components/deal-rooms/knowledge/GroundedChatShell";
import { KnowledgeAskEntryCard } from "@/components/deal-rooms/knowledge/KnowledgeAskEntryCard";
import { KnowledgeOpsStrip } from "@/components/deal-rooms/knowledge/KnowledgeOpsStrip";
import { KnowledgeSessionHistoryMenu } from "@/components/deal-rooms/knowledge/KnowledgeSessionHistoryMenu";
import { ColdArchivePanel } from "@/components/deal-rooms/knowledge/ColdArchivePanel";
import { EvalGoldReviewPanel } from "@/components/deal-rooms/knowledge/EvalGoldReviewPanel";
import { MissionProgressRail } from "@/components/deal-rooms/knowledge/MissionProgressRail";
import { SessionStateRail } from "@/components/deal-rooms/knowledge/SessionStateRail";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import { useAsyncData } from "@/hooks/useAsyncData";
import { useKnowledgeDeskSession } from "@/hooks/useKnowledgeDeskSession";
import {
  formatHitLocusLabel,
  formatPagesLabel,
  renderAnswerWithCitations,
  viewerPath,
} from "@/lib/knowledge/citations";
import { downloadDiligencePack } from "@/lib/knowledge/downloadDiligence";
import { knowledgeErrorMessage } from "@/lib/knowledge/errors";
import { useKnowledgeFollowUpChips } from "@/lib/knowledge/useKnowledgeFollowUpChips";
import { useKnowledgeQueryStore } from "@/stores/knowledgeQueryStore";

interface DealRoomKnowledgeTabProps {
  roomId: string;
}

export { knowledgeErrorMessage };

function isKnowledgeBusy(status?: string, jobStatus?: string) {
  if (status === "syncing" || status === "provisioning") return true;
  return jobStatus === "pending" || jobStatus === "running";
}

/** docling-rag grounded-answer refusals — retrieval may still return low-score hits. */
/** @deprecated Prefer `@/lib/knowledge/trustGates` — kept for existing test imports. */
export { isUngroundedKnowledgeAnswer } from "@/lib/knowledge/trustGates";

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
  const { workspaceSlug } = useParams<{ workspaceSlug?: string }>();
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
  /** Bumps ops strip + gold review queue after accept/reject. */
  const [opsRefreshKey, setOpsRefreshKey] = useState(0);
  /** Desk chrome: SessionStateRail stays opt-in; MissionProgressRail auto-opens on empty desk. */
  const [sessionStateOpen, setSessionStateOpen] = useState(false);
  const [missionProgressOpen, setMissionProgressOpen] = useState(
    () => (useKnowledgeQueryStore.getState().byRoom[roomId]?.turns.length ?? 0) === 0,
  );

  // Landing shows corpus + ask-entry; restore desk when store already has turns
  // (viewer → Back) or after active-session hydrate.
  const [chatOpen, setChatOpen] = useState(
    () => (useKnowledgeQueryStore.getState().byRoom[roomId]?.turns.length ?? 0) > 0,
  );

  const corpusAllowAsk =
    !!data?.enabled && resolveCorpusAttentionStage(data) === "ready";

  const {
    query,
    setQuery,
    activeSessionId,
    turns,
    setActiveCite,
    sessionState,
    asking,
    liveTurn,
    viewTurns,
    sessionHydrated,
    onAsk,
    onStop,
    onFeedback,
    recordCiteOpen,
  } = useKnowledgeDeskSession(roomId, {
    allowAsk: corpusAllowAsk,
    onActiveSessionHydrated: () => setChatOpen(true),
  });

  // First visit / new session → show follow-up templates; once Q&A is active → hide.
  const deskHasQaActivity = turns.length > 0 || asking || !!liveTurn;
  useEffect(() => {
    setMissionProgressOpen(!deskHasQaActivity);
  }, [deskHasQaActivity]);

  const openViewer = (documentId: string, page?: number) => {
    recordCiteOpen();
    navigate(
      viewerPath(documentId, page, {
        roomId,
        workspaceSlug: (workspaceSlug || "").trim() || undefined,
      }),
    );
  };

  const lastFollowUpTurn = turns.length > 0 ? turns[turns.length - 1]! : null;
  const {
    chips: followUpChips,
    source: followUpSource,
    upgrading: followUpUpgrading,
    setEngaged: setFollowUpsEngaged,
  } = useKnowledgeFollowUpChips(roomId, lastFollowUpTurn, t);

  useEffect(() => {
    if (!isKnowledgeBusy(data?.status, data?.progress?.jobStatus)) return;
    const timer = window.setInterval(() => {
      void refetch();
    }, 2500);
    return () => window.clearInterval(timer);
  }, [data?.status, data?.progress?.jobStatus, refetch]);

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
      sessionState: null,
    });
    setMissionProgressOpen(true);
  };

  const onExportSession = async () => {
    if (!activeSessionId) return;
    try {
      const pack = await api.exportDealRoomKnowledgeSession(roomId, activeSessionId);
      downloadDiligencePack(pack);
      toast.success(t("knowledge.exportSuccess"));
    } catch {
      toast.error(t("knowledge.exportFailed"));
    }
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
      sessionState: detail.session?.state ?? null,
    });
    setChatOpen(true);
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
        <KnowledgeOpsStrip
          roomId={roomId}
          refreshKey={opsRefreshKey}
          className="max-w-4xl"
        />
        <EvalGoldReviewPanel
          roomId={roomId}
          refreshKey={opsRefreshKey}
          className="max-w-4xl"
          onReviewed={() => setOpsRefreshKey((k) => k + 1)}
        />
        <ColdArchivePanel
          roomId={roomId}
          refreshKey={opsRefreshKey}
          className="max-w-4xl"
        />
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
    // Fill remaining viewport under TopNav + page header/tabs; bleed into AppShell
    // main bottom padding (p-6 / md:p-8) so the composer sits flush with the window.
    <div
      className="relative -mb-6 flex min-h-[calc(100dvh-12rem)] flex-col md:-mb-8"
      data-testid="deal-room-knowledge-tab"
    >
      <section
        className="relative isolate flex min-h-0 flex-1 flex-col rounded-2xl border border-border/70 bg-[linear-gradient(165deg,#f8fafc_0%,#ffffff_42%,#ffffff_100%)]"
        data-testid="deal-room-knowledge-desk"
      >
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 overflow-hidden rounded-2xl"
        >
          <div className="absolute -right-16 -top-20 h-56 w-56 rounded-full bg-[radial-gradient(circle_at_center,rgba(15,23,42,0.06),transparent_68%)]" />
          <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-foreground/12 to-transparent" />
        </div>

        <div className="relative flex min-h-0 flex-1 flex-col px-5 pt-6 sm:px-7 sm:pt-7">
          <div className="flex shrink-0 flex-wrap items-center justify-end gap-2 pb-5">
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
            {activeSessionId && turns.length > 0 ? (
              <button
                type="button"
                onClick={() => {
                  void onExportSession();
                }}
                data-testid="deal-room-knowledge-export"
                className="inline-flex items-center gap-1.5 rounded-full border border-border/80 bg-background/80 px-2.5 py-1 text-[11px] font-medium text-foreground/80 backdrop-blur-sm transition-colors hover:bg-muted/50 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                {t("knowledge.exportDiligence")}
              </button>
            ) : null}
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
            <button
              type="button"
              onClick={() => setSessionStateOpen((v) => !v)}
              aria-pressed={sessionStateOpen}
              data-testid="deal-room-knowledge-session-state-toggle"
              className={
                sessionStateOpen
                  ? "inline-flex items-center gap-1.5 rounded-full border border-foreground/20 bg-foreground/[0.06] px-2.5 py-1 text-[11px] font-medium text-foreground backdrop-blur-sm transition-colors hover:bg-foreground/[0.09] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  : "inline-flex items-center gap-1.5 rounded-full border border-border/80 bg-background/80 px-2.5 py-1 text-[11px] font-medium text-foreground/80 backdrop-blur-sm transition-colors hover:bg-muted/50 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              }
            >
              <LockKey size={12} weight="bold" className="text-foreground/55" />
              {t("knowledge.trustScoped")}
            </button>
            <button
              type="button"
              onClick={() => setMissionProgressOpen((v) => !v)}
              aria-pressed={missionProgressOpen}
              data-testid="deal-room-knowledge-mission-progress-toggle"
              className={
                missionProgressOpen
                  ? "inline-flex items-center gap-1.5 rounded-full border border-foreground/20 bg-foreground/[0.06] px-2.5 py-1 text-[11px] font-medium text-foreground backdrop-blur-sm transition-colors hover:bg-foreground/[0.09] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  : "inline-flex items-center gap-1.5 rounded-full border border-border/80 bg-background/80 px-2.5 py-1 text-[11px] font-medium text-foreground/80 backdrop-blur-sm transition-colors hover:bg-muted/50 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              }
            >
              <SealCheck size={12} weight="bold" className="text-foreground/55" />
              {t("knowledge.trustGrounded")}
            </button>
          </div>

          <GroundedChatShell
            layout="desk"
            beforeTimeline={
              <>
                {sessionStateOpen ? (
                  <SessionStateRail
                    state={sessionState}
                    onAskOpenQuestion={(text) => {
                      void onAsk(text);
                    }}
                  />
                ) : null}
                {missionProgressOpen ? (
                  <MissionProgressRail
                    roomId={roomId}
                    sessionId={activeSessionId}
                    refreshKey={turns.length}
                    onAskItem={(text) => {
                      void onAsk(text);
                    }}
                  />
                ) : null}
              </>
            }
            query={query}
            onQueryChange={setQuery}
            turns={viewTurns}
            asking={asking}
            askEnabled={corpusReady}
            onAsk={(overrideQuery) => {
              void onAsk(overrideQuery);
            }}
            onStop={asking ? onStop : undefined}
            onActiveCite={setActiveCite}
            onOpenViewer={openViewer}
            onFeedback={onFeedback}
            followUpChips={followUpChips}
            followUpSource={followUpSource}
            followUpUpgrading={followUpUpgrading}
            onFollowUpsEngagedChange={setFollowUpsEngaged}
          />
        </div>
      </section>
    </div>
  );
}
