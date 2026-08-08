import { ChartLine, Flag, Users } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { StatCard } from "@/components/common/StatCard";
import type { DocumentReadingFunnel } from "@/lib/api";
import { cn } from "@/lib/utils";

interface ReadingFunnelCardProps {
  funnel: DocumentReadingFunnel | null;
  loading?: boolean;
  onOpenPage?: (pageNumber: number) => void;
}

function formatPct(rate: number): string {
  return `${Math.round(rate * 100)}%`;
}

export function ReadingFunnelCard({ funnel, loading, onOpenPage }: ReadingFunnelCardProps) {
  const { t } = useTranslation("insights");

  if (loading) {
    return <Skeleton className="h-64 w-full" data-testid="reading-funnel-loading" />;
  }

  if (!funnel || funnel.sessionCount === 0) {
    return (
      <Card data-testid="reading-funnel-empty">
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <ChartLine size={20} />
            {t("funnel.title")}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-body text-muted-foreground">{t("funnel.empty")}</p>
        </CardContent>
      </Card>
    );
  }

  const maxReached = Math.max(1, ...funnel.steps.map((s) => s.visitorsReached));
  const dropPage = funnel.biggestDropOffPage;

  return (
    <Card data-testid="reading-funnel">
      <CardHeader className="space-y-1">
        <CardTitle className="text-h2 flex items-center gap-2">
          <ChartLine size={20} />
          {t("funnel.title")}
        </CardTitle>
        <p className="text-caption text-muted-foreground">{t("funnel.subtitle")}</p>
      </CardHeader>
      <CardContent className="space-y-5">
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <StatCard
            size="sm"
            icon={<Users size={16} />}
            label={t("funnel.sessions")}
            value={funnel.sessionCount}
            subtext={t("funnel.sessionsHint")}
          />
          <StatCard
            size="sm"
            icon={<Flag size={16} />}
            label={t("funnel.completion")}
            value={formatPct(funnel.completionRate)}
            subtext={t("funnel.completionHint", {
              completed: funnel.completedSessions,
              total: funnel.sessionCount,
            })}
          />
          <StatCard
            size="sm"
            label={t("funnel.medianDepth")}
            value={t("funnel.medianDepthValue", {
              page: Number(funnel.medianMaxPage.toFixed(1)),
              pages: funnel.pageCount,
            })}
            subtext={t("funnel.avgPages", {
              pages: Number(funnel.avgPagesPerSession.toFixed(1)),
            })}
          />
        </div>

        {dropPage > 0 ? (
          <p className="text-caption text-muted-foreground" data-testid="reading-funnel-drop">
            {t("funnel.biggestDrop", { page: dropPage })}
          </p>
        ) : null}

        <ul className="max-h-80 space-y-2 overflow-y-auto pr-1">
          {funnel.steps.map((step) => {
            const widthPct = Math.round((step.visitorsReached / maxReached) * 100);
            const isDrop = step.pageNumber === dropPage;
            const label = (
              <span className="w-16 shrink-0 text-caption tabular-nums text-muted-foreground">
                {t("funnel.pageLabel", { page: step.pageNumber })}
              </span>
            );
            return (
              <li key={step.pageNumber} className="flex items-center gap-3">
                {onOpenPage ? (
                  <button
                    type="button"
                    className="w-16 shrink-0 text-left text-caption tabular-nums text-muted-foreground underline-offset-4 hover:underline"
                    onClick={() => onOpenPage(step.pageNumber)}
                  >
                    {t("funnel.pageLabel", { page: step.pageNumber })}
                  </button>
                ) : (
                  label
                )}
                <div className="h-2 min-w-0 flex-1 overflow-hidden rounded-full bg-muted">
                  <div
                    className={cn(
                      "h-full rounded-full",
                      isDrop ? "bg-risk-500/80" : "bg-primary/70",
                    )}
                    style={{ width: `${widthPct}%` }}
                  />
                </div>
                <span className="w-20 shrink-0 text-right text-caption tabular-nums text-muted-foreground">
                  {step.visitorsReached}
                  {step.dropOffFromPrev > 0
                    ? ` · −${Math.round(step.dropOffFromPrev * 100)}%`
                    : ""}
                </span>
              </li>
            );
          })}
        </ul>
      </CardContent>
    </Card>
  );
}
