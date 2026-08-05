import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import { formatDuration } from "@/lib/formatters";
import type { HeatLevel, Link, VisitorSummary } from "@/types";

interface DocumentStatsProps {
  links: Link[];
  visitors: VisitorSummary[];
  className?: string;
}

export function DocumentStats({ links, visitors, className }: DocumentStatsProps) {
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

  const metrics = [
    { label: t("documents:detail.totalViews"), value: String(totalViews) },
    { label: t("documents:detail.uniqueVisitors"), value: String(uniqueVisitors) },
    { label: t("documents:detail.avgDuration"), value: formatDuration(avgDuration) },
  ];

  return (
    <section
      className={cn(
        "overflow-hidden rounded-2xl border border-border/70 bg-background",
        "shadow-[0_1px_0_rgba(15,23,42,0.04)]",
        className,
      )}
    >
      <div className="grid grid-cols-2 divide-x divide-y divide-border/70 md:grid-cols-4 md:divide-y-0">
        {metrics.map((metric) => (
          <div key={metric.label} className="px-5 py-4 md:py-5">
            <p className="text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground/80">
              {metric.label}
            </p>
            <p className="mt-2 font-mono text-[1.75rem] leading-none tracking-tight tabular-nums text-foreground">
              {metric.value}
            </p>
          </div>
        ))}
        <div className="col-span-2 px-5 py-4 md:col-span-1 md:py-5">
          <p className="text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground/80">
            {t("documents:detail.heatDistribution")}
          </p>
          <div className="mt-3 flex flex-wrap gap-1.5">
            {heatDistribution.hot > 0 && (
              <span className="inline-flex items-center gap-1.5 rounded-md bg-hot-500/10 px-2 py-1 text-xs font-medium text-hot-500">
                <span className="size-1.5 rounded-full bg-hot-500" />
                {t("documents:detail.heatHot", { count: heatDistribution.hot })}
              </span>
            )}
            {heatDistribution.warm > 0 && (
              <span className="inline-flex items-center gap-1.5 rounded-md bg-warm-500/10 px-2 py-1 text-xs font-medium text-warm-500">
                <span className="size-1.5 rounded-full bg-warm-500" />
                {t("documents:detail.heatWarm", { count: heatDistribution.warm })}
              </span>
            )}
            {heatDistribution.cold > 0 && (
              <span className="inline-flex items-center gap-1.5 rounded-md bg-cold-500/10 px-2 py-1 text-xs font-medium text-cold-500">
                <span className="size-1.5 rounded-full bg-cold-500" />
                {t("documents:detail.heatCold", { count: heatDistribution.cold })}
              </span>
            )}
            {links.length === 0 && (
              <p className="text-sm text-muted-foreground">{t("documents:detail.noLinks")}</p>
            )}
          </div>
        </div>
      </div>
    </section>
  );
}
