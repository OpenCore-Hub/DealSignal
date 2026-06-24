import { Plus } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { VisitorList } from "@/components/common/VisitorList";
import type { HeatLevel, AccessLog } from "@/types";

interface VisitorSummary {
  id: string;
  email: string;
  organization?: string;
  heatLevel: HeatLevel;
  visitCount: number;
  avgDurationSeconds: number;
  lastSeenAt: string;
}

function aggregateVisitors(logs: AccessLog[]): VisitorSummary[] {
  const byEmail = new Map<string, { duration: number; count: number; lastSeen: string; name?: string }>();
  for (const log of logs) {
    const email = log.visitorEmail || "unknown";
    const existing = byEmail.get(email);
    const timestamp = new Date(log.timestamp).toISOString();
    if (existing) {
      existing.count += 1;
      existing.duration += log.durationSeconds || 0;
      if (timestamp > existing.lastSeen) {
        existing.lastSeen = timestamp;
        if (log.visitorName) existing.name = log.visitorName;
      }
    } else {
      byEmail.set(email, {
        count: 1,
        duration: log.durationSeconds || 0,
        lastSeen: timestamp,
        name: log.visitorName,
      });
    }
  }

  const hotThreshold = 3;
  return Array.from(byEmail.entries())
    .map(([email, v], index) => ({
      id: `${email}-${index}`,
      email: v.name && v.name !== email ? `${v.name} <${email}>` : email,
      organization: undefined,
      heatLevel: (v.count >= hotThreshold ? "hot" : v.count >= 1 ? "warm" : "cold") as HeatLevel,
      visitCount: v.count,
      avgDurationSeconds: Math.round(v.duration / v.count),
      lastSeenAt: v.lastSeen,
    }))
    .sort((a, b) => new Date(b.lastSeenAt).getTime() - new Date(a.lastSeenAt).getTime())
    .slice(0, 10);
}

interface DocumentVisitorsCardProps {
  logs: AccessLog[];
}

export function DocumentVisitorsCard({ logs }: DocumentVisitorsCardProps) {
  const { t } = useTranslation("documents");
  const visitors = aggregateVisitors(logs);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-h2 flex items-center gap-2">
          <Plus size={20} />
          {t("documents:detail.recentVisitors")}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <VisitorList visitors={visitors} />
      </CardContent>
    </Card>
  );
}
