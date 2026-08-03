import {
  ArrowRight,
  CircleNotch,
  MagnifyingGlass,
  SealCheck,
  Stop,
} from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  formatHitLocusLabel,
  renderAnswerWithCitations,
  type LocusFormatters,
} from "@/lib/knowledge/citations";
import { TurnFeedbackControls } from "@/components/deal-rooms/knowledge/TurnFeedbackControls";
import { knowledgeErrorMessage } from "@/lib/knowledge/errors";
import { buildRoomFollowUps } from "@/lib/knowledge/followUps";
import type { KnowledgeTurn } from "@/lib/knowledge/streamEvents";
import { shouldShowEvidence } from "@/lib/knowledge/streamEvents";
import { cn } from "@/lib/utils";
import type { DealRoomKnowledgeFeedbackKind } from "@/types";

export interface GroundedChatShellProps {
  /** Current composer value (controlled by room store). */
  query: string;
  onQueryChange: (value: string) => void;
  /** Ordered audit turns (oldest → newest). Empty = composer only. */
  turns: KnowledgeTurn[];
  asking: boolean;
  onAsk: () => void;
  /** Optional stop for AbortController-backed streams. */
  onStop?: () => void;
  onActiveCite: (n: number | null) => void;
  onOpenViewer: (documentId: string, page?: number) => void;
  onFeedback?: (
    turnId: string,
    body: { kind: DealRoomKnowledgeFeedbackKind; note?: string },
  ) => Promise<void>;
  className?: string;
}

function isTerminalTurn(turn: KnowledgeTurn): boolean {
  return turn.phase === "done" || turn.phase === "refused" || turn.phase === "error";
}

/**
 * Research-desk shell: scrollable Turn timeline above, fixed follow-ups + composer dock.
 * Philosophy: trust > fluency; evidence first-class; refuse hides rail.
 */
export function GroundedChatShell({
  query,
  onQueryChange,
  turns,
  asking,
  onAsk,
  onStop,
  onActiveCite,
  onOpenViewer,
  onFeedback,
  className,
}: GroundedChatShellProps) {
  const { t } = useTranslation("dealRooms");
  const locusFmt: LocusFormatters = {
    sheetPrefix: t("knowledge.sheetLabel"),
    pageSingle: (page) => t("knowledge.pageSingle", { page }),
    pageRange: (from, to) => t("knowledge.pageRange", { from, to }),
    pageListSep: t("knowledge.pageListSep"),
    pageList: (pages) => t("knowledge.pageList", { pages }),
  };

  // Spec §8.3: show when ≥1 turn exists — use latest terminal turn (keep chips while asking).
  const lastTerminal = [...turns].reverse().find(isTerminalTurn);
  const followUps =
    turns.length > 0 && lastTerminal
      ? buildRoomFollowUps({
          refused: lastTerminal.refused,
          resultStatus: lastTerminal.resultStatus,
          hits: lastTerminal.results,
        })
      : [];

  return (
    <div
      className={cn(
        "flex max-h-[min(70vh,44rem)] min-h-[18rem] flex-col",
        className,
      )}
      data-testid="grounded-chat-shell"
    >
      <div
        className="min-h-0 flex-1 space-y-5 overflow-y-auto overscroll-contain pr-0.5"
        data-testid="grounded-chat-timeline"
      >
        {turns.length > 0 ? (
          turns.map((turn) => {
            const showEvidence = shouldShowEvidence(turn);
            const phaseHint =
              turn.phase === "retrieving"
                ? t("knowledge.phaseRetrieving")
                : turn.phase === "generating"
                  ? t("knowledge.phaseGenerating")
                  : null;
            return (
              <section
                key={turn.id}
                className="space-y-3"
                data-testid="grounded-chat-turn"
              >
                <p className="px-1 text-sm font-medium text-foreground/85">
                  <span className="mr-2 font-mono text-[11px] uppercase tracking-[0.12em] text-muted-foreground">
                    {t("knowledge.turnQuestion")}
                  </span>
                  {turn.query}
                </p>
                <div
                  className={cn(
                    "grid gap-4",
                    showEvidence && "lg:grid-cols-[minmax(0,1.15fr)_minmax(0,0.85fr)]",
                  )}
                >
                  <div className="rounded-2xl border border-border/70 bg-background">
                    <div className="flex items-center gap-2 border-b border-border/60 px-5 py-3.5">
                      <span className="flex h-7 w-7 items-center justify-center rounded-lg bg-foreground/[0.04] text-foreground">
                        <SealCheck size={15} weight="duotone" />
                      </span>
                      <div>
                        <p className="text-sm font-semibold tracking-tight">
                          {t("knowledge.answerLabel")}
                        </p>
                        <p className="text-[11px] text-muted-foreground">
                          {phaseHint ?? t("knowledge.answerHint")}
                        </p>
                      </div>
                    </div>
                    <div className="px-5 py-5">
                      {turn.phase === "error" ? (
                        <p className="text-sm text-destructive" role="alert">
                          {knowledgeErrorMessage(t, turn.errorMessage)}
                        </p>
                      ) : turn.answer ? (
                        <div className="text-[15px] leading-[1.7] text-foreground/90 whitespace-pre-wrap">
                          {renderAnswerWithCitations(turn.answer, (n) => onActiveCite(n))}
                          {turn.phase === "generating" ? (
                            <span className="ml-0.5 inline-block h-4 w-0.5 animate-pulse bg-foreground/50 align-middle" />
                          ) : null}
                        </div>
                      ) : turn.phase === "retrieving" || turn.phase === "generating" ? (
                        <p className="text-sm text-muted-foreground">{phaseHint}</p>
                      ) : (
                        <p className="text-sm text-muted-foreground">{t("knowledge.noHits")}</p>
                      )}
                    </div>
                  </div>

                  {showEvidence ? (
                    <div className="rounded-2xl border border-border/70 bg-muted/[0.25]">
                      <div className="flex items-center justify-between gap-2 border-b border-border/50 px-5 py-3.5">
                        <p className="text-sm font-semibold tracking-tight">
                          {t("knowledge.sourcesTitle")}
                        </p>
                        <span className="font-mono text-[11px] text-muted-foreground">
                          {t("knowledge.sourcesCount", { count: turn.results.length })}
                        </span>
                      </div>
                      <ul className="space-y-2 px-3 py-3">
                        {turn.results.map((hit, idx) => {
                          const n = idx + 1;
                          const locus = formatHitLocusLabel(hit, locusFmt);
                          const canJump = !!(hit.documentId && hit.viewerPage);
                          const canOpenDoc = !!(hit.documentId && !hit.viewerPage);
                          return (
                            <li
                              key={hit.chunkId || `${turn.id}-${idx}`}
                              className={cn(
                                "rounded-xl border bg-background px-3.5 py-3 text-sm transition-colors",
                                turn.activeCite === n
                                  ? "border-foreground/25 shadow-[0_0_0_1px_rgba(15,23,42,0.06)]"
                                  : "border-border/60 hover:border-border",
                              )}
                              data-testid="deal-room-knowledge-hit"
                            >
                              <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                                <button
                                  type="button"
                                  className="inline-flex h-5 min-w-5 items-center justify-center rounded-sm bg-foreground/[0.06] px-1.5 font-mono text-[11px] font-semibold"
                                  onClick={() => onActiveCite(n)}
                                >
                                  {n}
                                </button>
                                <span className="font-mono text-[11px] tabular-nums text-muted-foreground">
                                  {hit.score.toFixed(3)}
                                </span>
                              </div>
                              {locus ? (
                                <p
                                  className="mb-1.5 truncate text-xs font-medium text-foreground/75"
                                  data-testid="deal-room-knowledge-locus"
                                >
                                  {locus}
                                </p>
                              ) : null}
                              <p className="line-clamp-4 text-[13px] leading-relaxed text-muted-foreground whitespace-pre-wrap">
                                {hit.text}
                              </p>
                              {canJump ? (
                                <Button
                                  size="sm"
                                  variant="outline"
                                  className="mt-3 h-8 border-border/80"
                                  data-testid="deal-room-knowledge-jump"
                                  onClick={() =>
                                    onOpenViewer(hit.documentId!, hit.viewerPage)
                                  }
                                >
                                  {t("knowledge.openPage", { page: hit.viewerPage })}
                                  <ArrowRight size={14} className="ml-1.5" />
                                </Button>
                              ) : null}
                              {canOpenDoc ? (
                                <Button
                                  size="sm"
                                  variant="outline"
                                  className="mt-3 h-8 border-border/80"
                                  title={
                                    hit.sheet
                                      ? t("knowledge.sheetMapMissing")
                                      : t("knowledge.noPageLocus")
                                  }
                                  data-testid="deal-room-knowledge-jump-doc"
                                  onClick={() => onOpenViewer(hit.documentId!)}
                                >
                                  {t("knowledge.openDocument")}
                                  <ArrowRight size={14} className="ml-1.5" />
                                </Button>
                              ) : null}
                            </li>
                          );
                        })}
                      </ul>
                    </div>
                  ) : null}
                </div>

                {onFeedback && isTerminalTurn(turn) ? (
                  <div className="rounded-2xl border border-border/70 bg-background">
                    <TurnFeedbackControls
                      turnId={turn.id}
                      feedback={turn.feedback}
                      disabled={asking}
                      onSubmit={(body) => onFeedback(turn.id, body)}
                    />
                  </div>
                ) : null}
              </section>
            );
          })
        ) : (
          <p className="px-1 text-sm text-muted-foreground">
            {t("knowledge.sourcesEmpty")}
          </p>
        )}
      </div>

      <div
        className="shrink-0 space-y-3 border-t border-border/60 bg-[linear-gradient(180deg,rgba(255,255,255,0)_0%,#ffffff_18%)] pt-3"
        data-testid="grounded-chat-dock"
      >
        {followUps.length > 0 ? (
          <div className="space-y-2" data-testid="grounded-chat-follow-ups">
            <p className="px-1 font-mono text-[10px] font-medium uppercase tracking-[0.12em] text-muted-foreground">
              {t("knowledge.followUpLabel")}
            </p>
            <div className="flex flex-wrap gap-2">
              {followUps.map((tip) => {
                const label = t(tip.messageKey, tip.params);
                return (
                  <button
                    key={tip.id}
                    type="button"
                    data-testid={`grounded-chat-follow-up-${tip.id}`}
                    disabled={asking}
                    className="rounded-full border border-border/80 bg-background px-3 py-1.5 text-left text-[12px] font-medium text-foreground/85 transition-colors hover:bg-muted/50 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50"
                    onClick={() => onQueryChange(label)}
                  >
                    {label}
                  </button>
                );
              })}
            </div>
          </div>
        ) : null}

        <div className="rounded-xl border border-foreground/10 bg-background p-2 shadow-[0_1px_0_rgba(15,23,42,0.03),0_12px_32px_-18px_rgba(15,23,42,0.28)]">
          <label
            htmlFor="grounded-chat-query"
            className="px-3 font-mono text-[10px] font-medium uppercase tracking-[0.12em] text-muted-foreground"
          >
            {t("knowledge.queryLabel")}
          </label>
          <div className="mt-1.5 flex flex-col gap-2 sm:flex-row sm:items-stretch">
            <Input
              id="grounded-chat-query"
              value={query}
              onChange={(e) => onQueryChange(e.target.value)}
              placeholder={t("knowledge.queryPlaceholder")}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !asking) onAsk();
              }}
              className="h-11 border-0 bg-transparent px-3 text-[15px] shadow-none focus-visible:ring-0"
            />
            {asking && onStop ? (
              <Button
                variant="outline"
                className="h-11 shrink-0 px-5"
                onClick={onStop}
                data-testid="grounded-chat-stop"
              >
                <Stop size={16} className="mr-1.5" weight="fill" />
                {t("knowledge.stop")}
              </Button>
            ) : (
              <Button
                className="h-11 shrink-0 px-5"
                disabled={asking || !query.trim()}
                onClick={onAsk}
                data-testid="deal-room-knowledge-ask"
              >
                {asking ? (
                  <CircleNotch size={16} className="mr-1.5 animate-spin" />
                ) : (
                  <MagnifyingGlass size={16} className="mr-1.5" weight="bold" />
                )}
                {asking ? t("knowledge.querying") : t("knowledge.ask")}
              </Button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
