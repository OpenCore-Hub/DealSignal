import { ChartLineUp, Users, Link as LinkIcon, FileText } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { StatCard } from "@/components/common/StatCard";
import { TrendChart } from "@/components/common/TrendChart";
import { VisitorList } from "@/components/common/VisitorList";
import { useTranslation } from "react-i18next";
import { useLocation } from "react-router";
import type { HeatLevel } from "@/types";

interface DealRoomAnalyticsTabProps {
  documentCount: number;
  viewCount?: number;
  activeLinkCount?: number;
  recentVisitors?: { email: string; name?: string; heatLevel: HeatLevel; lastSeenAt: string }[];
}

export function DealRoomAnalyticsTab({
  documentCount,
  viewCount,
  activeLinkCount,
  recentVisitors,
}: DealRoomAnalyticsTabProps) {
  const { t } = useTranslation("dealRooms");
  const location = useLocation();

  const visitors = (recentVisitors ?? []).map((v, idx) => ({
    id: `${v.email}-${idx}`,
    email: v.email,
    name: v.name,
    heatLevel: v.heatLevel,
    visitCount: 0,
    avgDurationSeconds: 0,
    lastSeenAt: v.lastSeenAt,
  }));

  const trendData: number[] = [];

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label={t("analytics.views")} value={viewCount ?? 0} icon={<ChartLineUp size={18} />} />
        <StatCard label={t("analytics.activeLinks")} value={activeLinkCount ?? 0} icon={<LinkIcon size={18} />} />
        <StatCard label={t("activity.documents")} value={documentCount} icon={<FileText size={18} />} />
        <StatCard label={t("analytics.recentVisitors")} value={visitors.length} icon={<Users size={18} />} />
      </div>

      <TrendChart
        title={t("analytics.trend")}
        data={trendData}
        emptyDescription={t("analytics.comingSoon")}
      />

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-h3">{t("analytics.recentVisitors")}</CardTitle>
        </CardHeader>
        <CardContent>
          {visitors.length > 0 ? (
            <VisitorList
              visitors={visitors}
              returnTo={location.pathname + location.search}
              returnLabel={t("detail.back")}
            />
          ) : (
            <p className="text-body text-muted-foreground">{t("activity.noVisitors")}</p>
          )}
        </CardContent>
      </Card>

      <p className="text-caption text-muted-foreground">{t("analytics.comingSoon")}</p>
    </div>
  );
}
