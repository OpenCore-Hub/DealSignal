import { useMemo, useState } from "react";
import { useLocation, useNavigate, useParams } from "react-router";
import { Crosshair, DownloadSimple, FileText, Link as LinkIcon } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { StatCard } from "@/components/common/StatCard";
import { HeatBadge } from "@/components/common/HeatBadge";
import { TrendChart } from "@/components/common/TrendChart";
import { EmptyState } from "@/components/common/EmptyState";
import { HeatBreakdownDialog } from "@/components/insights/HeatBreakdownDialog";
import {
  InsightsRangeControls,
  useInsightsRange,
} from "@/components/insights/InsightsRangeControls";
import { api, type InsightsOverview } from "@/lib/api";
import { exportInsightsDailyVisitsCsv } from "@/lib/exportInsightsDailyVisits";
import { useTranslation } from "react-i18next";
import { useAsyncData } from "@/hooks/useAsyncData";

function isLocalViewerUrl(raw: string): boolean {
  if (!raw) return false;
  if (raw.startsWith("/l/")) return false;
  try {
    const u = new URL(raw, "http://local.invalid");
    const host = u.hostname.toLowerCase();
    return host === "localhost" || host === "127.0.0.1" || host === "::1";
  } catch {
    return false;
  }
}

function linkPrimaryLabel(link: InsightsOverview["topLinks"][number], fallback: string): string {
  const title = link.title?.trim();
  if (title) return title;
  // Never promote localhost / raw host URLs as the primary label.
  if (isLocalViewerUrl(link.shortUrl)) {
    try {
      const u = new URL(link.shortUrl, "http://local.invalid");
      const path = u.pathname.replace(/\/$/, "");
      return path || fallback;
    } catch {
      return fallback;
    }
  }
  try {
    const u = new URL(link.shortUrl);
    const path = u.pathname.replace(/\/$/, "");
    return path || fallback;
  } catch {
    return fallback;
  }
}

function linkTooltip(link: InsightsOverview["topLinks"][number], label: string): string {
  if (!link.shortUrl || isLocalViewerUrl(link.shortUrl)) return label;
  return link.shortUrl;
}

function periodCompareLabel(
  current: number,
  previous: number,
  days: number,
  t: (key: string, opts?: Record<string, unknown>) => string,
): string {
  if (previous === 0 && current === 0) {
    return t("overview.compareFlat", { days });
  }
  if (previous === 0) {
    return t("overview.compareNew", { days });
  }
  const pct = Math.round(((current - previous) / previous) * 100);
  if (pct === 0) {
    return t("overview.compareFlat", { days });
  }
  if (pct > 0) {
    return t("overview.compareUp", { pct, days, previous });
  }
  return t("overview.compareDown", { pct: Math.abs(pct), days, previous });
}

function rateCompareLabel(
  current: number,
  previous: number,
  days: number,
  t: (key: string, opts?: Record<string, unknown>) => string,
): string {
  const curPct = Math.round(current * 100);
  const prevPct = Math.round(previous * 100);
  if (prevPct === 0 && curPct === 0) {
    return t("overview.compareFlat", { days });
  }
  if (prevPct === 0) {
    return t("overview.compareNew", { days });
  }
  const delta = curPct - prevPct;
  if (delta === 0) {
    return t("overview.compareFlat", { days });
  }
  if (delta > 0) {
    return t("overview.compareRateUp", { points: delta, days, previous: prevPct });
  }
  return t("overview.compareRateDown", { points: Math.abs(delta), days, previous: prevPct });
}

function formatCompletionPct(rate: number): string {
  return `${Math.round(rate * 100)}%`;
}

export function InsightsOverviewPage() {
  const { t } = useTranslation("insights");
  const { t: tc } = useTranslation("common");
  const { i18n } = useTranslation();
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const location = useLocation();
  const locale = i18n.language;
  const rangeCtl = useInsightsRange(7);
  const range = rangeCtl.range;
  const [heatExplain, setHeatExplain] = useState<{
    linkId: string;
    label: string;
  } | null>(null);

  const { data: overview, loading, error, refetch } = useAsyncData(
    () => api.getInsightsOverview(rangeCtl.apiParams),
    [rangeCtl.apiParams],
  );

  const trend = useMemo(() => {
    const series = overview?.dailyVisits ?? [];
    const labels = series.map((d) =>
      new Date(d.date).toLocaleDateString(locale, { month: "short", day: "numeric" }),
    );
    const data = series.map((d) => d.opens);
    const visitors = series.map((d) => d.uniqueVisitors);
    const periodOpens = overview?.periodOpens ?? data.reduce((sum, n) => sum + n, 0);
    const previousOpens = overview?.previousPeriodOpens ?? 0;
    const periodUV = overview?.periodUniqueVisitors ?? 0;
    const previousUV = overview?.previousPeriodUniqueVisitors ?? 0;
    const medianSeconds = overview?.periodMedianDurationSeconds ?? 0;
    const previousMedianSeconds = overview?.previousPeriodMedianDurationSeconds ?? 0;
    const pageViewCount = overview?.periodPageViewCount ?? 0;
    const completionRate = overview?.periodCompletionRate ?? 0;
    const previousCompletionRate = overview?.previousPeriodCompletionRate ?? 0;
    const completedSessions = overview?.periodCompletedSessions ?? 0;
    const measurableSessions = overview?.periodMeasurableSessions ?? 0;
    const sessionCount = overview?.periodSessionCount ?? 0;
    const days =
      overview?.rangeDays ?? (range.kind === "preset" ? range.days : series.length || 7);
    const rangeFrom = overview?.rangeFrom ?? (range.kind === "custom" ? range.from : undefined);
    const rangeTo = overview?.rangeTo ?? (range.kind === "custom" ? range.to : undefined);
    const custom = Boolean(overview?.rangeCustom ?? range.kind === "custom");
    return {
      labels,
      data,
      visitors,
      periodOpens,
      previousOpens,
      periodUV,
      previousUV,
      medianSeconds,
      previousMedianSeconds,
      pageViewCount,
      completionRate,
      previousCompletionRate,
      completedSessions,
      measurableSessions,
      sessionCount,
      days,
      rangeFrom,
      rangeTo,
      custom,
      // Trend / CSV are opens+UV series — session completion is a separate KPI.
      hasActivity: periodOpens > 0 || periodUV > 0,
    };
  }, [overview, locale, range]);

  const generatedLabel = useMemo(() => {
    if (!overview?.generatedAt) return null;
    const tms = Date.parse(overview.generatedAt);
    if (Number.isNaN(tms)) return null;
    return new Date(tms).toLocaleString(locale, {
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  }, [overview?.generatedAt, locale]);

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
        <p className="text-body text-muted-foreground">{error}</p>
        <Button onClick={refetch}>{tc("retry")}</Button>
      </div>
    );
  }

  if (loading || !overview) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-64" />
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <Skeleton className="h-24" />
          <Skeleton className="h-24" />
          <Skeleton className="h-24" />
        </div>
      </div>
    );
  }

  const linkScope = t("overview.tierScope", { count: overview.activeLinkCount });
  const activeLinks = overview.tierCounts.hot + overview.tierCounts.warm;
  const heatStripHint = `${t("overview.activeLinks")}: ${activeLinks} · ${t("overview.activeLinksHint")}`;

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
        <div className="space-y-1">
          <InsightsRangeControls
            range={rangeCtl.range}
            customOpen={rangeCtl.customOpen}
            draftFrom={rangeCtl.draftFrom}
            draftTo={rangeCtl.draftTo}
            rangeError={rangeCtl.rangeError}
            onSelectPreset={rangeCtl.selectPreset}
            onOpenCustom={rangeCtl.openCustom}
            onDraftFromChange={rangeCtl.setDraftFrom}
            onDraftToChange={rangeCtl.setDraftTo}
            onApplyCustom={() => {
              rangeCtl.applyCustom();
            }}
          />
          {generatedLabel ? (
            <p className="text-caption text-muted-foreground">
              {t("overview.updatedAt", { time: generatedLabel })}
            </p>
          ) : null}
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={!trend.hasActivity}
          onClick={() =>
            exportInsightsDailyVisitsCsv(
              overview.dailyVisits,
              trend.days,
              [
                t("overview.exportColDate"),
                t("overview.exportColOpens"),
                t("overview.exportColVisitors"),
              ],
              { from: trend.rangeFrom, to: trend.rangeTo },
            )
          }
        >
          <DownloadSimple size={16} className="mr-1.5" />
          {t("overview.exportCsv")}
        </Button>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label={t("overview.kpiOpens")}
          value={trend.periodOpens}
          subtext={periodCompareLabel(trend.periodOpens, trend.previousOpens, trend.days, t)}
        />
        <StatCard
          label={t("overview.kpiVisitors")}
          value={trend.periodUV}
          subtext={periodCompareLabel(trend.periodUV, trend.previousUV, trend.days, t)}
        />
        <StatCard
          label={t("overview.kpiMedianDwell")}
          value={
            trend.pageViewCount > 0
              ? t("overview.kpiMedianDwellValue", {
                  seconds: Math.round(trend.medianSeconds),
                })
              : t("overview.kpiEmpty")
          }
          subtext={
            trend.pageViewCount > 0
              ? `${t("overview.kpiMedianDwellHint")} · ${periodCompareLabel(
                  Math.round(trend.medianSeconds),
                  Math.round(trend.previousMedianSeconds),
                  trend.days,
                  t,
                )}`
              : t("overview.kpiMedianDwellEmpty")
          }
        />
        <StatCard
          label={t("overview.kpiCompletion")}
          value={
            trend.measurableSessions > 0
              ? formatCompletionPct(trend.completionRate)
              : t("overview.kpiEmpty")
          }
          subtext={
            trend.measurableSessions > 0
              ? `${t("overview.kpiCompletionHint", {
                  completed: trend.completedSessions,
                  measurable: trend.measurableSessions,
                })} · ${rateCompareLabel(
                  trend.completionRate,
                  trend.previousCompletionRate,
                  trend.days,
                  t,
                )}`
              : t("overview.kpiCompletionEmpty")
          }
        />
      </div>

      <div className="space-y-2">
        <p className="text-caption text-muted-foreground">{heatStripHint}</p>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <StatCard
            label={t("overview.hot")}
            value={overview.tierCounts.hot}
            subtext={linkScope}
            icon={<HeatBadge level="hot" />}
            size="sm"
          />
          <StatCard
            label={t("overview.warm")}
            value={overview.tierCounts.warm}
            subtext={linkScope}
            icon={<HeatBadge level="warm" />}
            size="sm"
          />
          <StatCard
            label={t("overview.cold")}
            value={overview.tierCounts.cold}
            subtext={linkScope}
            icon={<HeatBadge level="cold" />}
            size="sm"
          />
        </div>
      </div>

      <TrendChart
        title={
          trend.custom && trend.rangeFrom && trend.rangeTo
            ? t("overview.trendTitleCustom", { from: trend.rangeFrom, to: trend.rangeTo })
            : t("overview.trendTitle", { days: trend.days })
        }
        labels={trend.hasActivity ? trend.labels : undefined}
        data={trend.hasActivity ? trend.data : undefined}
        secondaryData={trend.hasActivity ? trend.visitors : undefined}
        emptyDescription={t("overview.trendEmpty")}
        primaryLegend={t("overview.legendOpens")}
        secondaryLegend={t("overview.legendVisitors")}
        formatValue={(v) => t("overview.trendOpens", { count: v })}
        formatSecondaryValue={(v) => t("overview.trendVisitors", { count: v })}
      />
      {trend.hasActivity && (
        <p className="-mt-4 text-caption text-muted-foreground">
          {trend.custom && trend.rangeFrom && trend.rangeTo
            ? t("overview.trendCaptionCustom", {
                opens: trend.periodOpens,
                visitors: trend.periodUV,
                from: trend.rangeFrom,
                to: trend.rangeTo,
              })
            : t("overview.trendCaption", {
                opens: trend.periodOpens,
                visitors: trend.periodUV,
                days: trend.days,
              })}
        </p>
      )}

      <p className="text-caption text-muted-foreground">{t("overview.topsLifetimeHint")}</p>
      {(() => {
        const accessDays = overview.eventRetentionDays ?? 0;
        const pageViewDays = overview.pageViewRetentionDays ?? 0;
        if (accessDays > 0 && pageViewDays > 0) {
          return (
            <p className="text-caption text-muted-foreground">
              {t("overview.retentionHint", {
                accessDays,
                pageViewDays,
              })}
            </p>
          );
        }
        if (accessDays > 0) {
          return (
            <p className="text-caption text-muted-foreground">
              {t("overview.retentionHintOpensOnly", { days: accessDays })}
            </p>
          );
        }
        if (pageViewDays > 0) {
          return (
            <p className="text-caption text-muted-foreground">
              {t("overview.retentionHintPageViewsOnly", { days: pageViewDays })}
            </p>
          );
        }
        return null;
      })()}

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="text-h2 flex items-center gap-2">
              <FileText size={20} />
              {t("overview.topDocuments")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            {overview.topDocuments.length === 0 ? (
              <EmptyState
                icon={<FileText size={32} />}
                title={t("overview.topDocumentsEmptyTitle")}
                description={t("overview.topDocumentsEmptyDescription")}
                size="compact"
              />
            ) : (
              <ul className="space-y-2">
                {overview.topDocuments.map((doc) => {
                  const handleClick = () =>
                    navigate(`/${workspaceSlug}/documents/${doc.id}`, {
                      state: {
                        returnTo: location.pathname + location.search,
                        returnLabel: tc("back"),
                      },
                    });
                  return (
                    <li
                      key={doc.id}
                      className="flex items-center justify-between gap-3 rounded-md border border-border p-3 transition-colors hover:bg-muted"
                    >
                      <button
                        type="button"
                        className="min-w-0 flex-1 truncate text-left text-sm font-medium"
                        onClick={handleClick}
                      >
                        {doc.title}
                      </button>
                      <div className="flex shrink-0 items-center gap-2">
                        <span className="text-caption tabular-nums text-muted-foreground">
                          {t("overview.views", { count: doc.views })}
                        </span>
                        {doc.score != null ? (
                          <span className="text-caption tabular-nums text-muted-foreground">
                            {t("heatBreakdown.score", { score: doc.score })}
                          </span>
                        ) : null}
                        <HeatBadge level={doc.heatLevel} />
                        {doc.primaryLinkId ? (
                          <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            onClick={(e) => {
                              e.stopPropagation();
                              setHeatExplain({
                                linkId: doc.primaryLinkId!,
                                label: doc.title,
                              });
                            }}
                          >
                            {t("heatBreakdown.explain")}
                          </Button>
                        ) : null}
                      </div>
                    </li>
                  );
                })}
              </ul>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-h2 flex items-center gap-2">
              <LinkIcon size={20} />
              {t("overview.topLinks")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            {overview.topLinks.length === 0 ? (
              <EmptyState
                icon={<LinkIcon size={32} />}
                title={t("overview.topLinksEmptyTitle")}
                description={t("overview.topLinksEmptyDescription")}
                size="compact"
              />
            ) : (
              <ul className="space-y-2">
                {overview.topLinks.map((link) => {
                  const handleClick = () =>
                    navigate(`/${workspaceSlug}/links/${link.id}`, {
                      state: {
                        returnTo: location.pathname + location.search,
                        returnLabel: tc("back"),
                      },
                    });
                  const label = linkPrimaryLabel(link, t("overview.untitledLink"));
                  return (
                    <li
                      key={link.id}
                      className="flex items-center justify-between gap-3 rounded-md border border-border p-3 transition-colors hover:bg-muted"
                    >
                      <button
                        type="button"
                        className="min-w-0 flex-1 truncate text-left text-sm font-medium"
                        title={linkTooltip(link, label)}
                        onClick={handleClick}
                      >
                        {label}
                      </button>
                      <div className="flex shrink-0 items-center gap-2">
                        <span className="text-caption tabular-nums text-muted-foreground">
                          {t("overview.views", { count: link.views })}
                        </span>
                        {link.score != null ? (
                          <span className="text-caption tabular-nums text-muted-foreground">
                            {t("heatBreakdown.score", { score: link.score })}
                          </span>
                        ) : null}
                        <HeatBadge level={link.heatLevel} />
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          onClick={(e) => {
                            e.stopPropagation();
                            setHeatExplain({ linkId: link.id, label });
                          }}
                        >
                          {t("heatBreakdown.explain")}
                        </Button>
                      </div>
                    </li>
                  );
                })}
              </ul>
            )}
          </CardContent>
        </Card>
      </div>

      <HeatBreakdownDialog
        open={heatExplain != null}
        onOpenChange={(open) => {
          if (!open) setHeatExplain(null);
        }}
        linkId={heatExplain?.linkId ?? null}
        linkLabel={heatExplain?.label ?? ""}
      />

      <Card data-testid="deal-radar-cta">
        <CardHeader className="flex flex-row items-start justify-between gap-3 space-y-0">
          <div className="space-y-1">
            <CardTitle className="text-h2 flex items-center gap-2">
              <Crosshair size={20} />
              {t("overview.radarCtaTitle")}
            </CardTitle>
            <p className="text-caption text-muted-foreground">
              {t("overview.radarCtaDescription")}
            </p>
          </div>
          <Button
            variant="default"
            size="sm"
            className="shrink-0"
            onClick={() => navigate(`/${workspaceSlug}/dashboard`)}
          >
            {t("overview.openDealRadar")}
          </Button>
        </CardHeader>
        {(overview.openSignalCount ?? 0) > 0 ? (
          <CardContent className="pt-0">
            <p className="text-sm text-muted-foreground">
              {t("overview.radarSignalsCount", { count: overview.openSignalCount })}
            </p>
          </CardContent>
        ) : null}
      </Card>
    </div>
  );
}
