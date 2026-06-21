import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router";
import {
  ArrowLeft,
  Envelope,
  FileText,
  Clock,
  Folder,
  Note,
  TrendUp,
  Users,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { PageHeader } from "@/components/common/PageHeader";
import { DetailLayout } from "@/components/common/DetailLayout";
import { StatCard } from "@/components/common/StatCard";
import { HeatBadge } from "@/components/common/HeatBadge";
import { ActivityTimeline } from "@/components/common/ActivityTimeline";
import { api, formatDuration, formatRelativeTime } from "@/lib/api";
import { mockContacts, mockDocuments } from "@/lib/mocks/data";
import type { Activity, Contact } from "@/types";

function ContactScoreChart({ score }: { score: number }) {
  const data = useMemo(() => {
    const base = Math.max(20, score - 30);
    return Array.from({ length: 12 }, (_, i) => {
      const progress = i / 11;
      return Math.round(base + (score - base) * progress + Math.sin(progress * Math.PI * 2) * 8);
    });
  }, [score]);

  const max = Math.max(...data, 1);
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-h2 flex items-center gap-2">
          <TrendUp size={20} />
          意向评分趋势
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="flex h-48 items-end gap-1">
          {data.map((h, i) => (
            <div
              key={i}
              className="flex-1 rounded-sm bg-primary/10 transition-all hover:bg-primary/20"
              style={{ height: `${(h / max) * 100}%` }}
            />
          ))}
        </div>
        <div className="mt-3 flex justify-between text-caption text-muted-foreground">
          <span>3 个月前</span>
          <span>本周</span>
        </div>
      </CardContent>
    </Card>
  );
}

export function ContactDetailPage() {
  const navigate = useNavigate();
  const { workspaceSlug, contactId } = useParams<{ workspaceSlug: string; contactId: string }>();
  const [contact, setContact] = useState<Contact | null>(null);
  const [activities, setActivities] = useState<Activity[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!contactId) return;
    Promise.all([api.getContactById(contactId), api.getActivitiesByContactId(contactId)]).then(
      ([c, a]) => {
        setContact(c);
        setActivities(a.data);
        setLoading(false);
      }
    );
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
    () =>
      (contact?.viewedDocuments ?? [])
        .map((id) => mockDocuments.find((d) => d.id === id))
        .filter(Boolean),
    [contact]
  );

  const relatedContacts = useMemo(
    () =>
      mockContacts
        .filter(
          (c) =>
            c.id !== contactId &&
            (c.organization === contact?.organization ||
              c.viewedDocuments.some((id) => contact?.viewedDocuments.includes(id)))
        )
        .slice(0, 5),
    [contact, contactId]
  );

  if (loading || !contact) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-48" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <button
        onClick={() => navigate(`/${workspaceSlug}/contacts`)}
        className="flex items-center gap-1 text-caption text-muted-foreground transition-colors hover:text-foreground"
      >
        <ArrowLeft size={14} />
        返回访问者
      </button>

      <PageHeader
        title={contact.name}
        description={`${contact.email} · ${contact.organization || "未知机构"} · ${contact.role || ""}`}
      >
        <Button variant="outline" className="gap-1.5">
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
              <CardHeader className="pb-2">
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
            <TabsTrigger value="overview">360° 视图</TabsTrigger>
            <TabsTrigger value="timeline">活动时间线</TabsTrigger>
            <TabsTrigger value="documents">浏览文档</TabsTrigger>
            <TabsTrigger value="notes">备注</TabsTrigger>
          </TabsList>

          <TabsContent value="overview" className="space-y-6">
            <ContactScoreChart score={contact.score} />
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-h2 flex items-center gap-2">
                  <Users size={20} />
                  相关联系人
                </CardTitle>
              </CardHeader>
              <CardContent>
                {relatedContacts.length === 0 ? (
                  <p className="text-sm text-muted-foreground">暂无相关联系人。</p>
                ) : (
                  <ul className="space-y-2">
                    {relatedContacts.map((c) => (
                      <li
                        key={c.id}
                        className="flex cursor-pointer items-center justify-between rounded-md border border-border p-3 transition-colors hover:bg-muted"
                        onClick={() => navigate(`/${workspaceSlug}/contacts/${c.id}`)}
                      >
                        <div>
                          <p className="text-sm font-medium">{c.name}</p>
                          <p className="text-caption text-muted-foreground">
                            {c.organization} · {c.email}
                          </p>
                        </div>
                        <HeatBadge level={c.heatLevel} />
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="timeline">
            <Card>
              <CardHeader className="pb-2">
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
              <CardHeader className="pb-2">
                <CardTitle className="text-h2 flex items-center gap-2">
                  <Folder size={20} />
                  浏览过的文档
                </CardTitle>
              </CardHeader>
              <CardContent>
                {viewedDocuments.length === 0 ? (
                  <p className="text-sm text-muted-foreground">暂无浏览记录。</p>
                ) : (
                  <ul className="space-y-2">
                    {viewedDocuments.map((doc) => (
                      <li
                        key={doc!.id}
                        className="flex cursor-pointer items-center justify-between rounded-md border border-border p-3 transition-colors hover:bg-muted"
                        onClick={() => navigate(`/${workspaceSlug}/documents/${doc!.id}`)}
                      >
                        <div className="flex items-center gap-3">
                          <FileText size={18} className="text-muted-foreground" />
                          <p className="text-sm font-medium">{doc!.title}</p>
                        </div>
                        <Button size="sm" variant="outline">
                          查看
                        </Button>
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="notes">
            <Card>
              <CardHeader className="pb-2">
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
