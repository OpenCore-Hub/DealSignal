import { Plus } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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
  const visitorList = toVisitorListItems(visitors);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-h2 flex items-center gap-2">
          <Plus size={20} />
          {t("documents:detail.recentVisitors")}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <VisitorList visitors={visitorList} />
      </CardContent>
    </Card>
  );
}
