import { CaretDown, Scales } from "@phosphor-icons/react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { PublicFormalAsk } from "@/types";

export interface VisitorFormalQASectionProps {
  entries: PublicFormalAsk[];
  onSuggestQuestion?: (question: string) => void;
}

export function VisitorFormalQASection({
  entries,
  onSuggestQuestion,
}: VisitorFormalQASectionProps) {
  const { t } = useTranslation("documents");
  const [expandedId, setExpandedId] = useState<string | null>(null);

  if (entries.length === 0) return null;

  const showLinkLabels = new Set(entries.map((entry) => entry.link_id).filter(Boolean)).size > 1;

  return (
    <section
      className="rounded-2xl border border-sky-500/20 bg-sky-500/5 p-3"
      aria-label={t("viewer.askFormalSectionTitle")}
    >
      <div className="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-sky-900 dark:text-sky-100">
        <Scales size={14} weight="fill" />
        {t("viewer.askFormalSectionTitle")}
      </div>
      <p className="mb-3 text-[11px] text-muted-foreground">{t("viewer.askFormalSectionHint")}</p>
      <ul className="space-y-2">
        {entries.map((entry) => {
          const expanded = expandedId === entry.id;
          return (
            <li
              key={entry.id}
              className="overflow-hidden rounded-xl border border-border/60 bg-background/80"
            >
              <div className="flex items-start gap-1">
                <button
                  type="button"
                  className="flex min-w-0 flex-1 items-start gap-2 px-3 py-2.5 text-left text-sm"
                  onClick={() => setExpandedId((prev) => (prev === entry.id ? null : entry.id))}
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
                    <span className="font-medium text-foreground">{entry.question}</span>
                    {showLinkLabels && entry.link_name ? (
                      <span className="mt-0.5 block text-[11px] font-normal text-muted-foreground">
                        {t("viewer.askFormalFromLink", { link: entry.link_name })}
                      </span>
                    ) : null}
                    {entry.visitor_email ? (
                      <span className="mt-0.5 block text-[11px] font-normal text-muted-foreground">
                        {t("viewer.askFormalAskedBy", { email: entry.visitor_email })}
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
                    onClick={() => onSuggestQuestion(entry.question)}
                  >
                    {t("viewer.askFormalAskSimilar")}
                  </Button>
                ) : null}
              </div>
              {expanded ? (
                <div className="border-t border-border/50 px-3 py-3 text-sm">
                  <p className="whitespace-pre-wrap text-muted-foreground">{entry.answer}</p>
                </div>
              ) : null}
            </li>
          );
        })}
      </ul>
    </section>
  );
}
