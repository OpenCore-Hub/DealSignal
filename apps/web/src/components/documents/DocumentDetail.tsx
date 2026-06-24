import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { DownloadSimple, Eye, Link as LinkIcon, Trash } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { PageHeader } from "@/components/common/PageHeader";
import { BackButton } from "@/components/common/BackButton";
import { DetailLayout } from "@/components/common/DetailLayout";
import { RowActions } from "@/components/common/RowActions";
import { SkeletonDetail } from "@/components/common/SkeletonLayout";
import { DocumentAnalytics } from "./DocumentAnalytics";
import { DocumentContent } from "./DocumentContent";
import { DocumentAIInsights } from "./DocumentAIInsights";
import { DocumentStats } from "./DocumentStats";
import { DocumentVisitorsCard } from "./DocumentVisitorsCard";
import { DocumentLinksCard } from "./DocumentLinksCard";
import { DeleteDocumentDialog } from "./DeleteDocumentDialog";
import { api } from "@/lib/api";
import { formatFileSize, formatRelativeTime } from "@/lib/formatters";
import { toast } from "sonner";
import type { Document, Link, PageAnalytics, AccessLog } from "@/types";

export function DocumentDetail() {
  const navigate = useNavigate();
  const { workspaceSlug, documentId } = useParams<{ workspaceSlug: string; documentId: string }>();
  const { t } = useTranslation(["documents", "common"]);
  const [doc, setDoc] = useState<Document | null>(null);
  const [links, setLinks] = useState<Link[]>([]);
  const [analytics, setAnalytics] = useState<PageAnalytics[]>([]);
  const [logs, setLogs] = useState<AccessLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [retryTick, setRetryTick] = useState(0);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const id = documentId;
    if (!id) return;
    async function load() {
      try {
        setLoading(true);
        setError(null);
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
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : t("common:error.loadFailed"));
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [documentId, retryTick, t]);

  if (error) {
    return (
      <div className="space-y-6">
        <BackButton to={`/${workspaceSlug}/documents`} label={t("documents:detail.back")} />
        <div className="rounded-xl border border-border bg-card py-12 text-center">
          <p className="text-body text-destructive mb-4">
            {t("documents:detail.loadFailed", { error })}
          </p>
          <Button onClick={() => setRetryTick((t) => t + 1)}>{t("common:retry")}</Button>
        </div>
      </div>
    );
  }

  if (loading || !doc) return <SkeletonDetail />;

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
        <RowActions
          actions={[
            {
              label: t("common:download"),
              icon: <DownloadSimple size={16} />,
              onClick: async () => {
                try {
                  const res = await api.getDocumentDownloadUrl(doc.id);
                  const a = document.createElement("a");
                  a.href = res.download_url;
                  a.download = res.filename || doc.title;
                  a.target = "_blank";
                  a.rel = "noopener noreferrer";
                  document.body.appendChild(a);
                  a.click();
                  a.remove();
                } catch {
                  toast.error(t("common:error.loadFailed"));
                }
              },
            },
            {
              label: t("common:delete"),
              icon: <Trash size={16} />,
              onClick: () => setDeleteDialogOpen(true),
              destructive: true,
              pro: true,
            },
          ]}
        />
      </PageHeader>

      <DetailLayout sidebar={<DocumentStats links={links} logs={logs} />}>
        <Tabs defaultValue="overview" className="w-full">
          <TabsList className="mb-4">
            <TabsTrigger value="overview">{t("documents:detail.tabs.overview")}</TabsTrigger>
            <TabsTrigger value="content">{t("documents:detail.tabs.content")}</TabsTrigger>
            <TabsTrigger value="analytics">{t("documents:detail.tabs.analytics")}</TabsTrigger>
            <TabsTrigger value="ai">{t("documents:detail.tabs.ai")}</TabsTrigger>
          </TabsList>
          <TabsContent value="overview" className="space-y-6">
            <DocumentAnalytics analytics={analytics} />
            <DocumentVisitorsCard logs={logs} />
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

      <DeleteDocumentDialog
        doc={doc}
        workspaceSlug={workspaceSlug!}
        open={deleteDialogOpen}
        onOpenChange={setDeleteDialogOpen}
      />
    </div>
  );
}
