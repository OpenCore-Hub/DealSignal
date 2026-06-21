import { useEffect, useState } from "react";
import { Fire, Link as LinkIcon, Users, FileText } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { StatCard } from "@/components/common/StatCard";
import { TrendChart } from "@/components/common/TrendChart";
import { HeatBadge } from "@/components/common/HeatBadge";
import { api } from "@/lib/api";
import type { InsightsOverview as InsightsOverviewType } from "@/lib/api";

export function InsightsOverview() {
  const [data, setData] = useState<InsightsOverviewType | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const timer = setTimeout(() => {
      api.getInsightsOverview().then((d) => {
        setData(d);
        setLoading(false);
      });
    }, 600);
    return () => clearTimeout(timer);
  }, []);

  if (loading || !data) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-24" />
        <Skeleton className="h-48" />
        <Skeleton className="h-48" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <StatCard label="高热度" value={data.tierCounts.hot} icon={<Fire size={20} />} />
        <StatCard label="中热度" value={data.tierCounts.warm} icon={<LinkIcon size={20} />} />
        <StatCard label="低热度" value={data.tierCounts.cold} icon={<Users size={20} />} />
      </div>

      <TrendChart title="访问量趋势" />

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-h2 flex items-center gap-2">
              <FileText size={20} />
              热度最高文档
            </CardTitle>
          </CardHeader>
          <CardContent>
            <ul className="space-y-2">
              {data.topDocuments.map((doc) => (
                <li
                  key={doc.id}
                  className="flex items-center justify-between rounded-md border border-border p-3"
                >
                  <span className="truncate text-sm font-medium">{doc.title}</span>
                  <div className="flex items-center gap-2">
                    <span className="text-caption text-muted-foreground">
                      {doc.views} views
                    </span>
                    <HeatBadge level={doc.heatLevel} />
                  </div>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-h2 flex items-center gap-2">
              <Users size={20} />
              热度最高访客
            </CardTitle>
          </CardHeader>
          <CardContent>
            <ul className="space-y-2">
              {data.topContacts.map((contact) => (
                <li
                  key={contact.id}
                  className="flex items-center justify-between rounded-md border border-border p-3"
                >
                  <span className="truncate text-sm font-medium">{contact.email}</span>
                  <div className="flex items-center gap-2">
                    <span className="text-caption text-muted-foreground">
                      评分 {contact.score}
                    </span>
                    <HeatBadge level={contact.heatLevel} />
                  </div>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
