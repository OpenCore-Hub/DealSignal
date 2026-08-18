import { MagnifyingGlass } from "@phosphor-icons/react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { filterVisitorFAQs } from "@/lib/visitorFaqSearch";
import type { DealRoomKnowledgeQueryHit, PublicAskFAQ } from "@/types";
import { VisitorPinnedFAQSection } from "./VisitorPinnedFAQSection";

export interface VisitorFaqPanelProps {
  faqs: PublicAskFAQ[];
  onAskQuestion: (question: string) => void;
  onAskThis?: (question: string) => void;
  onOpenCitation?: (hit: DealRoomKnowledgeQueryHit) => void;
}

export function VisitorFaqPanel({
  faqs,
  onAskQuestion,
  onAskThis,
  onOpenCitation,
}: VisitorFaqPanelProps) {
  const { t } = useTranslation("documents");
  const [query, setQuery] = useState("");
  const filtered = useMemo(() => filterVisitorFAQs(faqs, query), [faqs, query]);
  const trimmed = query.trim();
  const emptySearch = trimmed.length > 0 && filtered.length === 0;

  return (
    <div className="flex h-full min-h-0 flex-col p-4">
      <label className="sr-only" htmlFor="visitor-faq-search">
        {t("viewer.askFaqSearchPlaceholder")}
      </label>
      <div className="relative mb-3">
        <MagnifyingGlass
          size={16}
          className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"
        />
        <Input
          id="visitor-faq-search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t("viewer.askFaqSearchPlaceholder")}
          className="pl-9"
        />
      </div>
      {emptySearch ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-3 rounded-2xl border border-border/60 bg-background/60 px-6 py-10 text-center">
          <p className="text-sm font-medium text-foreground">{t("viewer.askFaqSearchEmpty")}</p>
          <p className="max-w-[32ch] text-xs text-muted-foreground">{t("viewer.askFaqSearchEmptyHint")}</p>
          <Button type="button" size="sm" onClick={() => onAskQuestion(trimmed)}>
            {t("viewer.askFaqGoAsk")}
          </Button>
        </div>
      ) : (
        <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain">
          <VisitorPinnedFAQSection
            faqs={filtered}
            hideHeading
            onSuggestQuestion={onAskThis ?? onAskQuestion}
            onOpenCitation={onOpenCitation}
          />
        </div>
      )}
    </div>
  );
}
