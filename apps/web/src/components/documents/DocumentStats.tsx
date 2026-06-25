import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { StatCard } from "@/components/common/StatCard";
import { formatDuration } from "@/lib/formatters";
import type { HeatLevel, Link, VisitorSummary } from "@/types";

interface DocumentStatsProps {
  links: Link[];
  visitors: VisitorSummary[];
}

export function DocumentStats({ links, visitors }: DocumentStatsProps) {
  const { t } = useTranslation(["documents", "common"]);

  const totalViews = links.reduce((sum, l) => sum + l.accessCount, 0);
  const uniqueVisitors = visitors.length;
  const avgDuration =
    links.length > 0
      ? Math.round(links.reduce((sum, l) => sum + (l.avgDurationSeconds || 0), 0) / links.length)
      : 0;

  const heatDistribution = useMemo(() => {
    const counts = { hot: 0, warm: 0, cold: 0 } as Record<HeatLevel, number>;
    for (const link of links) {
      counts[link.heatLevel] = (counts[link.heatLevel] ?? 0) + 1;
    }
    return counts;
  }, [links]);

  return (
    <div className="space-y-4">
      <StatCard label={t("documents:detail.totalViews")} value={totalViews} />
      <StatCard label={t("documents:detail.uniqueVisitors")} value={uniqueVisitors} />
      <StatCard label={t("documents:detail.avgDuration")} value={formatDuration(avgDuration)} />
      <Card>
        <CardHeader>
          <CardTitle className="text-h3">{t("documents:detail.heatDistribution")}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap gap-2">
            {heatDistribution.hot > 0 && (
              <div className="flex items-center gap-1.5 rounded-full bg-hot-500/10 px-2 py-1 text-xs font-medium text-hot-500">
                <span className="h-1.5 w-1.5 rounded-full bg-hot-500" />
                {t("documents:detail.heatHot", { count: heatDistribution.hot })}
              </div>
            )}
            {heatDistribution.warm > 0 && (
              <div className="flex items-center gap-1.5 rounded-full bg-warm-500/10 px-2 py-1 text-xs font-medium text-warm-500">
                <span className="h-1.5 w-1.5 rounded-full bg-warm-500" />
                {t("documents:detail.heatWarm", { count: heatDistribution.warm })}
              </div>
            )}
            {heatDistribution.cold > 0 && (
              <div className="flex items-center gap-1.5 rounded-full bg-cold-500/10 px-2 py-1 text-xs font-medium text-cold-500">
                <span className="h-1.5 w-1.5 rounded-full bg-cold-500" />
                {t("documents:detail.heatCold", { count: heatDistribution.cold })}
              </div>
            )}
            {links.length === 0 && (
              <p className="text-sm text-muted-foreground">{t("documents:detail.noLinks")}</p>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
