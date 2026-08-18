import { ArrowClockwise, CircleNotch, PushPin, Robot, Stop, User } from "@phosphor-icons/react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { AnswerMarkdown } from "@/components/deal-rooms/knowledge/AnswerMarkdown";
import { Button } from "@/components/ui/button";
import { formatDate } from "@/lib/formatters";
import { formatHitLocusLabel } from "@/lib/knowledge/citations";
import {
  shouldShowEvidence,
  type KnowledgeTurn,
} from "@/lib/knowledge/streamEvents";
import { isAskQuotaExceededTurn, isAwaitingHostReply, isFormalUnderReview, isHostQueuedTurn, isPinnedFAQReplayTurn } from "@/lib/visitorAsk/turnModel";
import { cn } from "@/lib/utils";
import type { DealRoomKnowledgeQueryHit, PublicAskTurn } from "@/types";

export interface VisitorAskTurnCardProps {
  turn: PublicAskTurn;
  aiTurn?: KnowledgeTurn | null;
  escalating?: boolean;
  stopped?: boolean;
  onEscalate?: () => void;
  onStopStream?: () => void;
  onRefresh?: () => void;
  refreshing?: boolean;
  onOpenCitation?: (hit: DealRoomKnowledgeQueryHit) => void;
}

export function VisitorAskTurnCard({
  turn,
  aiTurn,
  escalating,
  stopped,
  onEscalate,
  onStopStream,
  onRefresh,
  refreshing,
  onOpenCitation,
}: VisitorAskTurnCardProps) {
  const { t, i18n } = useTranslation("documents");
  const [activeCite, setActiveCite] = useState<number | null>(null);
  const locusFmt = {
    sheetPrefix: t("viewer.askSheetLabel"),
    pageSingle: (page: number) => t("viewer.askPageSingle", { page }),
    pageRange: (from: number, to: number) => t("viewer.askPageRange", { from, to }),
    pageListSep: t("viewer.askPageListSep"),
    pageList: (pages: string) => t("viewer.askPageList", { pages }),
  };

  const showHostAnswer =
    isHostQueuedTurn(turn) &&
    turn.status === "host_answered" &&
    Boolean(turn.host_answer?.trim());

  const streaming =
    aiTurn &&
    aiTurn.phase !== "done" &&
    aiTurn.phase !== "refused" &&
    aiTurn.phase !== "error";

  const showAIBlock = Boolean(aiTurn) && (turn.lane === "ai" || turn.lane === "hybrid");
  const showEvidence = aiTurn ? shouldShowEvidence(aiTurn) : false;
  const showEscalate =
    !isPinnedFAQReplayTurn(turn) &&
    turn.lane === "ai" &&
    turn.status === "ai_refused" &&
    (aiTurn?.phase === "refused" || turn.status === "ai_refused") &&
    onEscalate;
  const showQuotaFallback =
    isAwaitingHostReply(turn) &&
    turn.status === "host_pending" &&
    isAskQuotaExceededTurn(turn);
  const showEscalatedDraft = turn.status === "host_escalated";
  const awaitingHost = isAwaitingHostReply(turn);
  const aiIsHistorical = showHostAnswer || showEscalatedDraft;
  const showAskHostHint =
    (aiTurn?.refused || aiTurn?.phase === "refused") &&
    !showHostAnswer &&
    !awaitingHost;
  const showFormalUnderReview = isFormalUnderReview(turn);
  const scheduledAtRaw =
    showFormalUnderReview && turn.formal_status === "scheduled"
      ? turn.formal_publish_at
      : undefined;
  const scheduledAtMs = scheduledAtRaw ? Date.parse(scheduledAtRaw) : Number.NaN;
  const pendingLabel = showFormalUnderReview
    ? turn.formal_status === "scheduled"
      ? scheduledAtRaw && !Number.isNaN(scheduledAtMs)
        ? t("viewer.askFormalScheduledAt", {
            time: formatDate(scheduledAtRaw, i18n.language),
          })
        : t("viewer.askFormalScheduled")
      : t("viewer.askFormalUnderReview")
    : showEscalatedDraft
      ? t("viewer.askAwaitingHostConfirm")
      : t("viewer.askPendingReply");
  const pendingIsScheduled = turn.formal_status === "scheduled";

  const openHit = (hit: DealRoomKnowledgeQueryHit) => {
    if (!onOpenCitation) return;
    const viewerPage = hit.viewerPage ?? hit.pages?.[0];
    if (!hit.documentId || !viewerPage) return;
    onOpenCitation({ ...hit, viewerPage });
  };

  const handleCite = (n: number) => {
    setActiveCite(n);
    const hit = aiTurn?.results[n - 1];
    if (hit) openHit(hit);
  };

  return (
    <div className="space-y-3">
      <div className="flex justify-end">
        <div className="max-w-[90%] rounded-2xl bg-foreground px-3.5 py-2.5 text-sm text-background shadow-sm">
          <p className="whitespace-pre-wrap break-words">{turn.question}</p>
        </div>
      </div>

      {showAIBlock && aiTurn ? (
        <div className="flex justify-start">
          <div className="max-w-[95%] space-y-2">
            <span className="inline-flex items-center gap-1 rounded-full bg-sky-500/10 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-sky-700 dark:text-sky-300">
              {isPinnedFAQReplayTurn(turn) ? <PushPin size={10} weight="fill" /> : <Robot size={10} weight="fill" />}
              {isPinnedFAQReplayTurn(turn) ? t("viewer.askSourceFaq") : t("viewer.askSourceAI")}
            </span>

            {streaming ? (
              <div className="flex items-center justify-between gap-2">
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <CircleNotch size={14} className="animate-spin" />
                  {aiTurn.phase === "generating"
                    ? t("viewer.askPhaseGenerating")
                    : t("viewer.askPhaseRetrieving")}
                </div>
                {onStopStream ? (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="h-7 rounded-full px-2 text-xs"
                    onClick={onStopStream}
                  >
                    <Stop size={12} weight="fill" />
                    {t("viewer.askStopStream")}
                  </Button>
                ) : null}
              </div>
            ) : null}

            {stopped && !aiTurn.answer ? (
              <p className="text-xs text-muted-foreground">{t("viewer.askStreamStopped")}</p>
            ) : null}

            {aiTurn.phase === "error" ? (
              <p className="text-sm text-destructive">{t("viewer.askAiUnavailable")}</p>
            ) : null}

            {aiTurn.refused || aiTurn.phase === "refused" ? (
              <div
                className={cn(
                  "rounded-2xl border border-amber-500/30 bg-amber-500/5 px-3.5 py-2.5 text-sm text-foreground",
                  aiIsHistorical && "border-dashed opacity-70",
                )}
              >
                <p>{t("viewer.qaNoEvidence")}</p>
                {showAskHostHint ? (
                  <p className="mt-1 text-xs text-muted-foreground">{t("viewer.qaSuggestAskHost")}</p>
                ) : null}
              </div>
            ) : aiTurn.answer ? (
              <div
                className={cn(
                  "rounded-2xl border border-border/60 bg-background/90 px-3.5 py-2.5 text-sm shadow-sm",
                  streaming && "animate-pulse",
                  aiIsHistorical && "border-dashed opacity-70",
                )}
              >
                <AnswerMarkdown
                  answer={aiTurn.answer}
                  activeCite={activeCite ?? undefined}
                  onCite={handleCite}
                />
              </div>
            ) : null}

            {showEvidence ? (
              <div className="rounded-xl border border-border/50 bg-muted/20 p-2.5">
                <p className="mb-2 text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">
                  {t("viewer.askEvidenceTitle", { count: aiTurn.results.length })}
                </p>
                <ul className="space-y-2">
                  {aiTurn.results.map((hit, idx) => {
                    const locus = formatHitLocusLabel(hit, locusFmt);
                    const citeNum = idx + 1;
                    const viewerPage = hit.viewerPage ?? hit.pages?.[0];
                    const canJump = Boolean(onOpenCitation && hit.documentId && viewerPage);
                    return (
                      <li
                        key={hit.chunkId || `${hit.documentId}-${idx}`}
                        className={cn(
                          "rounded-lg border border-border/40 bg-background/80 px-2.5 py-2 text-xs transition-colors",
                          activeCite === citeNum && "border-foreground/30 shadow-sm",
                        )}
                      >
                        {locus ? (
                          <p className="mb-1 font-medium text-foreground">{locus}</p>
                        ) : null}
                        <p className="line-clamp-3 text-muted-foreground">{hit.text}</p>
                        {canJump ? (
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            className="mt-2 h-7 rounded-full text-[11px]"
                            onClick={() => {
                              setActiveCite(citeNum);
                              openHit(hit);
                            }}
                          >
                            {t("viewer.openPage", { pageNumber: viewerPage })}
                          </Button>
                        ) : null}
                      </li>
                    );
                  })}
                </ul>
              </div>
            ) : null}

            {showEscalate ? (
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="rounded-full text-xs"
                disabled={escalating}
                onClick={onEscalate}
              >
                {escalating ? t("viewer.askEscalating") : t("viewer.askEscalateToHost")}
              </Button>
            ) : null}
          </div>
        </div>
      ) : null}

      {awaitingHost ? (
        <div className="space-y-1 text-center">
          {showQuotaFallback ? (
            <p className="text-xs text-amber-700 dark:text-amber-300">
              {t("viewer.askAiQuotaExceededVisitor")}
            </p>
          ) : null}
          {onRefresh ? (
            <Button
              type="button"
              variant="ghost"
              className={cn(
                "h-auto gap-1.5 px-2 py-1 font-medium text-muted-foreground hover:text-foreground",
                pendingIsScheduled
                  ? "max-w-[22rem] whitespace-normal text-xs font-normal normal-case tracking-normal"
                  : "text-[10px] uppercase tracking-wide",
              )}
              onClick={onRefresh}
              disabled={refreshing}
              aria-label={pendingLabel}
              data-testid="visitor-ask-refresh"
            >
              <ArrowClockwise
                size={12}
                className={cn("shrink-0", refreshing ? "animate-spin" : undefined)}
              />
              <span className="text-left leading-snug">{pendingLabel}</span>
            </Button>
          ) : (
            <p
              className={cn(
                "font-medium text-muted-foreground",
                pendingIsScheduled
                  ? "text-xs font-normal normal-case tracking-normal"
                  : "text-[10px] uppercase tracking-wide",
              )}
            >
              {pendingLabel}
            </p>
          )}
        </div>
      ) : null}

      {showHostAnswer ? (
        <div className="flex justify-start">
          <div className="max-w-[90%] space-y-1">
            <span className="inline-flex items-center gap-1 rounded-full bg-emerald-500/10 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-emerald-700 dark:text-emerald-300">
              {isPinnedFAQReplayTurn(turn) ? <PushPin size={10} weight="fill" /> : <User size={10} />}
              {isPinnedFAQReplayTurn(turn) ? t("viewer.askSourceFaq") : t("viewer.askSourceOwner")}
            </span>
            <div className="rounded-2xl border border-border/60 bg-background/90 px-3.5 py-2.5 text-sm shadow-sm">
              <p className="whitespace-pre-wrap break-words">{turn.host_answer}</p>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
