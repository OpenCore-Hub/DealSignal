import { MagnifyingGlass } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { DealRoomMetricCard } from "@/components/deal-rooms/DealRoomMetricCard";

export interface KnowledgeAskEntryCardProps {
  onStartAsk: () => void;
  /**
   * True only when the vector library is truly ready (synced corpus, status「就绪」).
   * When false, the start CTA stays visible but disabled.
   */
  ready?: boolean;
}

/**
 * Landing entry into room-scoped grounded Q&A.
 * Tags communicate scope / isolation / citation trust before the chat surface opens.
 */
export function KnowledgeAskEntryCard({
  onStartAsk,
  ready = false,
}: KnowledgeAskEntryCardProps) {
  const { t } = useTranslation("dealRooms");

  return (
    <DealRoomMetricCard
      title={t("knowledge.askEntryTitle")}
      metrics={[
        {
          label: t("knowledge.askEntryTagScope"),
          value: t("knowledge.trustScoped"),
        },
        {
          label: t("knowledge.askEntryTagSecurity"),
          value: t("knowledge.trustIsolated"),
        },
        {
          label: t("knowledge.askEntryTagTrust"),
          value: t("knowledge.trustGrounded"),
        },
      ]}
      metricValueClassName="font-normal text-muted-foreground"
      footerNote={
        ready ? t("knowledge.askEntryNoteReady") : t("knowledge.askEntryBuilding")
      }
      footerActions={
        <Button
          size="sm"
          className="h-8"
          disabled={!ready}
          onClick={onStartAsk}
          data-testid="deal-room-knowledge-ask-entry-start"
          aria-disabled={!ready}
        >
          <MagnifyingGlass size={14} className="mr-1.5" weight="bold" />
          {t("knowledge.askEntryAction")}
        </Button>
      }
      data-testid="deal-room-knowledge-ask-entry"
    />
  );
}
