import { useEffect, useState } from "react";
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
import { ThumbnailNav } from "./ThumbnailNav";
import { HighlightOverlay } from "./HighlightOverlay";
import { WatermarkOverlay, type WatermarkInfo } from "./WatermarkOverlay";

const DEFAULT_WATERMARK: WatermarkInfo = {
  email: "viewer@example.test",
};

const DEFAULT_EVIDENCE: Evidence[] = [
  {
    id: "ev-demo-001",
    pageNumber: 1,
    text: "Revenue grew 3x.",
    bbox: { x: 0.15, y: 0.25, w: 0.45, h: 0.06 },
  },
];

interface CanvasViewerProps {
  evidence?: Evidence[];
  watermark?: WatermarkInfo | null;
}

export function CanvasViewer({ evidence, watermark }: CanvasViewerProps = {}) {
  const { documentId } = useParams<{ documentId: string }>();
  const { t } = useTranslation(["documents", "common"]);
  const [doc, setDoc] = useState<Document | null>(null);
  const [analytics, setAnalytics] = useState<PageAnalytics[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [retryTick, setRetryTick] = useState(0);
  const [page, setPage] = useState(1);
  const [zoom, setZoom] = useState(100);

  useEffect(() => {
    let cancelled = false;
    const id = documentId;
    if (!id) return;
    async function load() {
      try {
        setLoading(true);
        setError(null);
        const [d, a] = await Promise.all([api.getDocumentById(id!), api.getPageAnalytics(id!)]);
        if (!cancelled) {
          setDoc(d);
          setAnalytics(a.data);
          setPage(1);
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

  const totalPages = doc.pageCount;
  const pageAnalytics = analytics.find((a) => a.pageNumber === page);
  const pages = Array.from({ length: totalPages }, (_, i) => {
    const num = i + 1;
    const a = analytics.find((x) => x.pageNumber === num);
    return { pageNumber: num, viewCount: a?.viewCount ?? 0, avgDurationSeconds: a?.avgDurationSeconds ?? 0 };
  });

  const pageWidth = Math.max(300, zoom * 6);
  const pageHeight = Math.max(400, zoom * 8);
  const activeEvidence = (evidence ?? DEFAULT_EVIDENCE).filter((e) => e.pageNumber === page);
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
            disabled
            title={t("documents:viewer.downloadDisabled")}
            onClick={() => {}}
          >
            <Download size={16} />
          </Button>
        </div>
      </header>

      <div className="flex flex-1 overflow-hidden">
        <ThumbnailNav
          pages={pages}
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
            <div className="flex h-full w-full flex-col items-center justify-center gap-4 p-8 text-center text-muted-foreground">
              <div className="text-h1 text-muted-foreground">
                {t("documents:viewer.pagePlaceholder", { pageNumber: page })}
              </div>
              <p className="text-body text-muted-foreground">{t("documents:viewer.previewPlaceholder")}</p>
              <p className="text-caption max-w-xs">
                {t("documents:viewer.signedUrlNotice")}
                {pageAnalytics && (
                  <>
                    <br />
                    {t("documents:viewer.currentPageStats", {
                      count: pageAnalytics.viewCount,
                      duration: formatDuration(pageAnalytics.avgDurationSeconds),
                    })}
                  </>
                )}
              </p>
            </div>

            <HighlightOverlay evidences={activeEvidence} />
            <WatermarkOverlay watermark={activeWatermark} />
          </div>
        </div>
      </div>
    </div>
  );
}
