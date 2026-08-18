import type { ReactNode } from "react";
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
import { renderBoundClaims } from "@/lib/knowledge/boundAnswer";
import {
  formatHitLocusLabel,
  renderAnswerWithCitations,
  type LocusFormatters,
} from "@/lib/knowledge/citations";
import { AnswerMarkdown } from "@/components/deal-rooms/knowledge/AnswerMarkdown";
import { ConflictPanel } from "@/components/deal-rooms/knowledge/ConflictPanel";
import { MultiHopPanel } from "@/components/deal-rooms/knowledge/MultiHopPanel";
import { RefusalPanel } from "@/components/deal-rooms/knowledge/RefusalPanel";
import { UnresolvedGapsPanel } from "@/components/deal-rooms/knowledge/UnresolvedGapsPanel";
import { TurnFeedbackControls } from "@/components/deal-rooms/knowledge/TurnFeedbackControls";
import { knowledgeErrorMessage } from "@/lib/knowledge/errors";
import { buildRoomFollowUps } from "@/lib/knowledge/followUps";
import type { KnowledgeTurn } from "@/lib/knowledge/streamEvents";
import {
  shouldShowEvidence,
  turnRetrieveDisclosure,
} from "@/lib/knowledge/streamEvents";
import { cn } from "@/lib/utils";
import type {
  DealRoomKnowledgeFeedbackKind,
  DealRoomKnowledgeFollowUpSuggestion,
} from "@/types";

export interface GroundedChatShellProps {
  /** Current composer value (controlled by room store). */
  query: string;
  onQueryChange: (value: string) => void;
  /** Ordered audit turns (oldest → newest). Empty = composer only. */
  turns: KnowledgeTurn[];
  asking: boolean;
  /** When false, Ask is disabled (e.g. corpus left ready after desk opened). */
  askEnabled?: boolean;
  /** Optional override asks immediately with that text (follow-up chips). */
  onAsk: (overrideQuery?: string) => void;
  /** Optional stop for AbortController-backed streams. */
  onStop?: () => void;
  onActiveCite: (n: number | null) => void;
  onOpenViewer: (documentId: string, page?: number) => void;
  onFeedback?: (
    turnId: string,
    body: { kind: DealRoomKnowledgeFeedbackKind; note?: string },
  ) => Promise<void>;
  /**
   * Server/evidence-grounded chips. When provided (incl. empty), replaces local
   * V1 templates. When omitted, shell falls back to buildRoomFollowUps.
   */
  followUpChips?: DealRoomKnowledgeFollowUpSuggestion[] | null;
  /** llm | template — surfaces after progressive upgrade settles. */
  followUpSource?: string;
  /** True while POST …/follow-ups is in flight. */
  followUpUpgrading?: boolean;
  /** Hover/focus on the chip dock — parent may defer chip swaps. */
  onFollowUpsEngagedChange?: (engaged: boolean) => void;
  /**
   * Content rendered above the turn timeline (e.g. mission rail on the desk).
   * Scrolls with turns when `layout="desk"`.
   */
  beforeTimeline?: ReactNode;
  /**
   * `rail` — capped height, internal scroll (Viewer sidebar).
   * `desk` — fills parent; timeline scrolls; composer dock stays pinned at the bottom.
   */
  layout?: "rail" | "desk";
  className?: string;
}

function isTerminalTurn(turn: KnowledgeTurn): boolean {
  return turn.phase === "done" || turn.phase === "refused" || turn.phase === "error";
}

/**
 * Recommended visible answer lines before the body scrolls (~50 × 15px / leading 1.7).
 * Static class string so Tailwind JIT can see it; evidence mirrors this budget.
 */
const TURN_BODY_MAX_H = "max-h-[calc(50*1.7em)]";

/**
 * Research-desk shell: scrollable Turn timeline above, pinned follow-ups + composer dock.
 * Philosophy: trust > fluency; evidence first-class; refuse hides rail.
 */
export function GroundedChatShell({
  query,
  onQueryChange,
  turns,
  asking,
  askEnabled = true,
  onAsk,
  onStop,
  onActiveCite,
  onOpenViewer,
  onFeedback,
  followUpChips = null,
  followUpSource = "template",
  followUpUpgrading = false,
  onFollowUpsEngagedChange,
  beforeTimeline,
  layout = "rail",
  className,
}: GroundedChatShellProps) {
  const { t } = useTranslation("dealRooms");
  const canAsk = askEnabled && !asking;
  const locusFmt: LocusFormatters = {
    sheetPrefix: t("knowledge.sheetLabel"),
    pageSingle: (page) => t("knowledge.pageSingle", { page }),
    pageRange: (from, to) => t("knowledge.pageRange", { from, to }),
    pageListSep: t("knowledge.pageListSep"),
    pageList: (pages) => t("knowledge.pageList", { pages }),
  };

  // Spec §8.3: show when ≥1 turn exists — prefer parent-provided evidence-grounded chips.
  const lastTerminal = [...turns].reverse().find(isTerminalTurn);
  const templateTips =
    turns.length > 0 && lastTerminal
      ? buildRoomFollowUps({
          refused: lastTerminal.refused,
          resultStatus: lastTerminal.resultStatus,
          hits: lastTerminal.results,
        })
      : [];
  const followUps: DealRoomKnowledgeFollowUpSuggestion[] =
    followUpChips != null
      ? followUpChips
      : templateTips.map((tip) => ({
          id: tip.id,
          text: t(tip.messageKey, tip.params),
        }));

  const isDesk = layout === "desk";

  return (
    <div
      className={cn(
        "flex flex-col",
        isDesk
          ? "min-h-0 flex-1"
          : "max-h-[min(70vh,44rem)] min-h-[18rem]",
        className,
      )}
      data-testid="grounded-chat-shell"
      data-layout={layout}
    >
      <div
        className="min-h-0 flex-1 space-y-5 overflow-y-auto overscroll-contain pr-0.5"
        data-testid="grounded-chat-timeline"
      >
        {beforeTimeline}
        {turns.length > 0 ? (
          turns.map((turn) => {
            const showEvidence = shouldShowEvidence(turn);
            const retrieveDisclosure = turnRetrieveDisclosure(turn);
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
                <div className="space-y-1 px-1">
                  <p className="text-sm font-medium text-foreground/85">
                    <span className="mr-2 font-mono text-[11px] uppercase tracking-[0.12em] text-muted-foreground">
                      {t("knowledge.turnQuestion")}
                    </span>
                    {turn.query}
                  </p>
                  {retrieveDisclosure ? (
                    <p
                      className="text-[12px] leading-snug text-muted-foreground"
                      data-testid="grounded-chat-turn-retrieve-query"
                    >
                      <span className="mr-1.5 font-mono text-[10px] uppercase tracking-[0.12em]">
                        {t("knowledge.retrieveQueryLabel")}
                      </span>
                      {retrieveDisclosure}
                    </p>
                  ) : null}
                </div>
                <div
                  className={cn(
                    "grid items-stretch gap-4",
                    showEvidence && "lg:grid-cols-[minmax(0,1.15fr)_minmax(0,0.85fr)]",
                  )}
                >
                  {/* Answer drives row height; body caps at ANSWER_VISIBLE_LINES. */}
                  <div
                    className="flex min-h-0 flex-col overflow-hidden rounded-2xl border border-border/70 bg-background"
                    data-testid="grounded-chat-answer-card"
                  >
                    <div className="flex shrink-0 items-center gap-2 border-b border-border/60 px-5 py-3.5">
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
                    <div
                      className={cn(
                        "min-h-0 overflow-y-auto overscroll-contain px-5 py-5 text-[15px] leading-[1.7]",
                        TURN_BODY_MAX_H,
                      )}
                      data-testid="grounded-chat-answer-body"
                    >
                      {turn.phase === "error" ? (
                        <div className="space-y-3">
                          <p className="text-sm text-destructive" role="alert">
                            {knowledgeErrorMessage(t, turn.errorMessage)}
                          </p>
                          {turn.refusal ? (
                            <RefusalPanel refusal={turn.refusal} />
                          ) : null}
                        </div>
                      ) : turn.answer || (turn.claims && turn.claims.length > 0) ? (
                        <div className="text-foreground/90">
                          {/*
                            B: answer owns layout (limited Markdown); claims own trust
                            (unresolved / ops). Streaming stays plain to avoid half-MD flicker.
                          */}
                          {turn.answer && turn.phase === "generating" ? (
                            <div className="whitespace-pre-wrap">
                              {renderAnswerWithCitations(turn.answer, (n) =>
                                onActiveCite(n),
                              )}
                              <span className="ml-0.5 inline-block h-4 w-0.5 animate-pulse bg-foreground/50 align-middle" />
                            </div>
                          ) : turn.answer ? (
                            <AnswerMarkdown
                              answer={turn.answer}
                              activeCite={turn.activeCite}
                              onCite={(n) => onActiveCite(n)}
                            />
                          ) : turn.claims && turn.claims.length > 0 ? (
                            <div className="whitespace-pre-wrap">
                              {renderBoundClaims(
                                turn.claims,
                                turn.results,
                                turn.activeCite,
                                (n) => onActiveCite(n),
                              )}
                            </div>
                          ) : null}
                          {turn.refusal &&
                          (turn.refused || turn.resultStatus === "no_hits") ? (
                            <RefusalPanel className="mt-3" refusal={turn.refusal} />
                          ) : null}
                          {(turn.unresolved?.length ?? 0) > 0 ? (
                            <UnresolvedGapsPanel
                              className="mt-3"
                              gaps={turn.unresolved!}
                              onAskGap={(gap) => onAsk(gap)}
                            />
                          ) : null}
                          {(turn.conflicts?.length ?? 0) > 0 ? (
                            <ConflictPanel
                              className="mt-3"
                              conflicts={turn.conflicts!}
                              onOpenHit={(hitId) => {
                                const idx = turn.results.findIndex(
                                  (h) => (h.chunkId || "").trim() === hitId.trim(),
                                );
                                if (idx >= 0) onActiveCite(idx + 1);
                              }}
                            />
                          ) : null}
                          {turn.multiHop?.applied ||
                          (turn.multiHop?.queries?.length ?? 0) > 0 ? (
                            <MultiHopPanel
                              className="mt-3"
                              multiHop={turn.multiHop!}
                            />
                          ) : null}
                        </div>
                      ) : turn.phase === "retrieving" || turn.phase === "generating" ? (
                        <p className="text-sm text-muted-foreground">{phaseHint}</p>
                      ) : (
                        <div className="space-y-3">
                          {turn.refusal ? (
                            <RefusalPanel refusal={turn.refusal} />
                          ) : (
                            <p className="text-sm text-muted-foreground">
                              {t("knowledge.noHits")}
                            </p>
                          )}
                        </div>
                      )}
                    </div>
                    {/* Same card as answer; only latest terminal turn (hidden when next ask starts). */}
                    {onFeedback &&
                    isTerminalTurn(turn) &&
                    turn.id === turns[turns.length - 1]?.id ? (
                      <div className="shrink-0 border-t border-border/60">
                        <TurnFeedbackControls
                          turnId={turn.id}
                          feedback={turn.feedback}
                          disabled={asking}
                          onSubmit={(body) => onFeedback(turn.id, body)}
                        />
                      </div>
                    ) : null}
                  </div>

                  {showEvidence ? (
                    <div
                      className={cn(
                        // Side-by-side: match answer card height (h-0 min-h-full).
                        // Stacked: own body budget so evidence does not grow unbound.
                        "flex min-h-0 flex-col overflow-hidden rounded-2xl border border-border/70 bg-muted/[0.25]",
                        "lg:h-0 lg:min-h-full",
                      )}
                      data-testid="grounded-chat-evidence-card"
                    >
                      <div className="flex shrink-0 items-center justify-between gap-2 border-b border-border/50 px-5 py-3.5">
                        <p className="text-sm font-semibold tracking-tight">
                          {t("knowledge.sourcesTitle")}
                        </p>
                        <span className="font-mono text-[11px] text-muted-foreground">
                          {t("knowledge.sourcesCount", { count: turn.results.length })}
                        </span>
                      </div>
                      <ul
                        className={cn(
                          "min-h-0 flex-1 space-y-2 overflow-y-auto overscroll-contain px-3 py-3",
                          TURN_BODY_MAX_H,
                          "lg:max-h-none",
                        )}
                        data-testid="grounded-chat-evidence-body"
                      >
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
              </section>
            );
          })
        ) : null}
      </div>

      <div
        className={cn(
          "shrink-0 space-y-3 border-t border-border/60 pt-3",
          isDesk
            ? // Stick near viewport bottom; keep a small inset from the card edge.
              "sticky bottom-[-1.5rem] z-20 -mx-5 rounded-b-2xl bg-background/95 px-5 pb-3 backdrop-blur supports-[backdrop-filter]:bg-background/90 sm:-mx-7 sm:px-7 md:bottom-[-2rem]"
            : "bg-[linear-gradient(180deg,rgba(255,255,255,0)_0%,#ffffff_18%)]",
        )}
        data-testid="grounded-chat-dock"
      >
        {followUps.length > 0 ? (
          <div
            className="space-y-2"
            data-testid="grounded-chat-follow-ups"
            data-source={followUpSource}
            data-upgrading={followUpUpgrading ? "true" : "false"}
            aria-busy={followUpUpgrading || undefined}
            onMouseEnter={() => onFollowUpsEngagedChange?.(true)}
            onMouseLeave={() => onFollowUpsEngagedChange?.(false)}
            onFocusCapture={() => onFollowUpsEngagedChange?.(true)}
            onBlurCapture={(e) => {
              if (!e.currentTarget.contains(e.relatedTarget as Node | null)) {
                onFollowUpsEngagedChange?.(false);
              }
            }}
          >
            <div className="flex flex-wrap items-center gap-x-2 gap-y-1 px-1">
              <p className="font-mono text-[10px] font-medium uppercase tracking-[0.12em] text-muted-foreground">
                {t("knowledge.followUpLabel")}
              </p>
              {followUpUpgrading ? (
                <span
                  className="inline-flex items-center gap-1 font-mono text-[10px] uppercase tracking-[0.1em] text-muted-foreground/80"
                  data-testid="grounded-chat-follow-ups-upgrading"
                >
                  <CircleNotch size={11} className="animate-spin" weight="bold" />
                  {t("knowledge.followUpUpgrading")}
                </span>
              ) : followUpSource === "llm" || followUpSource === "mission" ? (
                <span
                  className="font-mono text-[10px] uppercase tracking-[0.1em] text-muted-foreground/80"
                  data-testid="grounded-chat-follow-ups-source"
                >
                  {followUpSource === "mission"
                    ? t("knowledge.followUpSourceMission")
                    : t("knowledge.followUpSourceEvidence")}
                </span>
              ) : null}
            </div>
            <div className="flex flex-wrap gap-2">
              {followUps.map((tip) => (
                <button
                  key={tip.id}
                  type="button"
                  data-testid={`grounded-chat-follow-up-${tip.id}`}
                  disabled={!canAsk}
                  className="rounded-full border border-border/80 bg-background px-3 py-1.5 text-left text-[12px] font-medium text-foreground/85 transition-colors hover:bg-muted/50 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50"
                  onClick={() => {
                    onQueryChange(tip.text);
                    onAsk(tip.text);
                  }}
                >
                  {tip.text}
                </button>
              ))}
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
              disabled={!canAsk}
              onKeyDown={(e) => {
                if (e.key === "Enter" && canAsk && query.trim()) onAsk();
              }}
              className="h-11 border-0 bg-transparent px-3 text-[15px] shadow-none focus-visible:ring-0 disabled:cursor-not-allowed disabled:opacity-60"
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
                disabled={!canAsk || !query.trim()}
                onClick={() => onAsk()}
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
