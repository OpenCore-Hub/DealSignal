import { ListChecks } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { filterPromotableDeskTexts } from "@/lib/knowledge/trustGates";
import { cn } from "@/lib/utils";

interface UnresolvedGapsPanelProps {
  gaps: string[];
  onAskGap?: (gap: string) => void;
  className?: string;
}

/** Show ~3 gap rows (text + ask), then scroll. */
const GAPS_LIST_SCROLL =
  "max-h-[calc(3*4.75rem+2*0.5rem)] overflow-y-auto overscroll-contain";

/**
 * Lists unbound factual sentences as actionable desk gaps (ceiling Phase J / L2).
 */
export function UnresolvedGapsPanel({
  gaps,
  onAskGap,
  className,
}: UnresolvedGapsPanelProps) {
  const { t } = useTranslation("dealRooms");
  const items = filterPromotableDeskTexts(gaps);
  if (!items.length) return null;

  return (
    <div
      className={cn(
        "rounded-xl border border-border/60 bg-muted/[0.18] px-3.5 py-2.5",
        className,
      )}
      data-testid="knowledge-unresolved-gaps-panel"
      role="status"
    >
      <div className="mb-1.5 flex items-center gap-1.5 text-[12px] font-semibold text-foreground/85">
        <ListChecks size={14} weight="duotone" className="text-foreground/55" />
        {t("knowledge.unresolvedGapsTitle")}
      </div>
      <p className="mb-2 text-[11px] leading-relaxed text-muted-foreground">
        {t("knowledge.unresolvedGapsHint")}
      </p>
      <ul
        className={cn("space-y-2", GAPS_LIST_SCROLL)}
        data-testid="knowledge-unresolved-gaps-list"
      >
        {items.map((gap, i) => (
          <li
            key={`${i}-${gap.slice(0, 24)}`}
            className="rounded-lg border border-border/50 bg-background/80 px-2.5 py-2"
            data-testid={`knowledge-unresolved-gap-${i}`}
          >
            <p className="text-[12px] leading-snug text-foreground/80">{gap}</p>
            {onAskGap ? (
              <Button
                type="button"
                size="sm"
                variant="ghost"
                className="mt-1.5 h-7 px-2 text-[11px]"
                onClick={() => onAskGap(gap)}
              >
                {t("knowledge.unresolvedGapAsk")}
              </Button>
            ) : null}
          </li>
        ))}
      </ul>
    </div>
  );
}
