import { FlowArrow } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import type { DealRoomKnowledgeMultiHop } from "@/types";
import { cn } from "@/lib/utils";

interface MultiHopPanelProps {
  multiHop: DealRoomKnowledgeMultiHop;
  className?: string;
}

/**
 * Surfaces audited second-hop retrieve (ceiling Phase I3) — definition / attachment only.
 */
export function MultiHopPanel({ multiHop, className }: MultiHopPanelProps) {
  const { t } = useTranslation("dealRooms");
  if (!multiHop.applied && !(multiHop.queries?.length ?? 0)) return null;

  const queries = multiHop.queries ?? [];
  const added = multiHop.addedHitIds?.length ?? 0;

  return (
    <div
      className={cn(
        "rounded-xl border border-border/60 bg-muted/[0.2] px-3.5 py-2.5",
        className,
      )}
      data-testid="knowledge-multi-hop-panel"
      role="status"
    >
      <div className="mb-1.5 flex items-center gap-1.5 text-[12px] font-semibold text-foreground/85">
        <FlowArrow size={14} weight="duotone" className="text-foreground/55" />
        {t("knowledge.multiHopTitle")}
      </div>
      <p className="mb-2 text-[11px] leading-relaxed text-muted-foreground">
        {multiHop.applied
          ? t("knowledge.multiHopHintApplied", { count: added })
          : t("knowledge.multiHopHintAttempted")}
      </p>
      {queries.length > 0 ? (
        <ul className="space-y-1">
          {queries.map((q, i) => (
            <li
              key={`${q.kind}-${q.anchor ?? q.query}-${i}`}
              className="truncate text-[11px] text-foreground/75"
              data-testid={`knowledge-multi-hop-${q.kind}`}
            >
              <span className="font-medium text-foreground/60">
                {q.kind === "attachment"
                  ? t("knowledge.multiHopKindAttachment")
                  : t("knowledge.multiHopKindDefinition")}
              </span>
              {": "}
              <span className="font-mono">{q.anchor || q.query}</span>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
