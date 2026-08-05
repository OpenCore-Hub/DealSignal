import { Users } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { useLocation } from "react-router";
import { VisitorList } from "@/components/common/VisitorList";
import type { HeatLevel, VisitorSummary } from "@/types";

interface VisitorListItem {
  id: string;
  email: string;
  organization?: string;
  heatLevel: HeatLevel;
  visitCount: number;
  avgDurationSeconds: number;
  lastSeenAt: string;
}

function toVisitorListItems(visitors: VisitorSummary[]): VisitorListItem[] {
  const hotThreshold = 3;
  return visitors
    .map((v) => ({
      id: v.visitorId || v.visitorEmail || "unknown",
      email: v.visitorEmail || v.visitorId || "unknown",
      organization: undefined,
      heatLevel: (v.pageViewCount >= hotThreshold ? "hot" : v.pageViewCount >= 1 ? "warm" : "cold") as HeatLevel,
      visitCount: v.pageViewCount,
      avgDurationSeconds: Math.round(v.avgDurationSeconds),
      lastSeenAt: v.lastSeenAt,
    }))
    .sort((a, b) => new Date(b.lastSeenAt).getTime() - new Date(a.lastSeenAt).getTime())
    .slice(0, 10);
}

interface DocumentVisitorsCardProps {
  visitors: VisitorSummary[];
}

export function DocumentVisitorsCard({ visitors }: DocumentVisitorsCardProps) {
  const { t } = useTranslation("documents");
  const location = useLocation();
  const visitorList = toVisitorListItems(visitors);

  return (
    <section className="rounded-2xl border border-border/70 bg-background px-5 py-5">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Users size={16} className="text-muted-foreground" weight="duotone" />
          <h2 className="text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground/80">
            {t("documents:detail.recentVisitors")}
          </h2>
        </div>
        {visitorList.length > 0 ? (
          <span className="font-mono text-[11px] tabular-nums text-muted-foreground">
            {visitorList.length}
          </span>
        ) : null}
      </div>
      <VisitorList
        visitors={visitorList}
        returnTo={location.pathname + location.search}
        returnLabel={t("documents:detail.back")}
      />
    </section>
  );
}
