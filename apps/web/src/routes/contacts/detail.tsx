import { useMemo } from "react";
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
import { useTranslation } from "react-i18next";
import { useAsyncData } from "@/hooks/useAsyncData";

function ContactScoreChart({
  scoreHistory,
  title,
  emptyDescription,
  locale,
}: {
  scoreHistory: { date: string; score: number }[];
  title: string;
  emptyDescription: string;
  locale: string;
}) {
  const labels = useMemo(
    () => scoreHistory.map((h) => new Date(h.date).toLocaleDateString(locale, { month: "short", day: "numeric" })),
    [scoreHistory, locale]
  );
  const data = useMemo(() => scoreHistory.map((h) => h.score), [scoreHistory]);

  return <TrendChart title={title} labels={labels} data={data} emptyDescription={emptyDescription} />;
}

export function ContactDetailPage() {
  const { t, i18n } = useTranslation("contacts");
  const { t: tc } = useTranslation("common");
  const navigate = useNavigate();
  const { workspaceSlug, contactId } = useParams<{ workspaceSlug: string; contactId: string }>();
  const locale = i18n.language;

  const { data, loading, error, refetch } = useAsyncData(async () => {
    if (!contactId) {
      throw new Error(t("detail.notFound"));
    }
    const [c, a, docsRes] = await Promise.all([
      api.getContactById(contactId),
      api.getActivitiesByContactId(contactId),
      api.getDocuments(),
    ]);
    return { contact: c, activities: a.data, documents: docsRes.data };
  }, [contactId, t]);

  const contact = data?.contact ?? null;

  const timelineActivities = useMemo(
    () =>
      (data?.activities ?? []).map((a) => ({
        id: a.id,
        time: formatRelativeTime(a.timestamp, locale),
        title: `${a.contactEmail} ${
          a.eventType === "open"
            ? t("activity.open")
            : a.eventType === "page_view"
            ? t("activity.pageView", { page: a.pageNumber })
            : a.eventType === "revisit"
            ? t("activity.revisit")
            : t("activity.download")
        }`,
        description: `${a.documentTitle} · ${t(a.description)}`,
      })),
    [data?.activities, locale, t]
  );

  const viewedDocuments = useMemo(
    () => (data?.documents ?? []).filter((d) => contact?.viewedDocuments.includes(d.id)),
    [data?.documents, contact]
  );

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
        <p className="text-body text-muted-foreground">{error}</p>
        <Button onClick={refetch}>{tc("retry")}</Button>
      </div>
    );
  }

  if (loading || !contact) {
    return <SkeletonDetail />;
  }

  return (
    <div className="space-y-6">
      <BackButton to={`/${workspaceSlug}/contacts`} label={t("detail.back")} />

      <PageHeader
        title={contact.name}
        description={`${contact.email} · ${contact.organization || t("unknownOrganization")} · ${contact.role || ""}`}
      >
        <Button variant="outline" className="gap-1.5" onClick={() => window.open(`mailto:${contact.email}`)}>
          <Envelope size={16} />
          {t("detail.writeEmail")}
        </Button>
      </PageHeader>

      <DetailLayout
        sidebar={
          <div className="space-y-4">
            <StatCard label={t("detail.totalVisits")} value={contact.totalVisits} />
            <StatCard label={t("detail.totalDuration")} value={formatDuration(contact.totalDurationSeconds, locale)} />
            <StatCard label={t("detail.score")} value={contact.score} />
            <Card>
              <CardHeader>
                <CardTitle className="text-h3">{t("detail.heat")}</CardTitle>
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
            <TabsTrigger value="overview">{t("detail.tabs.overview")}</TabsTrigger>
            <TabsTrigger value="timeline">{t("detail.tabs.timeline")}</TabsTrigger>
            <TabsTrigger value="documents">{t("detail.tabs.documents")}</TabsTrigger>
            <TabsTrigger value="notes">{t("detail.tabs.notes")}</TabsTrigger>
          </TabsList>

          <TabsContent value="overview" className="space-y-6">
            <ContactScoreChart
              scoreHistory={contact.scoreHistory}
              title={t("detail.recentActivity")}
              emptyDescription={t("detail.noActivities.description")}
              locale={locale}
            />
            <Card>
              <CardHeader>
                <CardTitle className="text-h2 flex items-center gap-2">
                  <Users size={20} />
                  {t("detail.recentActivity")}
                </CardTitle>
              </CardHeader>
              <CardContent>
                {(data?.activities ?? []).length === 0 ? (
                  <EmptyState
                    icon={<Clock size={48} />}
                    title={t("detail.noActivities.title")}
                    description={t("detail.noActivities.description")}
                    size="large"
                  />
                ) : (
                  <ul className="space-y-2">
                    {(data?.activities ?? []).slice(0, 5).map((a) => (
                      <li key={a.id} className="flex items-center justify-between rounded-md border border-border p-3">
                        <div>
                          <p className="text-sm font-medium">{a.documentTitle}</p>
                          <p className="text-caption text-muted-foreground">
                            {a.eventType === "open"
                              ? t("activity.openShort")
                              : a.eventType === "page_view"
                              ? t("activity.pageViewShort", { page: a.pageNumber })
                              : a.eventType === "revisit"
                              ? t("activity.revisitShort")
                              : t("activity.downloadShort")}
                            {" · "}
                            {formatRelativeTime(a.timestamp, locale)}
                          </p>
                        </div>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => navigate(`/${workspaceSlug}/links/${a.linkId}`)}
                        >
                          {t("detail.view")}
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
                  {t("detail.tabs.timeline")}
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
                  {t("detail.viewedDocuments")}
                </CardTitle>
              </CardHeader>
              <CardContent>
                {viewedDocuments.length === 0 ? (
                  <EmptyState
                    icon={<Folder size={48} />}
                    title={t("detail.noDocuments.title")}
                    description={t("detail.noDocuments.description")}
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
                            {t("detail.view")}
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
                  {t("detail.tabs.notes")}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-sm text-muted-foreground">{t("detail.notesHint")}</p>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </DetailLayout>
    </div>
  );
}
