import { useCallback, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { Buildings, Eye, Link as LinkIcon } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { PageHeader } from "@/components/common/PageHeader";
import { BackButton } from "@/components/common/BackButton";
import { DetailLayout } from "@/components/common/DetailLayout";
import { SkeletonDetail } from "@/components/common/SkeletonLayout";
import { DocumentAnalytics } from "./DocumentAnalytics";
import { DocumentContent } from "./DocumentContent";
import { DocumentAIInsights } from "./DocumentAIInsights";
import { DocumentStats } from "./DocumentStats";
import { DocumentVisitorsCard } from "./DocumentVisitorsCard";
import { DocumentLinksCard } from "./DocumentLinksCard";
import { AddToDealRoomDialog } from "./AddToDealRoomDialog";
import { api } from "@/lib/api";
import { useAsyncData } from "@/hooks/useAsyncData";
import { formatFileSize, formatRelativeTime } from "@/lib/formatters";
import type { Document, Link, PageAnalytics, VisitorSummary } from "@/types";

interface DocumentDetailData {
  doc: Document;
  links: Link[];
  analytics: PageAnalytics[];
  visitors: VisitorSummary[];
}

export function DocumentDetail() {
  const navigate = useNavigate();
  const { workspaceSlug, documentId } = useParams<{ workspaceSlug: string; documentId: string }>();
  const { t } = useTranslation(["documents", "common"]);
  const [addToRoomOpen, setAddToRoomOpen] = useState(false);

  const loadDetail = useCallback(async (): Promise<DocumentDetailData> => {
    if (!documentId) {
      throw new Error(t("documents:detail.notFound"));
    }
    const [d, l, a, v] = await Promise.all([
      api.getDocumentById(documentId),
      api.getLinksByDocumentId(documentId),
      api.getPageAnalytics(documentId),
      api.getDocumentVisitors(documentId),
    ]);
    return { doc: d, links: l.data, analytics: a.data, visitors: v.data };
  }, [documentId, t]);

  const { data, loading, error, refetch } = useAsyncData(loadDetail, [loadDetail]);

  if (error) {
    return (
      <div className="space-y-6">
        <BackButton to={`/${workspaceSlug}/documents`} label={t("documents:detail.back")} />
        <div className="rounded-xl border border-border bg-card py-12 text-center">
          <p className="text-body text-destructive mb-4">
            {t("documents:detail.loadFailed", { error })}
          </p>
          <Button onClick={refetch}>{t("common:retry")}</Button>
        </div>
      </div>
    );
  }

  if (loading || !data) return <SkeletonDetail />;

  const { doc, links, analytics, visitors } = data;

  return (
    <div className="space-y-6">
      <BackButton to={`/${workspaceSlug}/documents`} label={t("documents:detail.back")} />

      <PageHeader
        title={doc.title}
        description={t("documents:detail.meta", {
          fileType: doc.fileType.toUpperCase(),
          pageCount: doc.pageCount,
          fileSize: formatFileSize(doc.fileSize),
          createdAt: formatRelativeTime(doc.createdAt),
        })}
      >
        <Button variant="outline" className="gap-1.5" onClick={() => navigate(`/viewer/${doc.id}`)}>
          <Eye size={16} />
          {t("common:preview")}
        </Button>
        <Button className="gap-1.5" onClick={() => navigate(`/${workspaceSlug}/links/new?documentId=${doc.id}`)}>
          <LinkIcon size={16} />
          {t("common:createLink")}
        </Button>
        <Button
          variant="outline"
          className="gap-1.5"
          onClick={() => setAddToRoomOpen(true)}
          disabled={doc.status === "uploading" || doc.status === "processing" || doc.status === "failed"}
        >
          <Buildings size={16} />
          {t("common:addToDealRoom")}
        </Button>
      </PageHeader>

      <DetailLayout sidebar={<DocumentStats links={links} visitors={visitors} />}>
        <Tabs defaultValue="overview" className="w-full">
          <TabsList className="mb-4">
            <TabsTrigger value="overview">{t("documents:detail.tabs.overview")}</TabsTrigger>
            <TabsTrigger value="content">{t("documents:detail.tabs.content")}</TabsTrigger>
            <TabsTrigger value="analytics">{t("documents:detail.tabs.analytics")}</TabsTrigger>
            <TabsTrigger value="ai">{t("documents:detail.tabs.ai")}</TabsTrigger>
          </TabsList>
          <TabsContent value="overview" className="space-y-6">
            <DocumentAnalytics analytics={analytics} />
            <DocumentVisitorsCard visitors={visitors} />
            <DocumentLinksCard doc={doc} links={links} workspaceSlug={workspaceSlug!} />
          </TabsContent>
          <TabsContent value="content">
            <DocumentContent title={doc.title} pageCount={doc.pageCount} documentId={doc.id} analytics={analytics} evidences={[]} />
          </TabsContent>
          <TabsContent value="analytics">
            <DocumentAnalytics analytics={analytics} />
          </TabsContent>
          <TabsContent value="ai">
            <DocumentAIInsights documentId={doc.id} analytics={analytics} />
          </TabsContent>
        </Tabs>
      </DetailLayout>

      <AddToDealRoomDialog
        documentId={doc.id}
        documentTitle={doc.title}
        open={addToRoomOpen}
        onOpenChange={setAddToRoomOpen}
      />
    </div>
  );
}
