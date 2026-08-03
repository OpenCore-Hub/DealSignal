import { useMemo } from "react";
import { ChartLineUp, Users, Link as LinkIcon, FileText } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { StatCard } from "@/components/common/StatCard";
import { TrendChart } from "@/components/common/TrendChart";
import { AskSecurityEventsPanel } from "@/components/links/share/AskSecurityEventsPanel";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { formatRelativeTime } from "@/lib/formatters";
import { useAsyncData } from "@/hooks/useAsyncData";
import type { DealRoomAnalytics, Link } from "@/types";

interface DealRoomAnalyticsTabProps {
  roomId: string;
  links?: Link[];
}

/** Build a continuous last-30-days series from sparse (or empty) daily buckets. */
export function buildDealRoomTrendSeries(
  viewsOverTime: DealRoomAnalytics["viewsOverTime"],
  now = new Date(),
): { data: number[]; labels: string[] } {
  const byDay = new Map(viewsOverTime.map((d) => [d.day, d.views]));
  const data: number[] = [];
  const labels: string[] = [];
  for (let i = 29; i >= 0; i -= 1) {
    const day = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate() - i));
    const key = day.toISOString().slice(0, 10);
    labels.push(key);
    data.push(byDay.get(key) ?? 0);
  }
  return { data, labels };
}

export function DealRoomAnalyticsTab({ roomId, links }: DealRoomAnalyticsTabProps) {
  const { t } = useTranslation("dealRooms");
  const { data, loading, error, refetch } = useAsyncData(
    () => api.getDealRoomAnalytics(roomId),
    [roomId],
  );

  const trend = useMemo(
    () => buildDealRoomTrendSeries(data?.viewsOverTime ?? []),
    [data?.viewsOverTime],
  );
  // Empty chart only when the room has no views at all. If lifetime views exist
  // but the last-30-day window is all zeros, still show the zero-filled window.
  const trendEmpty =
    !data || (data.totalViews === 0 && trend.data.every((v) => v === 0));

  if (loading && !data) {
    return (
      <div className="rounded-lg border border-border px-4 py-10 text-center text-sm text-muted-foreground">
        {t("common:loading")}
      </div>
    );
  }

  if (error && !data) {
    return (
      <div
        className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-6 text-center"
        role="alert"
      >
        <p className="text-sm text-destructive">{t("analytics.loadFailed")}</p>
        <Button size="sm" variant="outline" className="mt-3" onClick={() => void refetch()}>
          {t("common:retry")}
        </Button>
      </div>
    );
  }

  const analytics = data!;
  const recentVisitors = analytics.recentVisitors ?? [];

  return (
    <div className="space-y-4" data-testid="deal-room-analytics-tab">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label={t("analytics.views")}
          value={analytics.totalViews}
          icon={<ChartLineUp size={18} />}
        />
        <StatCard
          label={t("analytics.activeLinks")}
          value={analytics.activeLinkCount}
          icon={<LinkIcon size={18} />}
        />
        <StatCard
          label={t("activity.documents")}
          value={analytics.documentCount}
          icon={<FileText size={18} />}
        />
        <StatCard
          label={t("analytics.uniqueVisitors")}
          value={analytics.uniqueVisitors}
          icon={<Users size={18} />}
        />
      </div>

      <TrendChart
        title={t("analytics.trend")}
        data={trendEmpty ? [] : trend.data}
        labels={trendEmpty ? [] : trend.labels}
        emptyTitle={t("analytics.trendEmptyTitle")}
        emptyDescription={t("analytics.trendEmpty")}
      />

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-h3">{t("analytics.recentVisitors")}</CardTitle>
        </CardHeader>
        <CardContent>
          {recentVisitors.length > 0 ? (
            <ul className="divide-y divide-border rounded-lg border border-border">
              {recentVisitors.map((visitor) => {
                const label = visitor.visitorEmail?.trim() || visitor.visitorId;
                return (
                  <li
                    key={visitor.visitorId}
                    className="flex items-center justify-between gap-3 px-3 py-2.5"
                    data-testid={`deal-room-analytics-visitor-${visitor.visitorId}`}
                  >
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">{label}</p>
                      <p className="text-caption text-muted-foreground">
                        {t("analytics.visitorViews", { count: visitor.totalViews })}
                        {" · "}
                        {t("analytics.visitorLastSeen", {
                          time: formatRelativeTime(visitor.lastAccessAt),
                        })}
                      </p>
                    </div>
                  </li>
                );
              })}
            </ul>
          ) : (
            <p className="text-body text-muted-foreground">{t("activity.noVisitors")}</p>
          )}
        </CardContent>
      </Card>

      <AskSecurityEventsPanel
        mode="room"
        roomId={roomId}
        links={(links ?? []).map((l) => ({ id: l.id, name: l.name || l.documentTitle }))}
      />
    </div>
  );
}
