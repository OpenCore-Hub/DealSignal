import { useEffect, useMemo, useState } from "react";
import {
  BookmarkSimple,
  CaretDown,
  CaretUp,
  Files,
  Question,
} from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { isPromotableDeskFollowUpText } from "@/lib/knowledge/trustGates";
import type { DealRoomKnowledgeSessionState } from "@/types";
import { cn } from "@/lib/utils";

interface SessionStateRailProps {
  state?: DealRoomKnowledgeSessionState | null;
  onAskOpenQuestion?: (text: string) => void;
  className?: string;
}

/** Show ~5 compact rows, then scroll. */
const RAIL_LIST_SCROLL =
  "max-h-[calc(5*2.625rem+4*0.375rem)] overflow-y-auto overscroll-contain";

function hasStateContent(
  openQuestions: { text: string }[],
  state?: DealRoomKnowledgeSessionState | null,
): boolean {
  if (!state) return false;
  return (
    openQuestions.length > 0 ||
    (state.entities?.length ?? 0) > 0 ||
    (state.coverageHints?.length ?? 0) > 0
  );
}

/**
 * Auditable desk state machine surface (ceiling Phase L / L3).
 * Shows open gaps, provenanced entities, and recent coverage — not chat memory.
 */
export function SessionStateRail({
  state,
  onAskOpenQuestion,
  className,
}: SessionStateRailProps) {
  const { t } = useTranslation("dealRooms");
  const openQuestions = useMemo(
    () =>
      (state?.openQuestions ?? []).filter((oq) =>
        isPromotableDeskFollowUpText(oq.text),
      ),
    [state?.openQuestions],
  );
  const entities = state?.entities ?? [];
  const coverage = state?.coverageHints?.[state.coverageHints.length - 1];
  const hasGaps = openQuestions.length > 0;
  // Auto-expand when there are actionable open gaps.
  const [expanded, setExpanded] = useState(hasGaps);

  useEffect(() => {
    if (hasGaps) setExpanded(true);
  }, [hasGaps, openQuestions.length]);

  if (!hasStateContent(openQuestions, state)) return null;

  return (
    <aside
      className={cn(
        "rounded-xl border border-border/60 bg-muted/[0.15] px-3.5 py-3",
        className,
      )}
      data-testid="knowledge-session-state-rail"
      data-expanded={expanded ? "true" : "false"}
      aria-label={t("knowledge.sessionStateTitle")}
    >
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
            {t("knowledge.sessionStateTitle")}
          </p>
          {hasGaps ? (
            <p className="mt-1 flex items-center gap-1.5 text-[12px] font-medium text-foreground/80">
              <Question size={13} weight="duotone" />
              {t("knowledge.sessionStateOpenQuestions", {
                count: openQuestions.length,
              })}
            </p>
          ) : (
            <p className="mt-1 text-[12px] text-muted-foreground">
              {t("knowledge.sessionStateNoGaps")}
            </p>
          )}
        </div>
        <Button
          type="button"
          size="sm"
          variant="ghost"
          className="h-7 shrink-0 px-2 text-[11px] text-muted-foreground"
          onClick={() => setExpanded((v) => !v)}
          aria-expanded={expanded}
          data-testid="knowledge-session-state-toggle"
        >
          {expanded
            ? t("knowledge.sessionStateCollapse")
            : t("knowledge.sessionStateExpand")}
          {expanded ? (
            <CaretUp size={12} className="ml-1" weight="bold" />
          ) : (
            <CaretDown size={12} className="ml-1" weight="bold" />
          )}
        </Button>
      </div>

      {expanded ? (
        <div className="mt-3" data-testid="knowledge-session-state-details">
          <p className="mb-3 text-[11px] leading-relaxed text-muted-foreground">
            {t("knowledge.sessionStateHint")}
          </p>

          {openQuestions.length > 0 ? (
            <div className="mb-3" data-testid="knowledge-session-open-questions">
              <ul
                className={cn("space-y-1.5", RAIL_LIST_SCROLL)}
                data-testid="knowledge-session-open-questions-list"
              >
                {openQuestions.map((oq, i) => (
                  <li
                    key={`${oq.sourceTurnId}-${i}`}
                    className="flex flex-wrap items-start justify-between gap-2 rounded-lg border border-border/50 bg-background/80 px-2.5 py-1.5"
                    data-testid="knowledge-session-open-question"
                  >
                    <p className="min-w-0 flex-1 text-[12px] leading-snug text-foreground/80">
                      {oq.text}
                    </p>
                    {onAskOpenQuestion ? (
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        className="h-7 shrink-0 px-2 text-[11px]"
                        onClick={() => onAskOpenQuestion(oq.text)}
                      >
                        {t("knowledge.sessionStateAskGap")}
                      </Button>
                    ) : null}
                  </li>
                ))}
              </ul>
            </div>
          ) : null}

          {entities.length > 0 ? (
            <div className="mb-3" data-testid="knowledge-session-entities">
              <div className="mb-1.5 flex items-center gap-1.5 text-[12px] font-medium text-foreground/80">
                <BookmarkSimple size={13} weight="duotone" />
                {t("knowledge.sessionStateEntities")}
              </div>
              <ul className="flex flex-wrap gap-1.5">
                {entities.slice(0, 8).map((e, i) => (
                  <li
                    key={`${e.name}-${i}`}
                    className="rounded-md border border-border/50 bg-background/70 px-2 py-0.5 text-[11px] text-foreground/75"
                    title={e.type}
                  >
                    <span className="font-medium">{e.name}</span>
                    {e.type && e.type !== "document" ? (
                      <span className="ml-1 text-muted-foreground">· {e.type}</span>
                    ) : null}
                  </li>
                ))}
              </ul>
            </div>
          ) : null}

          {coverage && (coverage.sourceNames?.length ?? 0) > 0 ? (
            <div data-testid="knowledge-session-coverage">
              <div className="mb-1.5 flex items-center gap-1.5 text-[12px] font-medium text-foreground/80">
                <Files size={13} weight="duotone" />
                {t("knowledge.sessionStateCoverage")}
              </div>
              <p className="text-[11px] leading-snug text-muted-foreground">
                {coverage.sourceNames.join(t("knowledge.pageListSep"))}
              </p>
            </div>
          ) : null}
        </div>
      ) : null}
    </aside>
  );
}
