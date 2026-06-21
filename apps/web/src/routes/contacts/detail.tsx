import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router";
import {
  Envelope,
  FileText,
  Clock,
  Folder,
  Note,
  Users,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { PageHeader } from "@/components/common/PageHeader";
import { BackButton } from "@/components/common/BackButton";
import { DetailLayout } from "@/components/common/DetailLayout";
import { StatCard } from "@/components/common/StatCard";
import { HeatBadge } from "@/components/common/HeatBadge";
import { ActivityTimeline } from "@/components/common/ActivityTimeline";
import { TrendChart } from "@/components/common/TrendChart";
import { SkeletonDetail } from "@/components/common/SkeletonLayout";
import { EmptyState } from "@/components/common/EmptyState";
import { api } from "@/lib/api";
import { formatDuration, formatRelativeTime } from "@/lib/formatters";
import type { Activity, Contact, Document } from "@/types";

function ContactScoreChart({
  scoreHistory,
}: {
  scoreHistory: { date: string; score: number }[];
}) {
  const labels = useMemo(
    () => scoreHistory.map((h) => new Date(h.date).toLocaleDateString("zh-CN", { month: "short", day: "numeric" })),
    [scoreHistory]
  );
  const data = useMemo(() => scoreHistory.map((h) => h.score), [scoreHistory]);

  return (
    <TrendChart
      title="意向评分趋势"
      labels={labels}
      data={data}
      emptyDescription="评分历史不足，趋势将在更多互动后生成。"
    />
  );
}

export function ContactDetailPage() {
  const navigate = useNavigate();
  const { workspaceSlug, contactId } = useParams<{ workspaceSlug: string; contactId: string }>();
  const [contact, setContact] = useState<Contact | null>(null);
  const [activities, setActivities] = useState<Activity[]>([]);
  const [documents, setDocuments] = useState<Document[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    const id = contactId;
    if (!id) return;
    async function load() {
      try {
        setLoading(true);
        setError(null);
        const [c, a, docsRes] = await Promise.all([
          api.getContactById(id!),
          api.getActivitiesByContactId(id!),
          api.getDocuments(),
        ]);
        if (!cancelled) {
          setContact(c);
          setActivities(a.data);
          setDocuments(docsRes.data);
        }
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : "加载失败");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [contactId]);

  const timelineActivities = useMemo(
    () =>
      activities.map((a) => ({
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
        description: `${a.documentTitle} · ${a.description}`,
      })),
    [activities]
  );

  const viewedDocuments = useMemo(
    () => documents.filter((d) => contact?.viewedDocuments.includes(d.id)),
    [documents, contact]
  );

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
        <p className="text-body text-muted-foreground">{error}</p>
        <Button onClick={() => window.location.reload()}>重试</Button>
      </div>
    );
  }

  if (loading || !contact) {
    return <SkeletonDetail />;
  }

  return (
    <div className="space-y-6">
      <BackButton to={`/${workspaceSlug}/contacts`} label="返回访问者" />

      <PageHeader
        title={contact.name}
        description={`${contact.email} · ${contact.organization || "未知机构"} · ${contact.role || ""}`}
      >
        <Button variant="outline" className="gap-1.5" onClick={() => window.open(`mailto:${contact.email}`)}>
          <Envelope size={16} />
          写邮件
        </Button>
      </PageHeader>

      <DetailLayout
        sidebar={
          <div className="space-y-4">
            <StatCard label="总访问" value={contact.totalVisits} />
            <StatCard label="累计时长" value={formatDuration(contact.totalDurationSeconds)} />
            <StatCard label="热度分" value={contact.score} />
            <Card>
              <CardHeader>
                <CardTitle className="text-h3">热度</CardTitle>
              </CardHeader>
              <CardContent>
                <HeatBadge level={contact.heatLevel} />
              </CardContent>
            </Card>
          </div>
        }
      >
        <Tabs defaultValue="overview" className="w-full">
          <TabsList className="mb-4">
            <TabsTrigger value="overview">概览</TabsTrigger>
            <TabsTrigger value="timeline">活动时间线</TabsTrigger>
            <TabsTrigger value="documents">浏览文档</TabsTrigger>
            <TabsTrigger value="notes">备注</TabsTrigger>
          </TabsList>

          <TabsContent value="overview" className="space-y-6">
            <ContactScoreChart scoreHistory={contact.scoreHistory} />
            <Card>
              <CardHeader>
                <CardTitle className="text-h2 flex items-center gap-2">
                  <Users size={20} />
                  最近活动摘要
                </CardTitle>
              </CardHeader>
              <CardContent>
                {activities.length === 0 ? (
                  <EmptyState
                    icon={<Clock size={48} />}
                    title="暂无活动记录"
                    description="当联系人访问文档时，这里会显示最近活动。"
                    size="large"
                  />
                ) : (
                  <ul className="space-y-2">
                    {activities.slice(0, 5).map((a) => (
                      <li key={a.id} className="flex items-center justify-between rounded-md border border-border p-3">
                        <div>
                          <p className="text-sm font-medium">{a.documentTitle}</p>
                          <p className="text-caption text-muted-foreground">
                            {a.eventType === "open"
                              ? "打开"
                              : a.eventType === "page_view"
                              ? `查看第 ${a.pageNumber} 页`
                              : a.eventType === "revisit"
                              ? "再次访问"
                              : "下载"}
                            {" · "}
                            {formatRelativeTime(a.timestamp)}
                          </p>
                        </div>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => navigate(`/${workspaceSlug}/links/${a.linkId}`)}
                        >
                          查看
                        </Button>
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="timeline">
            <Card>
              <CardHeader>
                <CardTitle className="text-h2 flex items-center gap-2">
                  <Clock size={20} />
                  活动时间线
                </CardTitle>
              </CardHeader>
              <CardContent>
                <ActivityTimeline activities={timelineActivities} />
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="documents">
            <Card>
              <CardHeader>
                <CardTitle className="text-h2 flex items-center gap-2">
                  <Folder size={20} />
                  浏览过的文档
                </CardTitle>
              </CardHeader>
              <CardContent>
                {viewedDocuments.length === 0 ? (
                  <EmptyState
                    icon={<Folder size={48} />}
                    title="暂无浏览记录"
                    description="联系人尚未通过链接浏览任何文档。"
                    size="large"
                  />
                ) : (
                  <ul className="space-y-2">
                    {viewedDocuments.map((doc) => {
                      const handleClick = () => navigate(`/${workspaceSlug}/documents/${doc.id}`);
                      return (
                        <li
                          key={doc.id}
                          role="link"
                          tabIndex={0}
                          className="flex cursor-pointer items-center justify-between rounded-md border border-border p-3 transition-colors hover:bg-muted"
                          onClick={handleClick}
                          onKeyDown={(e) => {
                            if (e.key === "Enter" || e.key === " ") {
                              e.preventDefault();
                              handleClick();
                            }
                          }}
                        >
                          <div className="flex items-center gap-3">
                            <FileText size={18} className="text-muted-foreground" />
                            <p className="text-sm font-medium">{doc.title}</p>
                          </div>
                          <Button size="sm" variant="outline">
                            查看
                          </Button>
                        </li>
                      );
                    })}
                  </ul>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="notes">
            <Card>
              <CardHeader>
                <CardTitle className="text-h2 flex items-center gap-2">
                  <Note size={20} />
                  联系人备注
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-sm text-muted-foreground">
                  暂无备注。点击右上角“写邮件”即可记录沟通要点。
                </p>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </DetailLayout>
    </div>
  );
}
