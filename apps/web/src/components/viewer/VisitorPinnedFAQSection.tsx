import { CaretDown, PushPin } from "@phosphor-icons/react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { AnswerMarkdown } from "@/components/deal-rooms/knowledge/AnswerMarkdown";
import { Button } from "@/components/ui/button";
import { formatHitLocusLabel } from "@/lib/knowledge/citations";
import { cn } from "@/lib/utils";
import type { DealRoomKnowledgeQueryHit, PublicAskFAQ } from "@/types";

export interface VisitorPinnedFAQSectionProps {
  faqs: PublicAskFAQ[];
  onSuggestQuestion?: (question: string) => void;
  onOpenCitation?: (hit: DealRoomKnowledgeQueryHit) => void;
}

export function VisitorPinnedFAQSection({
  faqs,
  onSuggestQuestion,
  onOpenCitation,
}: VisitorPinnedFAQSectionProps) {
  const { t } = useTranslation("documents");
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [activeCiteById, setActiveCiteById] = useState<Record<string, number | null>>({});

  if (faqs.length === 0) return null;

  const showLinkLabels = new Set(faqs.map((faq) => faq.link_id).filter(Boolean)).size > 1;

  const locusFmt = {
    sheetPrefix: t("viewer.askSheetLabel"),
    pageSingle: (page: number) => t("viewer.askPageSingle", { page }),
    pageRange: (from: number, to: number) => t("viewer.askPageRange", { from, to }),
    pageListSep: t("viewer.askPageListSep"),
    pageList: (pages: string) => t("viewer.askPageList", { pages }),
  };

  const toggle = (id: string) => {
    setExpandedId((prev) => (prev === id ? null : id));
  };

  return (
    <section
      className="rounded-2xl border border-amber-500/20 bg-amber-500/5 p-3"
      aria-label={t("viewer.askFaqSectionTitle")}
    >
      <div className="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-amber-900 dark:text-amber-100">
        <PushPin size={14} weight="fill" />
        {t("viewer.askFaqSectionTitle")}
      </div>
      <ul className="space-y-2">
        {faqs.map((faq) => {
          const expanded = expandedId === faq.id;
          const hits = faq.ai_payload?.hits ?? [];
          const activeCite = activeCiteById[faq.id] ?? null;
          const useMarkdown = Boolean(faq.ai_payload?.answer && faq.answer === faq.ai_payload.answer);

          const openHit = (hit: DealRoomKnowledgeQueryHit, citeNum: number) => {
            setActiveCiteById((prev) => ({ ...prev, [faq.id]: citeNum }));
            if (!onOpenCitation) return;
            const viewerPage = hit.viewerPage ?? hit.pages?.[0];
            if (!hit.documentId || !viewerPage) return;
            onOpenCitation({ ...hit, viewerPage });
          };

          return (
            <li key={faq.id} className="overflow-hidden rounded-xl border border-border/60 bg-background/80">
              <div className="flex items-start gap-1">
                <button
                  type="button"
                  className="flex min-w-0 flex-1 items-start gap-2 px-3 py-2.5 text-left text-sm"
                  onClick={() => toggle(faq.id)}
                  aria-expanded={expanded}
                >
                  <CaretDown
                    size={14}
                    className={cn(
                      "mt-0.5 shrink-0 text-muted-foreground transition-transform",
                      expanded && "rotate-180",
                    )}
                  />
                  <span className="min-w-0">
                    <span className="font-medium text-foreground">{faq.question}</span>
                    {showLinkLabels && faq.link_name ? (
                      <span className="mt-0.5 block text-[11px] font-normal text-muted-foreground">
                        {t("viewer.askFaqFromLink", { link: faq.link_name })}
                      </span>
                    ) : null}
                  </span>
                </button>
                {onSuggestQuestion ? (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="mr-1 mt-1 h-7 shrink-0 text-[11px]"
                    onClick={() => onSuggestQuestion(faq.question)}
                  >
                    {t("viewer.askFaqAskSimilar")}
                  </Button>
                ) : null}
              </div>
              {expanded ? (
                <div className="space-y-2 border-t border-border/50 px-3 py-3 text-sm">
                  {useMarkdown ? (
                    <AnswerMarkdown
                      answer={faq.answer}
                      activeCite={activeCite ?? undefined}
                      onCite={(n) => {
                        const hit = hits[n - 1];
                        if (hit) openHit(hit, n);
                      }}
                    />
                  ) : (
                    <p className="whitespace-pre-wrap text-muted-foreground">{faq.answer}</p>
                  )}
                  {hits.length > 0 ? (
                    <div className="space-y-1.5 pt-1">
                      <p className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">
                        {t("viewer.askEvidenceTitle", { count: hits.length })}
                      </p>
                      {hits.slice(0, 3).map((hit, idx) => {
                        const citeNum = idx + 1;
                        const locus = formatHitLocusLabel(hit, locusFmt);
                        const viewerPage = hit.viewerPage ?? hit.pages?.[0];
                        const canJump = Boolean(onOpenCitation && hit.documentId && viewerPage);
                        return (
                          <div
                            key={hit.chunkId || `${hit.documentId}-${idx}`}
                            className={cn(
                              "rounded border border-border/40 bg-muted/30 px-2 py-1.5 text-xs",
                              activeCite === citeNum && "border-foreground/30",
                            )}
                          >
                            {locus ? <p className="font-medium">{locus}</p> : null}
                            <p className="line-clamp-2 text-muted-foreground">{hit.text}</p>
                            {canJump ? (
                              <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                className="mt-2 h-7 rounded-full text-[11px]"
                                onClick={() => openHit(hit, citeNum)}
                              >
                                {t("viewer.openPage", { pageNumber: viewerPage })}
                              </Button>
                            ) : null}
                          </div>
                        );
                      })}
                    </div>
                  ) : null}
                </div>
              ) : null}
            </li>
          );
        })}
      </ul>
    </section>
  );
}
