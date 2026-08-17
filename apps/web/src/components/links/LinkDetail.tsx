import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { Copy, PencilSimple, ToggleRight, FileText, ChatTeardropText } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { PageHeader } from "@/components/common/PageHeader";
import { SmartBackButton } from "@/components/common/SmartBackButton";
import { DetailLayout } from "@/components/common/DetailLayout";
import { StatCard } from "@/components/common/StatCard";
import { ActivityTimeline } from "@/components/common/ActivityTimeline";
import { PageDurationChart } from "@/components/common/PageDurationChart";
import { PermissionBadge } from "@/components/common/PermissionBadge";
import { SkeletonDetail } from "@/components/common/SkeletonLayout";
import { OwnerAskInboxPanel } from "@/components/ask/OwnerAskInboxPanel";
import { LinkAccessLog } from "./LinkAccessLog";
import { copyToClipboard } from "@/lib/clipboard";
import { api } from "@/lib/api";
import { apiErrorMessage } from "@/lib/apiErrors";
import { documentsSharePath } from "@/lib/documentsSharePath";
import { formatDate, formatDuration, formatRelativeTime } from "@/lib/formatters";
import { buildPageDurationDataFromMetrics } from "@/lib/linkPageDuration";
import { accessLogDocumentTitle, formatShareDocumentLabel } from "@/lib/shareDocumentLabel";
import { parseOwnerAskInboxView } from "@/lib/ownerAskInbox";
import { useWorkspaceAccess } from "@/hooks/useWorkspaceAccess";
import { canManageAskHost, canMutateShareLink } from "@/lib/dealRoomCapabilities";
import type { AccessLog, Document, Link, LinkAnalytics } from "@/types";

export function LinkDetail() {
  const navigate = useNavigate();
  const { workspaceSlug, linkId } = useParams<{ workspaceSlug: string; linkId: string }>();
  const [searchParams] = useSearchParams();
  const { t } = useTranslation("links");
  const { t: tShare } = useTranslation("linkShare");
  const { t: tc } = useTranslation("common");
  const { canWrite, canManage } = useWorkspaceAccess(workspaceSlug);
  const askInboxView = parseOwnerAskInboxView(searchParams.get("askInbox"));
  const [link, setLink] = useState<Link | null>(null);
  const [document, setDocument] = useState<Document | null>(null);
  const [logs, setLogs] = useState<AccessLog[]>([]);
  const [analytics, setAnalytics] = useState<LinkAnalytics | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [retryTick, setRetryTick] = useState(0);

  useEffect(() => {
    let cancelled = false;
    const id = linkId;
    if (!id) return;
    async function load() {
      try {
        setLoading(true);
        setError(null);
        const l = await api.getLinkById(id!);
        // Deal-room / multi-doc links may omit a primary documentId.
        const primaryDocId =
          l.documentId?.trim() ||
          l.documentIds?.find((docId) => Boolean(docId?.trim())) ||
          l.documents?.find((d) => Boolean(d.id))?.id ||
          undefined;
        const [logData, docData, analyticsRes] = await Promise.all([
          api.getAccessLogs(id!),
          primaryDocId
            ? api.getDocumentById(primaryDocId).catch(() => null)
            : Promise.resolve(null),
          api.getLinkAnalytics(id!).catch(() => null),
        ]);
        if (!cancelled) {
          setLink(l);
          setDocument(docData);
          setLogs(logData.data);
          setAnalytics(analyticsRes?.data ?? null);
        }
      } catch (e) {
        if (!cancelled) setError(apiErrorMessage(e));
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [linkId, retryTick, tc]);

  const pageDurationData = useMemo(() => {
    const documents =
      link?.documents && link.documents.length > 0
        ? link.documents
        : document
          ? [{ id: document.id, title: document.title, pageCount: document.pageCount }]
          : [];
    return buildPageDurationDataFromMetrics(
      (analytics?.page_durations ?? []).map((page) => ({
        documentId: page.document_id,
        pageNumber: page.page_number,
        avgDurationSeconds: page.average_duration_seconds,
      })),
      {
        documents,
        primaryDocumentId: link?.documentId ?? document?.id,
        formatBundleLabel: (title, page) => t("detail.pageOnDocument", { title, page }),
      },
    );
  }, [analytics, link, document, t]);

  const timelineActivities = useMemo(() => {
    const docs = link?.documents ?? [];
    const isBundle = docs.length > 1;
    return [...logs]
      .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
      .slice(0, 20)
      .map((log) => {
        const visitor = log.visitorName || log.visitorEmail || tc("visitor.unknown");
        const title = accessLogDocumentTitle(log, docs, link?.documentId ?? document?.id);
        return {
          id: log.id,
          time: formatRelativeTime(log.timestamp),
          title:
            log.pageNumber && isBundle && title
              ? t("timeline.viewedPageOnDocument", { visitor, title, page: log.pageNumber })
              : log.pageNumber
                ? t("timeline.viewedPage", { visitor, page: log.pageNumber })
                : t("timeline.viewedLink", { visitor }),
          description: log.durationSeconds
            ? t("timeline.description", { duration: formatDuration(log.durationSeconds), device: log.device || "", location: log.location || "" })
            : undefined,
        };
      });
  }, [logs, link, document, t, tc]);

  if (error) {
    return (
      <div className="space-y-6">
        <SmartBackButton fallbackTo={documentsSharePath(workspaceSlug!)} fallbackLabel={t("backToLinks")} />
        <Card>
          <CardContent className="py-12 text-center">
            <p className="text-body text-destructive mb-4">{error || tc("error.loadFailed")}</p>
            <Button onClick={() => setRetryTick((t) => t + 1)}>{tc("retry")}</Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (loading || !link) return <SkeletonDetail />;

  const canMutateLink = canMutateShareLink({
    dealRoomId: link.dealRoomId,
    workspaceCanWrite: canWrite,
    linkCanManageAsk: link.canManageAsk,
  });

  return (
    <div className="space-y-6">
      <SmartBackButton fallbackTo={documentsSharePath(workspaceSlug!)} fallbackLabel={t("backToLinks")} />

      <PageHeader
        title={(link.shortUrl || link.id).split("/").pop() || link.id}
        description={t("detail.headerDescription", {
          doc:
            formatShareDocumentLabel(link, (title, count) =>
              t("table.bundleDocument", { title, count }),
            ) || link.documentTitle,
          date: formatDate(link.createdAt),
        })}
      >
        {canMutateLink ? (
          <Button
            variant="outline"
            className="gap-1.5"
            onClick={() => navigate(`/${workspaceSlug}/links/${link.id}/edit`)}
          >
            <PencilSimple size={16} />
            {tc("edit")}
          </Button>
        ) : null}
        <Button
          variant="outline"
          className="gap-1.5"
          onClick={() => {
            void copyToClipboard(link.shortUrl, t("detail.copySuccess"));
          }}
        >
          <Copy size={16} />
          {tc("copy")}
        </Button>
        {canMutateLink ? (
          <Button
            className="gap-1.5"
            onClick={async () => {
              const next = !link.isActive;
              const updated = await api.updateLink(link.id, { isActive: next });
              setLink(updated);
            }}
          >
            <ToggleRight size={16} />
            {link.isActive ? tc("status.disabled") : tc("status.enabled")}
          </Button>
        ) : null}
      </PageHeader>

      {link.isBundle && link.documents.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="text-h3 flex items-center gap-2">
              <FileText size={20} />
              {t("bundle.documents.label")} ({link.documents.length})
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="divide-y divide-border rounded-md border border-border">
              {link.documents.map((doc, i) => (
                <div key={doc.id} className="flex items-center gap-3 px-3 py-2.5">
                  <span className="text-sm text-muted-foreground w-5 text-right">{i + 1}.</span>
                  <FileText size={18} className="shrink-0 text-muted-foreground" />
                  <span className="flex-1 truncate text-sm font-medium">{doc.title}</span>
                  <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-caption text-muted-foreground">
                    {doc.sourceType.toUpperCase()}
                  </span>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      <DetailLayout
        sidebar={
          <div className="space-y-4">
            <StatCard size="sm" label={t("detail.totalVisits")} value={link.accessCount} />
            <StatCard
              size="sm"
              label={t("detail.uniqueVisitors")}
              value={analytics ? analytics.unique_visitors : "—"}
            />
            <p className="text-caption text-muted-foreground">{t("detail.uniqueVisitorsHint")}</p>
            <StatCard
              size="sm"
              label={t("detail.avgDuration")}
              value={
                analytics ? formatDuration(analytics.average_duration_seconds || 0) : "—"
              }
            />
            <StatCard
              size="sm"
              label={t("detail.lastVisit")}
              value={link.lastViewedAt ? formatRelativeTime(link.lastViewedAt) : "-"}
            />
            <Card>
              <CardHeader>
                <CardTitle className="text-body font-semibold">{t("detail.permissionConfig")}</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="flex flex-wrap gap-2">
                  <PermissionBadge type={link.permissionType || "public"} />
                  {link.expiresAt && (
                    <span className="text-caption text-muted-foreground">
                      {t("detail.expiresAt", { time: formatRelativeTime(link.expiresAt) })}
                    </span>
                  )}
                </div>
              </CardContent>
            </Card>
          </div>
        }
      >
        <div className="space-y-6">
          <p className="text-caption text-muted-foreground">{t("detail.pageDurationHint")}</p>
          <PageDurationChart
            title={t("detail.pageDurationTitle")}
            data={pageDurationData}
            emptyDescription={t("detail.trendEmpty")}
            formatValue={(v) => formatDuration(v)}
            xAxisTitle={t("detail.pageAxisTitle")}
            yAxisTitle={t("detail.durationAxisTitle")}
            tooltipName={t("detail.avgDurationTooltip")}
            pageLabel={(page) => t("detail.pageLabel", { page })}
          />
          <Card>
            <CardHeader>
              <CardTitle className="text-h3">{t("detail.timelineTitle")}</CardTitle>
            </CardHeader>
            <CardContent>
              <ActivityTimeline activities={timelineActivities} />
            </CardContent>
          </Card>
        </div>
      </DetailLayout>

      {link.dealRoomId ? (
        <Card>
          <CardHeader>
            <CardTitle className="text-h2 flex items-center gap-2">
              <ChatTeardropText size={20} />
              {tShare("management.questionsTitle")}
            </CardTitle>
            <CardDescription>{tShare("management.questionsDescription")}</CardDescription>
          </CardHeader>
          <CardContent>
            <OwnerAskInboxPanel
              scope={{ type: "link", linkId: link.id }}
              i18nNs="linkShare"
              initialView={askInboxView}
              canManageAsk={canManageAskHost({
                dealRoomId: link.dealRoomId,
                workspaceCanManage: canManage,
                linkCanManageAsk: link.canManageAsk,
              })}
            />
          </CardContent>
        </Card>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle className="text-h2">{t("detail.accessLogTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          <LinkAccessLog
            logs={logs}
            documents={link.documents}
            primaryDocumentId={link.documentId}
          />
        </CardContent>
      </Card>
    </div>
  );
}
