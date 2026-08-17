import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { FileText } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { DocumentAnalytics } from "@/components/documents/DocumentAnalytics";
import { DocumentVisitorsCard } from "@/components/documents/DocumentVisitorsCard";
import { InsightsDocumentPicker } from "@/components/insights/InsightsDocumentPicker";
import {
  InsightsRangeControls,
  useInsightsRange,
} from "@/components/insights/InsightsRangeControls";
import { ReadingFunnelCard } from "@/components/insights/ReadingFunnelCard";
import { ReadingSessionsCard } from "@/components/insights/ReadingSessionsCard";
import { EmptyState } from "@/components/common/EmptyState";
import { useAsyncData } from "@/hooks/useAsyncData";
import { api } from "@/lib/api";
import { documentDetailPath } from "@/lib/documentDetailNav";
import { mergeInsightDocuments } from "@/lib/insightsDocumentPicker";
import { useTranslation } from "react-i18next";

export {
  activeInsightRooms,
  collectDealRoomDocumentIds,
  documentTitle,
  filterInsightDocuments,
  filterInsightRooms,
  insightDocScope,
  insightRoomPickerMode,
  mergeInsightDocuments,
} from "@/lib/insightsDocumentPicker";
export type { InsightDocScope, InsightRoomPickerMode } from "@/lib/insightsDocumentPicker";

export function InsightsPagesPage() {
  const { t } = useTranslation("insights");
  const { t: tc } = useTranslation("common");
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const [selectedDocId, setSelectedDocId] = useState("");
  const rangeCtl = useInsightsRange(30);
  const {
    data: documents,
    loading: loadingDocs,
    error,
    refetch,
  } = useAsyncData(async () => {
    // Insights covers library + data-room docs; agreements stay in the agreements surface.
    const [generalRes, dealRoomRes] = await Promise.all([
      api.getDocuments("all", "general"),
      api.getDocuments("all", "deal_room"),
    ]);
    return mergeInsightDocuments(generalRes.data, dealRoomRes.data);
  }, []);

  useEffect(() => {
    if (!documents?.length) {
      setSelectedDocId("");
      return;
    }
    setSelectedDocId((prev) => {
      if (prev && documents.some((d) => d.id === prev)) return prev;
      return documents[0]?.id || "";
    });
  }, [documents]);

  const {
    data: analytics,
    loading: loadingAnalytics,
  } = useAsyncData(
    async () => {
      if (!selectedDocId) return [];
      const res = await api.getPageAnalytics(selectedDocId, rangeCtl.apiParams);
      return res.data;
    },
    [selectedDocId, rangeCtl.apiParams],
  );

  const {
    data: funnel,
    loading: loadingFunnel,
  } = useAsyncData(
    async () => {
      if (!selectedDocId) return null;
      return api.getDocumentReadingFunnel(selectedDocId, rangeCtl.apiParams);
    },
    [selectedDocId, rangeCtl.apiParams],
  );

  const {
    data: sessions,
    loading: loadingSessions,
  } = useAsyncData(
    async () => {
      if (!selectedDocId) return null;
      return api.getDocumentReadingSessions(selectedDocId, 40, rangeCtl.apiParams);
    },
    [selectedDocId, rangeCtl.apiParams],
  );

  const {
    data: visitors,
    loading: loadingVisitors,
  } = useAsyncData(
    async () => {
      if (!selectedDocId) return [];
      const res = await api.getDocumentVisitors(selectedDocId, rangeCtl.apiParams);
      return res.data;
    },
    [selectedDocId, rangeCtl.apiParams],
  );

  const openPage =
    workspaceSlug && selectedDocId
      ? (pageNumber: number) =>
          navigate(
            documentDetailPath(workspaceSlug, selectedDocId, {
              tab: "content",
              page: pageNumber,
            }),
          )
      : undefined;

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
        <p className="text-body text-muted-foreground">{error}</p>
        <Button onClick={refetch}>{tc("retry")}</Button>
      </div>
    );
  }

  if (loadingDocs) {
    return <Skeleton className="h-80" />;
  }

  if (!documents || documents.length === 0) {
    return (
      <EmptyState
        icon={<FileText size={48} />}
        title={t("pages.emptyTitle")}
        description={t("pages.emptyDescription")}
      />
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <InsightsRangeControls
          variant="chips"
          className="w-full sm:w-auto sm:max-w-xl"
          range={rangeCtl.range}
          customOpen={rangeCtl.customOpen}
          draftFrom={rangeCtl.draftFrom}
          draftTo={rangeCtl.draftTo}
          rangeError={rangeCtl.rangeError}
          onSelectPreset={rangeCtl.selectPreset}
          onOpenCustom={rangeCtl.openCustom}
          onDraftFromChange={rangeCtl.setDraftFrom}
          onDraftToChange={rangeCtl.setDraftTo}
          onApplyCustom={rangeCtl.applyCustom}
        />

        <InsightsDocumentPicker
          documents={documents}
          selectedDocId={selectedDocId}
          onSelectedDocIdChange={setSelectedDocId}
        />
      </div>
      <p className="text-caption text-muted-foreground">{t("pages.actorHint")}</p>

      <ReadingFunnelCard
        funnel={funnel}
        loading={loadingFunnel}
        onOpenPage={openPage}
      />

      <ReadingSessionsCard
        data={sessions}
        loading={loadingSessions}
        onOpenPage={openPage}
      />

      {loadingVisitors ? (
        <Skeleton className="h-48" />
      ) : (
        <DocumentVisitorsCard
          visitors={visitors ?? []}
          title={t("pages.visitorsTitle")}
          linkToContacts={false}
          emptyTitle={t("pages.visitorsEmpty")}
          emptyDescription={t("pages.noAnalyticsDescription")}
          anonymousLabel={t("pages.anonymousVisitor")}
          metaLabel={({ pages, duration }) =>
            t("pages.visitorMeta", { pages, duration })
          }
        />
      )}

      {loadingAnalytics ? (
        <Skeleton className="h-80" />
      ) : analytics?.length === 0 ? (
        <EmptyState
          icon={<FileText size={48} />}
          title={t("pages.noAnalyticsTitle")}
          description={t("pages.noAnalyticsDescription")}
        />
      ) : (
        <DocumentAnalytics
          analytics={analytics ?? []}
          variant="detail"
          onOpenPage={openPage}
        />
      )}
    </div>
  );
}
