import { useEffect, useMemo, useState } from "react";
import { useParams } from "react-router";
import { Copy, PencilSimple, ToggleRight } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { PageHeader } from "@/components/common/PageHeader";
import { BackButton } from "@/components/common/BackButton";
import { DetailLayout } from "@/components/common/DetailLayout";
import { StatCard } from "@/components/common/StatCard";
import { ActivityTimeline } from "@/components/common/ActivityTimeline";
import { TrendChart } from "@/components/common/TrendChart";
import { PermissionBadge } from "@/components/common/PermissionBadge";
import { SkeletonDetail } from "@/components/common/SkeletonLayout";
import { LinkAccessLog } from "./LinkAccessLog";
import {
  api,
  formatDate,
  formatDuration,
  formatRelativeTime,
  calculateUniqueVisitors,
} from "@/lib/api";
import type { AccessLog, Link } from "@/types";

function buildDailyTrend(logs: AccessLog[]): { labels: string[]; data: number[] } {
  const counts = new Map<string, { date: Date; count: number }>();
  for (const log of logs) {
    const date = new Date(log.timestamp);
    const dayKey = date.toISOString().slice(0, 10);
    const existing = counts.get(dayKey);
    if (existing) {
      existing.count += 1;
    } else {
      counts.set(dayKey, { date, count: 1 });
    }
  }
  const sorted = Array.from(counts.entries())
    .map(([, value]) => value)
    .sort((a, b) => a.date.getTime() - b.date.getTime());
  return {
    labels: sorted.map((d) => d.date.toLocaleDateString("zh-CN", { month: "short", day: "numeric" })),
    data: sorted.map((d) => d.count),
  };
}

export function LinkDetail() {
  const { workspaceSlug, linkId } = useParams<{ workspaceSlug: string; linkId: string }>();
  const [link, setLink] = useState<Link | null>(null);
  const [logs, setLogs] = useState<AccessLog[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    const id = linkId;
    if (!id) return;
    async function load() {
      try {
        setLoading(true);
        const [l, logData] = await Promise.all([api.getLinkById(id!), api.getAccessLogs(id!)]);
        if (!cancelled) {
          setLink(l);
          setLogs(logData.data);
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [linkId]);

  const trend = useMemo(() => buildDailyTrend(logs), [logs]);

  const timelineActivities = useMemo(() => {
    return [...logs]
      .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
      .slice(0, 20)
      .map((log) => ({
        id: log.id,
        time: formatRelativeTime(log.timestamp),
        title: `${log.visitorName || log.visitorEmail || "未知访客"}${
          log.pageNumber ? ` 查看第 ${log.pageNumber} 页` : " 访问链接"
        }`,
        description: log.durationSeconds
          ? `停留 ${formatDuration(log.durationSeconds)} · ${log.device || ""} · ${log.location || ""}`
          : undefined,
      }));
  }, [logs]);

  if (loading || !link) return <SkeletonDetail />;

  return (
    <div className="space-y-6">
      <BackButton to={`/${workspaceSlug}/links`} label="返回链接管理" />

      <PageHeader
        title={link.shortUrl.split("/").pop() || link.id}
        description={`文档：${link.documentTitle} · 创建于 ${formatDate(link.createdAt)}`}
      >
        <Button variant="outline" className="gap-1.5" onClick={() => {}}>
          <PencilSimple size={16} />
          编辑
        </Button>
        <Button
          variant="outline"
          className="gap-1.5"
          onClick={() => navigator.clipboard.writeText(link.shortUrl)}
        >
          <Copy size={16} />
          复制
        </Button>
        <Button className="gap-1.5" onClick={() => {}}>
          <ToggleRight size={16} />
          {link.isActive ? "停用" : "启用"}
        </Button>
      </PageHeader>

      <DetailLayout
        sidebar={
          <div className="space-y-4">
            <StatCard label="总访问" value={link.accessCount} />
            <StatCard label="独立访客" value={calculateUniqueVisitors(logs)} />
            <StatCard label="平均时长" value={formatDuration(link.avgDurationSeconds || 0)} />
            <StatCard
              label="最后访问"
              value={link.lastViewedAt ? formatRelativeTime(link.lastViewedAt) : "-"}
            />
            <Card>
              <CardHeader>
                <CardTitle className="text-h3">权限配置</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="flex flex-wrap gap-2">
                  <PermissionBadge type={link.permissionType || "public"} />
                  {link.expiresAt && (
                    <span className="text-caption text-muted-foreground">
                      过期于 {formatRelativeTime(link.expiresAt)}
                    </span>
                  )}
                </div>
              </CardContent>
            </Card>
          </div>
        }
      >
        <div className="space-y-6">
          <TrendChart
            title="访问量趋势"
            labels={trend.labels}
            data={trend.data}
            emptyDescription="暂无访问数据，趋势将在首次访问后自动生成。"
          />
          <Card>
            <CardHeader>
              <CardTitle className="text-h3">访问者时间线</CardTitle>
            </CardHeader>
            <CardContent>
              <ActivityTimeline activities={timelineActivities} />
            </CardContent>
          </Card>
        </div>
      </DetailLayout>

      <Card>
        <CardHeader>
          <CardTitle className="text-h2">访问日志</CardTitle>
        </CardHeader>
        <CardContent>
          <LinkAccessLog logs={logs} />
        </CardContent>
      </Card>
    </div>
  );
}
