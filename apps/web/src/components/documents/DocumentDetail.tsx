import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { Copy, DownloadSimple, Eye, Link as LinkIcon, Plus, Trash } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { PageHeader } from "@/components/common/PageHeader";
import { BackButton } from "@/components/common/BackButton";
import { DetailLayout } from "@/components/common/DetailLayout";
import { StatCard } from "@/components/common/StatCard";
import { HeatBadge } from "@/components/common/HeatBadge";
import { FileTypeIcon } from "@/components/common/FileTypeIcon";
import { VisitorList } from "@/components/common/VisitorList";
import { RowActions } from "@/components/common/RowActions";
import { SkeletonDetail } from "@/components/common/SkeletonLayout";
import { DocumentAnalytics } from "./DocumentAnalytics";
import { DocumentContent } from "./DocumentContent";
import { DocumentAIInsights } from "./DocumentAIInsights";
import { api, formatDuration, formatFileSize, formatRelativeTime, calculateUniqueVisitors } from "@/lib/api";
import type { Document, Link, PageAnalytics, AccessLog, HeatLevel } from "@/types";

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
    const email = log.visitorEmail;
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

export function DocumentDetail() {
  const navigate = useNavigate();
  const { workspaceSlug, documentId } = useParams<{ workspaceSlug: string; documentId: string }>();
  const [doc, setDoc] = useState<Document | null>(null);
  const [links, setLinks] = useState<Link[]>([]);
  const [analytics, setAnalytics] = useState<PageAnalytics[]>([]);
  const [logs, setLogs] = useState<AccessLog[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    const id = documentId;
    if (!id) return;
    async function load() {
      try {
        setLoading(true);
        const [d, l, a] = await Promise.all([
          api.getDocumentById(id!),
          api.getLinksByDocumentId(id!),
          api.getPageAnalytics(id!),
        ]);
        const allLogs = await Promise.all(l.data.map((link) => api.getAccessLogs(link.id).then((r) => r.data)));
        if (!cancelled) {
          setDoc(d);
          setLinks(l.data);
          setAnalytics(a.data);
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
  }, [documentId]);

  const visitors = useMemo(() => aggregateVisitors(logs), [logs]);

  const heatDistribution = useMemo(() => {
    const counts = { hot: 0, warm: 0, cold: 0 } as Record<HeatLevel, number>;
    for (const link of links) {
      counts[link.heatLevel] = (counts[link.heatLevel] ?? 0) + 1;
    }
    return counts;
  }, [links]);

  if (loading || !doc) return <SkeletonDetail />;

  const totalViews = links.reduce((sum, l) => sum + l.accessCount, 0);
  const uniqueVisitors = calculateUniqueVisitors(logs);
  const avgDuration =
    links.length > 0
      ? Math.round(links.reduce((sum, l) => sum + (l.avgDurationSeconds || 0), 0) / links.length)
      : 0;

  return (
    <div className="space-y-6">
      <BackButton to={`/${workspaceSlug}/documents`} label="返回文档库" />

      <PageHeader
        title={doc.title}
        description={`${doc.fileType.toUpperCase()} · ${doc.pageCount} 页 · ${formatFileSize(
          doc.fileSize
        )} · 上传于 ${formatRelativeTime(doc.createdAt)}`}
      >
        <Button variant="outline" className="gap-1.5" onClick={() => navigate(`/${workspaceSlug}/viewer/${doc.id}`)}>
          <Eye size={16} />
          预览
        </Button>
        <Button className="gap-1.5" onClick={() => navigate(`/${workspaceSlug}/links/new?documentId=${doc.id}`)}>
          <LinkIcon size={16} />
          创建链接
        </Button>
        <RowActions
          actions={[
            {
              label: "下载",
              icon: <DownloadSimple size={16} />,
              onClick: () => {},
              pro: true,
            },
            {
              label: "删除",
              icon: <Trash size={16} />,
              onClick: () => navigate(`/${workspaceSlug}/documents`),
              destructive: true,
              pro: true,
            },
          ]}
        />
      </PageHeader>

      <DetailLayout
        sidebar={
          <div className="space-y-4">
            <StatCard label="总访问" value={totalViews} />
            <StatCard label="独立访客" value={uniqueVisitors} />
            <StatCard label="平均停留" value={formatDuration(avgDuration)} />
            <Card>
              <CardHeader>
                <CardTitle className="text-h3">链接热度分布</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="flex flex-wrap gap-2">
                  {heatDistribution.hot > 0 && (
                    <div className="flex items-center gap-1.5 rounded-full bg-hot-500/10 px-2 py-1 text-xs font-medium text-hot-500">
                      <span className="h-1.5 w-1.5 rounded-full bg-hot-500" />
                      高 {heatDistribution.hot}
                    </div>
                  )}
                  {heatDistribution.warm > 0 && (
                    <div className="flex items-center gap-1.5 rounded-full bg-warm-500/10 px-2 py-1 text-xs font-medium text-warm-500">
                      <span className="h-1.5 w-1.5 rounded-full bg-warm-500" />
                      中 {heatDistribution.warm}
                    </div>
                  )}
                  {heatDistribution.cold > 0 && (
                    <div className="flex items-center gap-1.5 rounded-full bg-cold-500/10 px-2 py-1 text-xs font-medium text-cold-500">
                      <span className="h-1.5 w-1.5 rounded-full bg-cold-500" />
                      低 {heatDistribution.cold}
                    </div>
                  )}
                  {links.length === 0 && (
                    <p className="text-sm text-muted-foreground">暂无链接</p>
                  )}
                </div>
              </CardContent>
            </Card>
          </div>
        }
      >
        <Tabs defaultValue="overview" className="w-full">
          <TabsList className="mb-4">
            <TabsTrigger value="overview">总览</TabsTrigger>
            <TabsTrigger value="content">内容</TabsTrigger>
            <TabsTrigger value="analytics">数据</TabsTrigger>
            <TabsTrigger value="ai">AI 洞察</TabsTrigger>
          </TabsList>
          <TabsContent value="overview" className="space-y-6">
            <DocumentAnalytics analytics={analytics} />
            <Card>
              <CardHeader>
                <CardTitle className="text-h2 flex items-center gap-2">
                  <Plus size={20} />
                  最近访客
                </CardTitle>
              </CardHeader>
              <CardContent>
                <VisitorList visitors={visitors} />
              </CardContent>
            </Card>
          </TabsContent>
          <TabsContent value="content">
            <DocumentContent title={doc.title} pageCount={doc.pageCount} analytics={analytics} evidences={[]} />
          </TabsContent>
          <TabsContent value="analytics">
            <DocumentAnalytics analytics={analytics} />
          </TabsContent>
          <TabsContent value="ai">
            <DocumentAIInsights documentId={doc.id} analytics={analytics} />
          </TabsContent>
        </Tabs>
      </DetailLayout>

      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <LinkIcon size={20} />
            此文档的链接
          </CardTitle>
        </CardHeader>
        <CardContent>
          {links.length === 0 ? (
            <div className="py-8 text-center text-muted-foreground">
              暂无链接，点击右上角创建链接开始分享。
            </div>
          ) : (
            <ul className="space-y-2">
              {links.map((link) => (
                <li
                  key={link.id}
                  className="flex items-center justify-between rounded-md border border-border p-3 transition-colors hover:bg-muted"
                >
                  <div className="flex min-w-0 items-center gap-3">
                    <FileTypeIcon type={doc.fileType} />
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">{link.shortUrl}</p>
                      <p className="text-caption text-muted-foreground">
                        {link.accessCount} views · {formatRelativeTime(link.createdAt)}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <HeatBadge level={link.heatLevel} />
                    <Button
                      size="icon-sm"
                      variant="ghost"
                      onClick={() => navigator.clipboard.writeText(link.shortUrl)}
                    >
                      <Copy size={14} />
                    </Button>
                    <Button size="sm" variant="outline" onClick={() => navigate(`/${workspaceSlug}/links/${link.id}`)}>
                      日志
                    </Button>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
