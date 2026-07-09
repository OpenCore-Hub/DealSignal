import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Eye, Users, Clock, Calendar } from "@phosphor-icons/react";
import { StatCard } from "@/components/common/StatCard";
import { EmptyState } from "@/components/common/EmptyState";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { formatDuration, formatRelativeTime } from "@/lib/formatters";
import { calculateUniqueVisitors } from "@/lib/calculations";
import { LinkAccessLog } from "../LinkAccessLog";
import type { AccessLog, Link } from "@/types";

interface AnalyticsTabProps {
  link: Link;
  logs: AccessLog[];
}

const RECENT_LOG_LIMIT = 20;

export function AnalyticsTab({ link, logs }: AnalyticsTabProps) {
  const { t } = useTranslation("linkShare");

  const uniqueVisitors = useMemo(() => calculateUniqueVisitors(logs), [logs]);
  const recentLogs = useMemo(() => {
    return [...logs]
      .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
      .slice(0, RECENT_LOG_LIMIT);
  }, [logs]);

  const stats = [
    {
      label: t("analytics.views"),
      value: link.accessCount ?? 0,
      icon: <Eye size={18} />,
    },
    {
      label: t("analytics.uniqueVisitors"),
      value: uniqueVisitors,
      icon: <Users size={18} />,
    },
    {
      label: t("analytics.avgDuration"),
      value: formatDuration(link.avgDurationSeconds || 0),
      icon: <Clock size={18} />,
    },
    {
      label: t("analytics.lastVisit"),
      value: link.lastViewedAt ? formatRelativeTime(link.lastViewedAt) : "—",
      icon: <Calendar size={18} />,
    },
  ];

  return (
    <div className="space-y-6 py-2">
      <div className="grid grid-cols-2 gap-3">
        {stats.map((stat) => (
          <StatCard
            key={stat.label}
            label={stat.label}
            value={stat.value}
            icon={stat.icon}
            size="sm"
          />
        ))}
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-h3">{t("analytics.recentActivity")}</CardTitle>
        </CardHeader>
        <CardContent>
          {recentLogs.length === 0 ? (
            <EmptyState
              icon={<Eye size={48} />}
              title={t("analytics.emptyTitle")}
              description={t("analytics.emptyDescription")}
            />
          ) : (
            <div className="max-h-[320px] overflow-auto">
              <LinkAccessLog logs={recentLogs} />
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
