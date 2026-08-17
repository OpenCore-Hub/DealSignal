import { useCallback, useEffect, useState } from "react";
import { HeatBreakdownDialog } from "@/components/insights/HeatBreakdownDialog";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { Buildings, Eye, Link as LinkIcon, Scales } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { SmartBackButton } from "@/components/common/SmartBackButton";
import { SkeletonDetail } from "@/components/common/SkeletonLayout";
import { DocumentAnalytics } from "./DocumentAnalytics";
import { DocumentContent } from "./DocumentContent";
import { DocumentInsights } from "./DocumentInsights";
import { DocumentStats } from "./DocumentStats";
import { DocumentVisitorsCard } from "./DocumentVisitorsCard";
import { DocumentLinksCard } from "./DocumentLinksCard";
import { AddToDealRoomDialog } from "./AddToDealRoomDialog";
import { DocumentCategoryBadge } from "./DocumentCategoryBadge";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import { useAsyncData } from "@/hooks/useAsyncData";
import { useWorkspaceAccess } from "@/hooks/useWorkspaceAccess";
import {
  isLegacyDocumentDetailTab,
  parseDocumentDetailTab,
  parseDocumentFocusPage,
  patchDocumentDetailSearchParams,
  type DocumentDetailTab,
} from "@/lib/documentDetailNav";
import {
  canAddDocumentToDealRoom,
  canToggleAgreementCategory,
  isAgreementCategory,
  isDealRoomCategory,
  agreementCategoryErrorCode,
} from "@/lib/documentCategory";
import { formatFileSize, formatRelativeTime } from "@/lib/formatters";
import { cn } from "@/lib/utils";
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
  const [searchParams, setSearchParams] = useSearchParams();
  const { t } = useTranslation(["documents", "common", "agreementDocuments"]);
  const { canWrite } = useWorkspaceAccess(workspaceSlug);
  const [addToRoomOpen, setAddToRoomOpen] = useState(false);
  const [togglingCategory, setTogglingCategory] = useState(false);
  const [heatExplainOpen, setHeatExplainOpen] = useState(false);

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

  const { data: documentHeat } = useAsyncData(async () => {
    if (!documentId || !data || isAgreementCategory(data.doc.category)) return null;
    try {
      return await api.getDocumentHeatScore(documentId);
    } catch {
      return null;
    }
  }, [documentId, data?.doc.category]);

  const rawTab = searchParams.get("tab");
  const tab = parseDocumentDetailTab(rawTab);
  const focusPage =
    tab === "content"
      ? parseDocumentFocusPage(searchParams.get("page"), data?.doc.pageCount)
      : null;

  const setDetailNav = useCallback(
    (patch: { tab?: DocumentDetailTab; page?: number | null }) => {
      setSearchParams(
        (prev) => patchDocumentDetailSearchParams(prev, patch),
        { replace: true },
      );
    },
    [setSearchParams],
  );

  // Rewrite legacy ?tab=ai → insights so bookmarks stay valid.
  useEffect(() => {
    if (!isLegacyDocumentDetailTab(rawTab)) return;
    setDetailNav({ tab });
  }, [rawTab, tab, setDetailNav]);

  const openContentPage = useCallback(
    (pageNumber: number) => {
      setDetailNav({ tab: "content", page: pageNumber });
    },
    [setDetailNav],
  );

  if (error) {
    return (
      <div className="mx-auto max-w-6xl space-y-6">
        <SmartBackButton fallbackTo={`/${workspaceSlug}/documents`} fallbackLabel={t("documents:detail.back")} />
        <div className="rounded-2xl border border-border/70 bg-background py-12 text-center">
          <p className="mb-4 text-body text-destructive">
            {error || t("documents:detail.loadFailed")}
          </p>
          <Button onClick={refetch}>{t("common:retry")}</Button>
        </div>
      </div>
    );
  }

  if (loading || !data) return <SkeletonDetail />;

  const { doc, links, analytics, visitors } = data;

  const isAgreement = isAgreementCategory(doc.category);
  const isDealRoomDoc = isDealRoomCategory(doc.category);
  const canMarkAgreement = canToggleAgreementCategory(doc.category, {
    fileType: doc.fileType,
    sourceType: doc.sourceType,
  });
  const canAddToDealRoom = canAddDocumentToDealRoom(doc.category);
  const busyDoc = doc.status === "uploading" || doc.status === "processing" || doc.status === "failed";

  const handleToggleCategory = async () => {
    if (!doc || !documentId || isDealRoomDoc) return;
    const newCategory = isAgreement ? "general" : "agreement";
    if (newCategory === "agreement" && !canMarkAgreement) {
      toast.error(t("agreementDocuments:page.pdfOnly"));
      return;
    }
    setTogglingCategory(true);
    try {
      await api.updateDocumentCategory(documentId, newCategory);
      refetch();
    } catch (err) {
      if (err instanceof ApiError && err.code === "agreement_pdf_required") {
        toast.error(t("agreementDocuments:page.pdfOnly"));
      } else if (
        err instanceof ApiError &&
        agreementCategoryErrorCode(err.code)
      ) {
        toast.error(t(`documents:detail.categoryErrors.${err.code}`));
      } else {
        toast.error(t("common:error.saveFailed"));
      }
    } finally {
      setTogglingCategory(false);
    }
  };

  return (
    <div className="mx-auto max-w-6xl space-y-8">
      <SmartBackButton fallbackTo={`/${workspaceSlug}/documents`} fallbackLabel={t("documents:detail.back")} />

      <header className="grid gap-6 border-b border-border/60 pb-7 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end">
        <div className="min-w-0 space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            <p className="text-[11px] font-medium uppercase tracking-[0.16em] text-muted-foreground/75">
              {doc.fileType.toUpperCase()}
            </p>
            <DocumentCategoryBadge category={doc.category} />
          </div>
          <h1 className="text-balance text-[1.85rem] font-semibold leading-[1.15] tracking-[-0.03em] text-foreground sm:text-[2.15rem]">
            {doc.title}
          </h1>
          <p className="text-[13px] leading-relaxed text-muted-foreground">
            {t("documents:detail.meta", {
              fileType: doc.fileType.toUpperCase(),
              pageCount: doc.pageCount,
              fileSize: formatFileSize(doc.fileSize),
              createdAt: formatRelativeTime(doc.createdAt),
            })}
          </p>
        </div>

        <div className="flex flex-col gap-2 sm:items-end">
          <div className="flex flex-wrap items-center gap-2 sm:justify-end">
            <Button
              variant="outline"
              size="sm"
              className="gap-1.5"
              onClick={() => navigate(`/viewer/${doc.id}`)}
            >
              <Eye size={15} />
              {t("common:preview")}
            </Button>
            {canWrite ? (
              <Button
                size="sm"
                className="gap-1.5"
                onClick={() => navigate(`/${workspaceSlug}/links/new?documentId=${doc.id}`)}
              >
                <LinkIcon size={15} />
                {t("common:createLink")}
              </Button>
            ) : null}
          </div>
          <div className="flex flex-wrap items-center gap-1.5 sm:justify-end">
            {canWrite && canAddToDealRoom ? (
              <Button
                variant="ghost"
                size="sm"
                className="gap-1.5 text-muted-foreground hover:text-foreground"
                onClick={() => setAddToRoomOpen(true)}
                disabled={busyDoc}
              >
                <Buildings size={15} />
                {t("common:addToDealRoom")}
              </Button>
            ) : null}
            {canWrite ? (
              <Button
                variant="ghost"
                size="sm"
                className={cn(
                  "gap-1.5 text-muted-foreground hover:text-foreground",
                  isAgreement && "bg-foreground/[0.04] text-foreground",
                )}
                onClick={() => { void handleToggleCategory(); }}
                disabled={togglingCategory || isDealRoomDoc || (!isAgreement && !canMarkAgreement)}
                title={
                  isDealRoomDoc
                    ? t("documents:detail.categoryErrors.category_immutable")
                    : !isAgreement && !canMarkAgreement
                      ? t("agreementDocuments:page.pdfOnly")
                      : undefined
                }
              >
                <Scales size={15} />
                {isAgreement
                  ? t("agreementDocuments:page.unsetAsAgreement")
                  : t("agreementDocuments:page.setAsAgreement")}
              </Button>
            ) : null}
          </div>
        </div>
      </header>

      <DocumentStats
        links={links}
        visitors={visitors}
        pages={analytics}
        heat={
          documentHeat
            ? { level: documentHeat.level, score: documentHeat.score }
            : null
        }
        onExplainHeat={documentHeat ? () => setHeatExplainOpen(true) : undefined}
      />
      <HeatBreakdownDialog
        open={heatExplainOpen}
        onOpenChange={setHeatExplainOpen}
        kind="document"
        entityId={documentId}
        label={doc.title}
      />

      <Tabs
        value={tab}
        onValueChange={(value) => {
          const next = parseDocumentDetailTab(value);
          // page is only meaningful on the content tab; drop it elsewhere.
          setDetailNav(
            next === "content" ? { tab: next } : { tab: next, page: null },
          );
        }}
        className="w-full gap-5"
      >
        <TabsList variant="line" className="mb-1 h-auto w-full justify-start gap-5 border-b border-border/60 pb-0">
          <TabsTrigger
            value="overview"
            className="rounded-none px-0 pb-2.5 text-[13px] data-active:shadow-none"
          >
            {t("documents:detail.tabs.overview")}
          </TabsTrigger>
          <TabsTrigger
            value="content"
            className="rounded-none px-0 pb-2.5 text-[13px] data-active:shadow-none"
          >
            {t("documents:detail.tabs.content")}
          </TabsTrigger>
          <TabsTrigger
            value="analytics"
            className="rounded-none px-0 pb-2.5 text-[13px] data-active:shadow-none"
          >
            {t("documents:detail.tabs.analytics")}
          </TabsTrigger>
          <TabsTrigger
            value="insights"
            className="rounded-none px-0 pb-2.5 text-[13px] data-active:shadow-none"
          >
            {t("documents:detail.tabs.insights")}
          </TabsTrigger>
        </TabsList>
        <TabsContent value="overview" className="space-y-5">
          <DocumentAnalytics
            key={`${doc.id}-overview`}
            analytics={analytics}
            variant="overview"
            onOpenPage={openContentPage}
          />
          <DocumentInsights
            key={`${doc.id}-insights-overview`}
            analytics={analytics}
            onOpenPage={openContentPage}
          />
          <DocumentVisitorsCard visitors={visitors} />
          <DocumentLinksCard doc={doc} links={links} workspaceSlug={workspaceSlug!} />
        </TabsContent>
        <TabsContent value="content">
          <DocumentContent
            title={doc.title}
            pageCount={doc.pageCount}
            documentId={doc.id}
            analytics={analytics}
            focusPage={focusPage}
            onFocusPageChange={(page) => setDetailNav({ tab: "content", page })}
          />
        </TabsContent>
        <TabsContent value="analytics">
          <DocumentAnalytics
            key={`${doc.id}-detail`}
            analytics={analytics}
            variant="detail"
            onOpenPage={openContentPage}
          />
        </TabsContent>
        <TabsContent value="insights">
          <DocumentInsights
            key={`${doc.id}-insights`}
            analytics={analytics}
            onOpenPage={openContentPage}
          />
        </TabsContent>
      </Tabs>

      <AddToDealRoomDialog
        documentId={doc.id}
        documentTitle={doc.title}
        open={addToRoomOpen}
        onOpenChange={setAddToRoomOpen}
      />
    </div>
  );
}
