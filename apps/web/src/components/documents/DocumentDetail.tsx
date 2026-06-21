import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router";
import {
  ArrowLeft,
  Copy,
  DownloadSimple,
  Eye,
  Link as LinkIcon,
  Plus,
  Trash,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { PageHeader } from "@/components/common/PageHeader";
import { DetailLayout } from "@/components/common/DetailLayout";
import { StatCard } from "@/components/common/StatCard";
import { HeatBadge } from "@/components/common/HeatBadge";
import { FileTypeIcon } from "@/components/common/FileTypeIcon";
import { VisitorList } from "@/components/common/VisitorList";
import { RowActions } from "@/components/common/RowActions";
import { DocumentAnalytics } from "./DocumentAnalytics";
import { DocumentContent } from "./DocumentContent";
import { DocumentAIInsights } from "./DocumentAIInsights";
import { api, formatDuration, formatFileSize, formatRelativeTime } from "@/lib/api";
import { mockContacts } from "@/lib/mocks/data";
import type { Document, Link, PageAnalytics, Evidence } from "@/types";

function DocumentSkeleton() {
  return (
    <div className="space-y-6">
      <Skeleton className="h-8 w-64" />
      <Skeleton className="h-48" />
      <Skeleton className="h-48" />
    </div>
  );
}

const mockEvidences: Evidence[] = [
  {
    id: "ev_doc_1",
    pageNumber: 12,
    text: "财务预测显示 2026 年收入 1,200 万美元，毛利率 72%。",
    bbox: { x: 0.1, y: 0.2, w: 0.8, h: 0.1 },
  },
  {
    id: "ev_doc_2",
    pageNumber: 7,
    text: "核心团队拥有 20+ 年企业服务经验，此前成功退出两次。",
    bbox: { x: 0.1, y: 0.4, w: 0.8, h: 0.1 },
  },
];

export function DocumentDetail() {
  const navigate = useNavigate();
  const { workspaceSlug, documentId } = useParams<{
    workspaceSlug: string;
    documentId: string;
  }>();
  const [doc, setDoc] = useState<Document | null>(null);
  const [links, setLinks] = useState<Link[]>([]);
  const [analytics, setAnalytics] = useState<PageAnalytics[]>([]);
  const [loading, setLoading] = useState(true);

  const visitors = useMemo(
    () =>
      mockContacts
        .filter((c) => c.viewedDocuments.includes(documentId || ""))
        .map((c) => ({
          id: c.id,
          email: c.email,
          organization: c.organization,
          heatLevel: c.heatLevel,
          visitCount: c.totalVisits,
          avgDurationSeconds: c.totalDurationSeconds,
          lastSeenAt: c.lastSeenAt || "-",
        }))
        .slice(0, 5),
    [documentId]
  );

  useEffect(() => {
    if (!documentId) return;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setLoading(true);
    Promise.all([
      api.getDocumentById(documentId),
      api.getLinksByDocumentId(documentId),
      api.getPageAnalytics(documentId),
    ]).then(([d, l, a]) => {
      setDoc(d);
      setLinks(l.data);
      setAnalytics(a.data);
      setLoading(false);
    });
  }, [documentId]);

  if (loading || !doc) return <DocumentSkeleton />;

  const totalViews = links.reduce((sum, l) => sum + l.accessCount, 0);
  const uniqueVisitors = visitors.length;
  const avgDuration =
    links.length > 0
      ? Math.round(links.reduce((sum, l) => sum + (l.avgDurationSeconds || 0), 0) / links.length)
      : 0;

  return (
    <div className="space-y-6">
      <button
        onClick={() => navigate(`/${workspaceSlug}/documents`)}
        className="flex items-center gap-1 text-caption text-muted-foreground transition-colors hover:text-foreground"
      >
        <ArrowLeft size={14} />
        返回文档库
      </button>

      <PageHeader
        title={doc.title}
        description={`${doc.fileType.toUpperCase()} · ${doc.pageCount} 页 · ${formatFileSize(
          doc.fileSize
        )} · 上传于 ${formatRelativeTime(doc.createdAt)}`}
      >
        <Button variant="outline" className="gap-1.5">
          <Eye size={16} />
          预览
        </Button>
        <Button
          className="gap-1.5"
          onClick={() => navigate(`/${workspaceSlug}/links/new?documentId=${doc.id}`)}
        >
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
              <CardHeader className="pb-2">
                <CardTitle className="text-h3">热度分布</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="flex flex-wrap gap-2">
                  <HeatBadge level="hot" />
                  <HeatBadge level="warm" />
                  <HeatBadge level="cold" />
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
              <CardHeader className="pb-3">
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
            <DocumentContent
              title={doc.title}
              pageCount={doc.pageCount}
              analytics={analytics}
              evidences={mockEvidences}
            />
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
        <CardHeader className="pb-3">
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
                    <FileTypeIcon type="pdf" />
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
                      size="icon"
                      variant="ghost"
                      onClick={() => navigator.clipboard.writeText(link.shortUrl)}
                    >
                      <Copy size={14} />
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => navigate(`/${workspaceSlug}/links/${link.id}`)}
                    >
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
