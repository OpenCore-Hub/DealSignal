import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { Copy, PencilSimple, ToggleRight, FileText } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { PageHeader } from "@/components/common/PageHeader";
import { BackButton } from "@/components/common/BackButton";
import { DetailLayout } from "@/components/common/DetailLayout";
import { StatCard } from "@/components/common/StatCard";
import { ActivityTimeline } from "@/components/common/ActivityTimeline";
import { PageDurationChart } from "@/components/common/PageDurationChart";
import { PermissionBadge } from "@/components/common/PermissionBadge";
import { SkeletonDetail } from "@/components/common/SkeletonLayout";
import { LinkAccessLog } from "./LinkAccessLog";
import { copyToClipboard } from "@/lib/clipboard";
import { api } from "@/lib/api";
import { formatDate, formatDuration, formatRelativeTime } from "@/lib/formatters";
import { calculateUniqueVisitors } from "@/lib/calculations";
import type { AccessLog, Document, Link } from "@/types";

function buildPageDurationData(
  logs: AccessLog[],
  pageCount: number
): { page: number; duration: number }[] {
  const groups = new Map<number, { total: number; count: number }>();
  for (const log of logs) {
    if (typeof log.pageNumber !== "number") continue;
    const existing = groups.get(log.pageNumber);
    if (existing) {
      existing.total += log.durationSeconds || 0;
      existing.count += 1;
    } else {
      groups.set(log.pageNumber, { total: log.durationSeconds || 0, count: 1 });
    }
  }

  const data: { page: number; duration: number }[] = [];
  for (let page = 1; page <= pageCount; page++) {
    const existing = groups.get(page);
    data.push({
      page,
      duration: existing ? Math.round(existing.total / existing.count) : 0,
    });
  }
  return data;
}

export function LinkDetail() {
  const navigate = useNavigate();
  const { workspaceSlug, linkId } = useParams<{ workspaceSlug: string; linkId: string }>();
  const { t } = useTranslation("links");
  const { t: tc } = useTranslation("common");
  const [link, setLink] = useState<Link | null>(null);
  const [document, setDocument] = useState<Document | null>(null);
  const [logs, setLogs] = useState<AccessLog[]>([]);
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
        const [logData, docData] = await Promise.all([
          api.getAccessLogs(id!),
          api.getDocumentById(l.documentId),
        ]);
        if (!cancelled) {
          setLink(l);
          setDocument(docData);
          setLogs(logData.data);
        }
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : tc("error.loadFailed"));
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
    const pageCount =
      document?.pageCount ??
      Math.max(0, ...logs.filter((l) => typeof l.pageNumber === "number").map((l) => l.pageNumber ?? 0));
    if (pageCount <= 0) return [];
    return buildPageDurationData(logs, pageCount);
  }, [logs, document]);

  const timelineActivities = useMemo(() => {
    return [...logs]
      .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
      .slice(0, 20)
      .map((log) => ({
        id: log.id,
        time: formatRelativeTime(log.timestamp),
        title: log.pageNumber
          ? t("timeline.viewedPage", { visitor: log.visitorName || log.visitorEmail || tc("visitor.unknown"), page: log.pageNumber })
          : t("timeline.viewedLink", { visitor: log.visitorName || log.visitorEmail || tc("visitor.unknown") }),
        description: log.durationSeconds
          ? t("timeline.description", { duration: formatDuration(log.durationSeconds), device: log.device || "", location: log.location || "" })
          : undefined,
      }));
  }, [logs, t, tc]);

  if (error) {
    return (
      <div className="space-y-6">
        <BackButton to={`/${workspaceSlug}/links`} label={t("backToLinks")} />
        <Card>
          <CardContent className="py-12 text-center">
            <p className="text-body text-destructive mb-4">{tc("error.loadFailed")}{error ? `: ${error}` : ""}</p>
            <Button onClick={() => setRetryTick((t) => t + 1)}>{tc("retry")}</Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (loading || !link) return <SkeletonDetail />;

  return (
    <div className="space-y-6">
      <BackButton to={`/${workspaceSlug}/links`} label={t("backToLinks")} />

      <PageHeader
        title={(link.shortUrl || link.id).split("/").pop() || link.id}
        description={t("detail.headerDescription", { doc: link.documentTitle, date: formatDate(link.createdAt) })}
      >
        <Button
          variant="outline"
          className="gap-1.5"
          onClick={() => navigate(`/${workspaceSlug}/links/${link.id}/edit`)}
        >
          <PencilSimple size={16} />
          {tc("edit")}
        </Button>
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
            <StatCard size="sm" label={t("detail.uniqueVisitors")} value={calculateUniqueVisitors(logs)} />
            <StatCard size="sm" label={t("detail.avgDuration")} value={formatDuration(link.avgDurationSeconds || 0)} />
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

      <Card>
        <CardHeader>
          <CardTitle className="text-h2">{t("detail.accessLogTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          <LinkAccessLog logs={logs} />
        </CardContent>
      </Card>
    </div>
  );
}
