import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { ArrowLeft, Copy, PencilSimple, ToggleRight } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { PageHeader } from "@/components/common/PageHeader";
import { DetailLayout } from "@/components/common/DetailLayout";
import { StatCard } from "@/components/common/StatCard";
import { ActivityTimeline } from "@/components/common/ActivityTimeline";
import { TrendChart } from "@/components/common/TrendChart";
import { PermissionBadge } from "@/components/common/PermissionBadge";
import { LinkAccessLog } from "./LinkAccessLog";
import { api, formatDate, formatDuration, formatRelativeTime } from "@/lib/api";
import { mockActivities } from "@/lib/mocks/data";
import type { AccessLog, Link } from "@/types";

function LinkSkeleton() {
  return (
    <div className="space-y-6">
      <Skeleton className="h-8 w-64" />
      <Skeleton className="h-48" />
      <Skeleton className="h-48" />
    </div>
  );
}

export function LinkDetail() {
  const navigate = useNavigate();
  const { workspaceSlug, linkId } = useParams<{ workspaceSlug: string; linkId: string }>();
  const [link, setLink] = useState<Link | null>(null);
  const [logs, setLogs] = useState<AccessLog[]>([]);
  const [loading, setLoading] = useState(true);

  const activities = useMemo(
    () => mockActivities.filter((a) => a.linkId === linkId).slice(0, 10),
    [linkId]
  );

  useEffect(() => {
    const timer = setTimeout(() => {
      if (!linkId) return;
      Promise.all([api.getLinkById(linkId), api.getAccessLogs(linkId)]).then(
        ([l, logData]) => {
          setLink(l);
          setLogs(logData.data);
          setLoading(false);
        }
      );
    }, 600);
    return () => clearTimeout(timer);
  }, [linkId]);

  if (loading || !link) return <LinkSkeleton />;

  const timelineActivities = activities.map((a) => ({
    id: a.id,
    time: formatRelativeTime(a.timestamp),
    title: `${a.contactEmail} ${
      a.eventType === "open"
        ? "打开文档"
        : a.eventType === "page_view"
        ? `查看第 ${a.pageNumber} 页`
        : a.eventType === "revisit"
        ? "再次访问"
        : "下载文档"
    }`,
    description: a.description,
  }));

  return (
    <div className="space-y-6">
      <button
        onClick={() => navigate(`/${workspaceSlug}/links`)}
        className="flex items-center gap-1 text-caption text-muted-foreground transition-colors hover:text-foreground"
      >
        <ArrowLeft size={14} />
        返回链接管理
      </button>

      <PageHeader
        title={link.shortUrl.split("/").pop() || link.id}
        description={`文档：${link.documentTitle} · 创建于 ${formatDate(link.createdAt)}`}
      >
        <Button variant="outline" className="gap-1.5">
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
        <Button className="gap-1.5">
          <ToggleRight size={16} />
          {link.isActive ? "停用" : "启用"}
        </Button>
      </PageHeader>

      <DetailLayout
        sidebar={
          <div className="space-y-4">
            <StatCard label="总访问" value={link.accessCount} />
            <StatCard label="独立访客" value={Math.max(1, Math.floor(link.accessCount / 4))} />
            <StatCard
              label="平均时长"
              value={formatDuration(link.avgDurationSeconds || 0)}
            />
            <StatCard
              label="最后访问"
              value={link.lastViewedAt ? formatRelativeTime(link.lastViewedAt) : "-"}
            />
            <Card>
              <CardHeader className="pb-2">
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
          <TrendChart title="访问量趋势" />
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-h3">访问者时间线</CardTitle>
            </CardHeader>
            <CardContent>
              <ActivityTimeline activities={timelineActivities} />
            </CardContent>
          </Card>
        </div>
      </DetailLayout>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-h2">访问日志</CardTitle>
        </CardHeader>
        <CardContent>
          <LinkAccessLog logs={logs} />
        </CardContent>
      </Card>
    </div>
  );
}
