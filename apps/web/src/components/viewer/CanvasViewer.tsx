import { useEffect, useRef, useState } from "react";
import { useParams } from "react-router";
import {
  Download,
  MagnifyingGlassPlus,
  MagnifyingGlassMinus,
  CaretLeft,
  CaretRight,
  FileText,
} from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { formatFileSize, formatDuration } from "@/lib/formatters";
import type { Document, Evidence, PageAnalytics } from "@/types";
import { useAIStore } from "@/stores/aiStore";
import { ThumbnailNav } from "./ThumbnailNav";
import { HighlightOverlay } from "./HighlightOverlay";
import { WatermarkOverlay, type WatermarkInfo } from "./WatermarkOverlay";

const DEFAULT_WATERMARK: WatermarkInfo = {
  email: "viewer@example.test",
};

interface CanvasViewerProps {
  evidence?: Evidence[];
  watermark?: WatermarkInfo | null;
  publicToken?: string;
  publicLink?: { id: string; downloadEnabled: boolean; watermarkEnabled: boolean };
  publicDocument?: Document;
  publicVisitorId?: string;
}

interface PageInfo {
  pageNumber: number;
  width: number;
  height: number;
}

export function CanvasViewer({
  evidence,
  watermark,
  publicToken,
  publicLink,
  publicDocument,
  publicVisitorId,
}: CanvasViewerProps = {}) {
  const { documentId: routeDocumentId } = useParams<{ documentId: string }>();
  const documentId = publicDocument?.id ?? routeDocumentId;
  const { t } = useTranslation(["documents", "common"]);
  const [doc, setDoc] = useState<Document | null>(null);
  const [analytics, setAnalytics] = useState<PageAnalytics[]>([]);
  const [pages, setPages] = useState<PageInfo[]>([]);
  const [imageUrl, setImageUrl] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [retryTick, setRetryTick] = useState(0);
  const [page, setPage] = useState(1);
  const [zoom, setZoom] = useState(100);
  const { highlightedEvidence, highlightedPage } = useAIStore();

  useEffect(() => {
    let cancelled = false;
    const id = documentId;
    if (!id) return;
    async function load() {
      try {
        setLoading(true);
        setError(null);
        if (publicDocument) {
          setDoc(publicDocument);
          setAnalytics([]);
          setPage(1);
          if (publicDocument.status === "ready" && publicToken) {
            const pagesRes = await api.getPublicDocumentPages(id!, publicToken);
            if (!cancelled) {
              setPages(pagesRes.pages);
            }
          } else {
            setPages([]);
          }
        } else {
          const [d, a] = await Promise.all([api.getDocumentById(id!), api.getPageAnalytics(id!)]);
          if (!cancelled) {
            setDoc(d);
            setAnalytics(a.data);
            setPage(1);
            if (d.status === "ready") {
              const pagesRes = await api.getDocumentPages(id!);
              if (!cancelled) {
                setPages(pagesRes.pages);
              }
            } else {
              setPages([]);
            }
          }
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
  }, [documentId, retryTick, t, publicDocument, publicToken]);

  useEffect(() => {
    if (highlightedPage && highlightedPage !== page) {
      setPage(highlightedPage);
    }
  }, [highlightedPage, page]);

  useEffect(() => {
    let cancelled = false;
    const id = documentId;
    if (!id || pages.length === 0) {
      setImageUrl(null);
      return;
    }
    async function loadSignedUrl() {
      try {
        const res = publicToken
          ? await api.getPublicPageSignedUrl(id!, publicToken, page)
          : await api.getPageSignedUrl(id!, page);
        if (!cancelled) setImageUrl(res.image_url);
      } catch (e) {
        if (!cancelled) setImageUrl(null);
      }
    }
    loadSignedUrl();
    return () => {
      cancelled = true;
    };
  }, [documentId, page, pages.length, publicToken]);

  const pageStartRef = useRef<number>(Date.now());
  useEffect(() => {
    if (!publicToken || !documentId) return;
    pageStartRef.current = Date.now();
    return () => {
      const duration = Math.max(0, Math.round((Date.now() - pageStartRef.current) / 1000));
      if (duration > 0) {
        void api.recordPublicEvent({
          event_type: "page_viewed",
          public_token: publicToken,
          visitor_id: publicVisitorId,
          page_number: page,
          duration_seconds: duration,
        });
      }
    };
  }, [publicToken, documentId, page, publicVisitorId]);

  const handleDownload = async () => {
    if (!documentId || !doc) return;
    try {
      const res = publicToken
        ? await api.getPublicDocumentDownloadUrl(documentId, publicToken)
        : await api.getDocumentDownloadUrl(documentId);
      if (publicToken && publicLink && !publicLink.downloadEnabled) {
        setError(t("documents:viewer.downloadDisabled"));
        return;
      }
      const a = document.createElement("a");
      a.href = res.download_url;
      a.download = res.filename || doc.title;
      a.target = "_blank";
      a.rel = "noopener noreferrer";
      document.body.appendChild(a);
      a.click();
      a.remove();
      if (publicToken) {
        void api.recordPublicEvent({
          event_type: "download_attempted",
          public_token: publicToken,
          visitor_id: publicVisitorId,
        });
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : t("common:error.loadFailed"));
    }
  };

  if (loading) {
    return (
      <div className="flex flex-1 flex-col bg-neutral-50 dark:bg-background">
        <header className="flex h-14 items-center border-b border-border bg-background px-4">
          <Skeleton className="h-8 w-64" />
        </header>
        <div className="flex flex-1">
          <Skeleton className="m-8 h-full w-48" />
          <Skeleton className="m-8 h-full flex-1" />
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center gap-4 bg-neutral-50 dark:bg-background">
        <FileText size={48} className="text-muted-foreground/50" />
        <p className="text-body text-destructive">{t("documents:viewer.loadFailed", { error })}</p>
        <Button onClick={() => setRetryTick((x) => x + 1)}>{t("common:retry")}</Button>
      </div>
    );
  }

  if (!doc) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center bg-neutral-50 dark:bg-background">
        <FileText size={48} className="text-muted-foreground/50" />
        <p className="mt-4 text-body text-muted-foreground">{t("documents:viewer.notFound")}</p>
      </div>
    );
  }

  const totalPages = pages.length > 0 ? pages.length : doc.pageCount;
  const pageAnalytics = analytics.find((a) => a.pageNumber === page);
  const pageList = Array.from({ length: totalPages }, (_, i) => {
    const num = i + 1;
    const a = analytics.find((x) => x.pageNumber === num);
    const p = pages.find((x) => x.pageNumber === num);
    return { pageNumber: num, viewCount: a?.viewCount ?? 0, avgDurationSeconds: a?.avgDurationSeconds ?? 0, width: p?.width, height: p?.height };
  });

  const currentPageInfo = pages.find((p) => p.pageNumber === page);
  const aspectRatio = currentPageInfo && currentPageInfo.height > 0
    ? currentPageInfo.width / currentPageInfo.height
    : 0.75;

  const baseWidth = Math.max(300, 800);
  const pageWidth = Math.max(300, baseWidth * (zoom / 100));
  const pageHeight = pageWidth / aspectRatio;

  const activeEvidence = (evidence ?? [])
    .filter((e) => e.page_number === page)
    .concat(
      highlightedEvidence && highlightedEvidence.page_number === page ? [highlightedEvidence] : []
    );
  const activeWatermark = watermark === null ? undefined : watermark ?? DEFAULT_WATERMARK;

  return (
    <div className="flex flex-1 flex-col bg-neutral-50 dark:bg-background">
      <header className="flex h-14 items-center justify-between border-b border-border bg-background px-4">
        <div className="flex items-center gap-3">
          <div className="flex h-7 w-7 items-center justify-center rounded-md bg-primary text-xs font-bold text-primary-foreground">
            D
          </div>
          <div>
            <p className="text-sm font-medium">{doc.title}</p>
            <p className="text-caption text-muted-foreground">
              {t("documents:viewer.meta", {
                fileType: doc.fileType.toUpperCase(),
                fileSize: formatFileSize(doc.fileSize),
                pageCount: totalPages,
              })}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-1">
          <Button
            size="icon-sm"
            variant="ghost"
            onClick={() => setZoom((z) => Math.max(50, z - 10))}
            aria-label={t("documents:viewer.zoomOut")}
          >
            <MagnifyingGlassMinus size={16} />
          </Button>
          <span className="min-w-[3rem] text-center text-sm tabular-nums">{zoom}%</span>
          <Button
            size="icon-sm"
            variant="ghost"
            onClick={() => setZoom((z) => Math.min(200, z + 10))}
            aria-label={t("documents:viewer.zoomIn")}
          >
            <MagnifyingGlassPlus size={16} />
          </Button>
        </div>

        <div className="flex items-center gap-1">
          <Button
            size="icon-sm"
            variant="ghost"
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={page <= 1}
            aria-label={t("documents:viewer.previousPage")}
          >
            <CaretLeft size={16} />
          </Button>
          <span className="min-w-[4rem] text-center text-sm tabular-nums">
            {page} / {totalPages}
          </span>
          <Button
            size="icon-sm"
            variant="ghost"
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            disabled={page >= totalPages}
            aria-label={t("documents:viewer.nextPage")}
          >
            <CaretRight size={16} />
          </Button>
          <Button
            size="icon-sm"
            variant="ghost"
            aria-label={t("common:download")}
            onClick={handleDownload}
          >
            <Download size={16} />
          </Button>
        </div>
      </header>

      <div className="flex flex-1 overflow-hidden">
        <ThumbnailNav
          pages={pageList}
          currentPage={page}
          onSelect={setPage}
          className="hidden w-48 md:flex"
        />

        {/* Canvas area */}
        <div className="relative flex flex-1 items-center justify-center overflow-auto p-8">
          <div
            className="relative overflow-hidden rounded-md bg-white shadow-card"
            style={{ width: `${pageWidth}px`, height: `${pageHeight}px` }}
          >
            {doc.status !== "ready" ? (
              <div className="flex h-full w-full flex-col items-center justify-center gap-4 p-8 text-center text-muted-foreground">
                <FileText size={48} className="text-muted-foreground/50" />
                <p className="text-body">{t("documents:viewer.processing", { status: doc.status })}</p>
              </div>
            ) : imageUrl ? (
              <img
                src={imageUrl}
                alt={t("documents:viewer.pageLabel", { pageNumber: page })}
                className="h-full w-full object-contain"
              />
            ) : (
              <div className="flex h-full w-full flex-col items-center justify-center gap-4 p-8 text-center text-muted-foreground">
                <div className="text-h1 text-muted-foreground">
                  {t("documents:viewer.pagePlaceholder", { pageNumber: page })}
                </div>
                <p className="text-body text-muted-foreground">{t("documents:viewer.previewPlaceholder")}</p>
              </div>
            )}

            <HighlightOverlay evidences={activeEvidence} />
            <WatermarkOverlay watermark={activeWatermark} />
          </div>

          {pageAnalytics && (
            <p className="absolute bottom-2 left-1/2 -translate-x-1/2 text-caption text-muted-foreground">
              {t("documents:viewer.currentPageStats", {
                count: pageAnalytics.viewCount,
                duration: formatDuration(pageAnalytics.avgDurationSeconds),
              })}
            </p>
          )}
        </div>
      </div>
    </div>
  );
}
