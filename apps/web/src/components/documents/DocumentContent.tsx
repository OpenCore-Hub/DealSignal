import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router";
import { Eye, FileText, MagnifyingGlassPlus } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { DocumentPageGrid } from "./DocumentPageGrid";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { formatDuration } from "@/lib/formatters";
import { useAsyncData } from "@/hooks/useAsyncData";
import { buildPageGridItems, pageAspectRatio } from "@/lib/projectPageGrid";
import type { PageAnalytics } from "@/types";

interface DocumentContentProps {
  title: string;
  pageCount: number;
  documentId: string;
  analytics: PageAnalytics[];
  /** Page to highlight from analytics / URL deep link. */
  focusPage?: number | null;
  onFocusPageChange?: (pageNumber: number) => void;
}

export function DocumentContent({
  title,
  pageCount,
  documentId,
  analytics,
  focusPage = null,
  onFocusPageChange,
}: DocumentContentProps) {
  const { t, i18n } = useTranslation(["documents", "common"]);
  const navigate = useNavigate();
  const [selectedPage, setSelectedPage] = useState<number | null>(focusPage);
  const [pageImageUrl, setPageImageUrl] = useState<string | null>(null);
  const [loadingImageUrl, setLoadingImageUrl] = useState(false);

  useEffect(() => {
    if (focusPage == null) return;
    setSelectedPage(focusPage);
  }, [focusPage]);

  const pages = useMemo(
    () => buildPageGridItems(pageCount, analytics),
    [analytics, pageCount],
  );

  const { data: pageMeta } = useAsyncData(
    async () => api.getDocumentPages(documentId),
    [documentId],
  );
  const documentAspectRatio = useMemo(() => {
    const first = pageMeta?.pages?.[0];
    return pageAspectRatio(first?.width, first?.height);
  }, [pageMeta]);

  const selectedAnalytic = useMemo(() => {
    if (!selectedPage) return null;
    return pages.find((p) => p.pageNumber === selectedPage) ?? null;
  }, [pages, selectedPage]);

  useEffect(() => {
    if (!selectedPage || !documentId) return;
    const controller = new AbortController();
    // eslint-disable-next-line react-hooks/set-state-in-effect -- loading state for async fetch
    setLoadingImageUrl(true);
    api
      .getPageSignedUrl(documentId, selectedPage, { signal: controller.signal })
      .then((res) => {
        if (!controller.signal.aborted) setPageImageUrl(res.image_url);
      })
      .catch(() => {
        if (!controller.signal.aborted) setPageImageUrl(null);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoadingImageUrl(false);
      });
    return () => {
      controller.abort();
    };
  }, [selectedPage, documentId]);

  const handleSelectPage = (pageNumber: number) => {
    setSelectedPage(pageNumber);
    onFocusPageChange?.(pageNumber);
  };

  const durationLabel = formatDuration(
    selectedAnalytic?.avgDurationSeconds ?? 0,
    i18n.language,
  );
  const exitPercent = Math.round((selectedAnalytic?.exitRate ?? 0) * 100);

  return (
    <div className="space-y-4" data-testid="document-content">
      <DocumentPageGrid
        items={pages}
        documentId={documentId}
        selectedPage={selectedPage}
        focusPage={focusPage ?? selectedPage}
        aspectRatio={documentAspectRatio}
        onSelectPage={handleSelectPage}
      />

      {selectedPage && (
        <Card data-testid="document-content-page-detail">
          <CardContent>
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <p className="text-h3 flex items-center gap-2">
                  <MagnifyingGlassPlus size={18} />
                  {t("documents:content.pageDetailTitle", { pageNumber: selectedPage })}
                </p>
                <p className="text-body mt-1 text-muted-foreground">{title}</p>
              </div>
              <Button
                type="button"
                size="sm"
                className="gap-1.5"
                onClick={() => navigate(`/viewer/${documentId}?page=${selectedPage}`)}
              >
                <Eye size={15} />
                {t("documents:content.openPreview")}
              </Button>
            </div>
            <div className="mt-4 grid grid-cols-3 gap-4 text-sm">
              <div>
                <p className="text-caption text-muted-foreground">
                  {t("documents:content.viewCount")}
                </p>
                <p className="text-h2">{selectedAnalytic?.viewCount ?? 0}</p>
              </div>
              <div>
                <p className="text-caption text-muted-foreground">
                  {t("documents:content.avgDuration")}
                </p>
                <p className="text-h2">
                  {t("documents:content.avgDurationValue", { duration: durationLabel })}
                </p>
              </div>
              <div>
                <p className="text-caption text-muted-foreground">
                  {t("documents:content.exitRate")}
                </p>
                <p className="text-h2">
                  {t("documents:content.exitRateValue", { percent: exitPercent })}
                </p>
              </div>
            </div>

            <div className="mt-4 flex justify-center rounded-md border border-border bg-muted/30 p-4">
              {loadingImageUrl ? (
                <Skeleton className="h-[400px] w-[300px]" />
              ) : pageImageUrl ? (
                <img
                  src={pageImageUrl}
                  alt={t("documents:content.pageLabel", { pageNumber: selectedPage })}
                  className="max-h-[600px] w-auto rounded-md shadow-sm"
                />
              ) : (
                <div className="flex h-[400px] w-[300px] flex-col items-center justify-center text-muted-foreground">
                  <FileText size={48} className="text-muted-foreground/50" />
                  <p className="mt-2 text-sm">
                    {t("documents:content.pageLabel", { pageNumber: selectedPage })}
                  </p>
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
