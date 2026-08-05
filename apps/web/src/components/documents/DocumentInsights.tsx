import { ChartLineUp, DoorOpen, Path } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { EmptyState } from "@/components/common/EmptyState";
import { cn } from "@/lib/utils";
import {
  projectDocumentInsights,
  type DocumentInsight,
} from "@/lib/projectDocumentInsights";
import type { PageAnalytics } from "@/types";

interface DocumentInsightsProps {
  analytics: PageAnalytics[];
  onOpenPage?: (pageNumber: number) => void;
  className?: string;
}

function insightIcon(kind: DocumentInsight["kind"]) {
  switch (kind) {
    case "top_dwell":
      return ChartLineUp;
    case "exit_risk":
      return DoorOpen;
    case "sparse":
      return Path;
  }
}

export function DocumentInsights({
  analytics,
  onOpenPage,
  className,
}: DocumentInsightsProps) {
  const { t } = useTranslation("documents");
  const insights = projectDocumentInsights(analytics);

  if (analytics.length === 0 || insights.length === 0) {
    return (
      <section
        className={cn("rounded-2xl border border-border/70 bg-background px-5 py-5", className)}
        data-testid="document-insights"
      >
        <h2 className="text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground/80">
          {t("documents:insights.title")}
        </h2>
        <div className="mt-4">
          <EmptyState
            icon={<ChartLineUp size={40} />}
            title={t("documents:insights.emptyTitle")}
            description={t("documents:insights.emptyDescription")}
          />
        </div>
      </section>
    );
  }

  return (
    <section
      className={cn("rounded-2xl border border-border/70 bg-background px-5 py-5", className)}
      data-testid="document-insights"
      data-count={insights.length}
    >
      <div className="space-y-1">
        <h2 className="text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground/80">
          {t("documents:insights.title")}
        </h2>
        <p className="text-[12px] text-muted-foreground">
          {t("documents:insights.subtitle")}
        </p>
      </div>

      <ul className="mt-5 divide-y divide-border/50">
        {insights.map((insight) => {
          const Icon = insightIcon(insight.kind);
          const openable =
            Boolean(onOpenPage) &&
            (insight.kind === "top_dwell" || insight.kind === "exit_risk");
          const pageNumber =
            insight.kind === "top_dwell" || insight.kind === "exit_risk"
              ? insight.pageNumber
              : null;

          const title =
            insight.kind === "top_dwell"
              ? t("documents:insights.topPage", { pageNumber: insight.pageNumber })
              : insight.kind === "exit_risk"
                ? t("documents:insights.exitRisk", { pageNumber: insight.pageNumber })
                : t("documents:insights.sparse", {
                    engaged: insight.engagedCount,
                    total: insight.totalPages,
                  });

          const description =
            insight.kind === "top_dwell"
              ? t("documents:insights.topPageDescription", {
                  seconds: insight.avgDurationSeconds,
                })
              : insight.kind === "exit_risk"
                ? t("documents:insights.exitRiskDescription", {
                    percent: Math.round(insight.exitRate * 100),
                  })
                : t("documents:insights.sparseDescription", {
                    count: insight.zeroCount,
                  });

          const body = (
            <>
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-foreground/[0.04] text-foreground">
                <Icon size={18} />
              </div>
              <div className="min-w-0 flex-1">
                <p className="text-[13px] font-medium text-foreground">{title}</p>
                <p className="mt-1 text-[12px] leading-relaxed text-muted-foreground">
                  {description}
                </p>
              </div>
            </>
          );

          return (
            <li key={insight.key} className="py-4 first:pt-0 last:pb-0">
              {openable && pageNumber != null ? (
                <button
                  type="button"
                  className="flex w-full items-start gap-3 rounded-lg text-left transition-colors hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  data-testid={`document-insight-${insight.kind}-${pageNumber}`}
                  onClick={() => onOpenPage?.(pageNumber)}
                >
                  {body}
                </button>
              ) : (
                <div className="flex items-start gap-3">{body}</div>
              )}
            </li>
          );
        })}
      </ul>
    </section>
  );
}
