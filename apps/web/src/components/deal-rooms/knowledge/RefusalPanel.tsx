import { SealWarning } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import type { DealRoomKnowledgeRefusal } from "@/types";
import { cn } from "@/lib/utils";

interface RefusalPanelProps {
  refusal: DealRoomKnowledgeRefusal;
  className?: string;
}

/**
 * Typed L2 refusal / retrieval gap (ceiling Phase J). Honest about why the desk did not stamp.
 */
export function RefusalPanel({ refusal, className }: RefusalPanelProps) {
  const { t } = useTranslation("dealRooms");
  const kind = (refusal.kind || "").trim() || "no_hits";

  let title: string;
  let hint: string;
  switch (kind) {
    case "ungrounded":
      title = t("knowledge.refusalUngroundedTitle");
      hint = refusal.hadHits
        ? t("knowledge.refusalUngroundedHintHadHits")
        : t("knowledge.refusalUngroundedHint");
      break;
    case "error":
      title = t("knowledge.refusalErrorTitle");
      hint = t("knowledge.refusalErrorHint");
      break;
    default:
      title = t("knowledge.refusalNoHitsTitle");
      hint = refusal.hadHits
        ? t("knowledge.refusalNoHitsHintHadHits")
        : t("knowledge.refusalNoHitsHint");
      break;
  }

  return (
    <div
      className={cn(
        "rounded-xl border border-border/60 bg-muted/[0.18] px-3.5 py-2.5",
        className,
      )}
      data-testid="knowledge-refusal-panel"
      data-refusal-kind={kind}
      role="status"
    >
      <div className="mb-1 flex items-center gap-1.5 text-[12px] font-semibold text-foreground/85">
        <SealWarning size={14} weight="duotone" className="text-foreground/55" />
        {title}
      </div>
      <p className="text-[11px] leading-relaxed text-muted-foreground">{hint}</p>
    </div>
  );
}
