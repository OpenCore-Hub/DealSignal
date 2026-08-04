import { Warning } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import type { DealRoomKnowledgeHitConflict } from "@/types";
import { cn } from "@/lib/utils";

interface ConflictPanelProps {
  conflicts: DealRoomKnowledgeHitConflict[];
  onOpenHit?: (hitId: string) => void;
  className?: string;
}

/**
 * Lists cross-file disagreements without picking a side (ceiling Phase I / §3.1).
 */
export function ConflictPanel({ conflicts, onOpenHit, className }: ConflictPanelProps) {
  const { t } = useTranslation("dealRooms");
  if (!conflicts.length) return null;

  return (
    <div
      className={cn(
        "rounded-xl border border-amber-500/25 bg-amber-500/[0.04] px-3.5 py-3",
        className,
      )}
      data-testid="knowledge-conflict-panel"
      role="status"
    >
      <div className="mb-2 flex items-center gap-1.5 text-[12px] font-semibold text-foreground/85">
        <Warning size={14} weight="duotone" className="text-amber-700/80" />
        {t("knowledge.conflictTitle")}
      </div>
      <p className="mb-2.5 text-[11px] leading-relaxed text-muted-foreground">
        {t("knowledge.conflictHint")}
      </p>
      <ul className="space-y-2.5">
        {conflicts.map((c) => (
          <li key={c.id} data-testid={`knowledge-conflict-${c.id}`}>
            <p className="text-[12px] font-medium text-foreground/80">
              {t("knowledge.conflictTopic", {
                topic: c.topic || t("knowledge.conflictTopicFallback"),
              })}
            </p>
            <ul className="mt-1 space-y-1">
              {(c.sides ?? []).map((side, i) => (
                <li
                  key={`${c.id}-${side.sourceName}-${i}`}
                  className="rounded-lg border border-border/50 bg-background/80 px-2.5 py-1.5 text-[11px] leading-snug"
                >
                  <div className="flex flex-wrap items-baseline justify-between gap-2">
                    <button
                      type="button"
                      className={cn(
                        "truncate font-medium text-foreground/85",
                        side.hitId && onOpenHit && "hover:underline",
                      )}
                      disabled={!side.hitId || !onOpenHit}
                      onClick={() => {
                        if (side.hitId && onOpenHit) onOpenHit(side.hitId);
                      }}
                    >
                      {side.sourceName}
                    </button>
                    {side.value ? (
                      <span className="font-mono text-[11px] text-foreground/70">
                        {side.value}
                      </span>
                    ) : null}
                  </div>
                  {side.excerpt ? (
                    <p className="mt-0.5 line-clamp-2 text-muted-foreground">
                      {side.excerpt}
                    </p>
                  ) : null}
                </li>
              ))}
            </ul>
          </li>
        ))}
      </ul>
    </div>
  );
}
