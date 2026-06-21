import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { FileText, Link as LinkIcon, Users } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { StatCard } from "@/components/common/StatCard";
import { HeatBadge } from "@/components/common/HeatBadge";
import { TrendChart } from "@/components/common/TrendChart";
import { api, type InsightsOverview } from "@/lib/api";
import type { AccessLog } from "@/types";

function buildLast7DaysTrend(logs: AccessLog[]): { labels: string[]; data: number[] } {
  const days: { label: string; date: Date; count: number }[] = [];
  for (let i = 6; i >= 0; i--) {
    const d = new Date();
    d.setDate(d.getDate() - i);
    days.push({
      label: d.toLocaleDateString("zh-CN", { month: "short", day: "numeric" }),
      date: new Date(d.getFullYear(), d.getMonth(), d.getDate()),
      count: 0,
    });
  }
  for (const log of logs) {
    const t = new Date(log.timestamp);
    const day = days.find((d) => t.getFullYear() === d.date.getFullYear() && t.getMonth() === d.date.getMonth() && t.getDate() === d.date.getDate());
    if (day) day.count += 1;
  }
  return { labels: days.map((d) => d.label), data: days.map((d) => d.count) };
}

export function InsightsOverviewPage() {
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const [data, setData] = useState<InsightsOverview | null>(null);
  const [logs, setLogs] = useState<AccessLog[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        setLoading(true);
        const overview = await api.getInsightsOverview();
        const linksRes = await api.getLinks();
        const allLogs = await Promise.all(
          linksRes.data.map((link) => api.getAccessLogs(link.id).then((r) => r.data))
        );
        if (!cancelled) {
          setData(overview);
          setLogs(allLogs.flat());
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, []);

  const trend = useMemo(() => buildLast7DaysTrend(logs), [logs]);

  if (loading || !data) {
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

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label="高热度" value={data.tierCounts.hot} icon={<HeatBadge level="hot" />} />
        <StatCard label="中热度" value={data.tierCounts.warm} icon={<HeatBadge level="warm" />} />
        <StatCard label="低热度" value={data.tierCounts.cold} icon={<HeatBadge level="cold" />} />
        <StatCard label="热度信号" value={data.tierCounts.hot + data.tierCounts.warm} />
      </div>

      <TrendChart
        title="近 7 天访问量趋势"
        labels={trend.labels}
        data={trend.data}
        emptyDescription="暂无访问数据，趋势将在链接被访问后自动生成。"
      />

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="text-h2 flex items-center gap-2">
              <FileText size={20} />
              热门文档
            </CardTitle>
          </CardHeader>
          <CardContent>
            <ul className="space-y-2">
              {data.topDocuments.map((doc) => (
                <li
                  key={doc.id}
                  className="flex cursor-pointer items-center justify-between rounded-md border border-border p-3 transition-colors hover:bg-muted"
                  onClick={() => navigate(`/${workspaceSlug}/documents/${doc.id}`)}
                >
                  <span className="truncate text-sm font-medium">{doc.title}</span>
                  <div className="flex items-center gap-3">
                    <span className="text-caption tabular-nums text-muted-foreground">{doc.views} views</span>
                    <HeatBadge level={doc.heatLevel} />
                  </div>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-h2 flex items-center gap-2">
              <LinkIcon size={20} />
              热门链接
            </CardTitle>
          </CardHeader>
          <CardContent>
            <ul className="space-y-2">
              {data.topLinks.map((link) => (
                <li
                  key={link.id}
                  className="flex cursor-pointer items-center justify-between rounded-md border border-border p-3 transition-colors hover:bg-muted"
                  onClick={() => navigate(`/${workspaceSlug}/links/${link.id}`)}
                >
                  <span className="truncate text-sm font-medium">{link.shortUrl}</span>
                  <div className="flex items-center gap-3">
                    <span className="text-caption tabular-nums text-muted-foreground">{link.views} views</span>
                    <HeatBadge level={link.heatLevel} />
                  </div>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <Users size={20} />
            高意向访问者
          </CardTitle>
        </CardHeader>
        <CardContent>
          <ul className="space-y-2">
            {data.topContacts.map((contact) => (
              <li
                key={contact.id}
                className="flex cursor-pointer items-center justify-between rounded-md border border-border p-3 transition-colors hover:bg-muted"
                onClick={() => navigate(`/${workspaceSlug}/contacts/${contact.id}`)}
              >
                <span className="truncate text-sm font-medium">{contact.email}</span>
                <div className="flex items-center gap-3">
                  <span className="text-caption tabular-nums text-muted-foreground">{contact.score} 分</span>
                  <HeatBadge level={contact.heatLevel} />
                </div>
              </li>
            ))}
          </ul>
        </CardContent>
      </Card>
    </div>
  );
}
