import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { FileText } from "@phosphor-icons/react";
import { ThumbnailNav } from "./ThumbnailNav";
import { HighlightOverlay } from "./HighlightOverlay";
import { WatermarkOverlay, type WatermarkInfo } from "./WatermarkOverlay";
import { useAIStore } from "@/stores/aiStore";
import { formatDuration } from "@/lib/formatters";
import type { Document, Evidence, PageAnalytics } from "@/types";

interface PageInfo {
  pageNumber: number;
  width: number;
  height: number;
}

const DEFAULT_WATERMARK: WatermarkInfo = {
  email: "viewer@example.test",
};

interface ViewerCanvasProps {
  doc: Document;
  page: number;
  zoom: number;
  pages: PageInfo[];
  analytics: PageAnalytics[];
  imageUrl: string | null;
  evidence?: Evidence[];
  watermark?: WatermarkInfo | null;
  onSelectPage: (page: number) => void;
}

export function ViewerCanvas({
  doc,
  page,
  zoom,
  pages,
  analytics,
  imageUrl,
  evidence,
  watermark,
  onSelectPage,
}: ViewerCanvasProps) {
  const { t } = useTranslation("documents");
  const { highlightedEvidence } = useAIStore();
  const viewportRef = useRef<HTMLDivElement>(null);
  const [viewportSize, setViewportSize] = useState({ width: 800, height: 600 });

  useEffect(() => {
    const el = viewportRef.current;
    if (!el || typeof ResizeObserver === "undefined") return;

    const updateSize = () => {
      // clientWidth/Height includes padding, so subtract it so the page fits
      // inside the content area rather than overflowing the padded viewport.
      const style = window.getComputedStyle(el);
      const padX = parseFloat(style.paddingLeft) + parseFloat(style.paddingRight);
      const padY = parseFloat(style.paddingTop) + parseFloat(style.paddingBottom);
      setViewportSize({
        width: Math.max(0, el.clientWidth - padX),
        height: Math.max(0, el.clientHeight - padY),
      });
    };

    updateSize();
    const observer = new ResizeObserver(updateSize);
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  const totalPages = pages.length > 0 ? pages.length : doc.pageCount;
  const pageList = Array.from({ length: totalPages }, (_, i) => {
    const num = i + 1;
    const a = analytics.find((x) => x.pageNumber === num);
    const p = pages.find((x) => x.pageNumber === num);
    return {
      pageNumber: num,
      viewCount: a?.viewCount ?? 0,
      avgDurationSeconds: a?.avgDurationSeconds ?? 0,
      width: p?.width,
      height: p?.height,
    };
  });

  const currentPageInfo = pages.find((p) => p.pageNumber === page);
  const aspectRatio = currentPageInfo && currentPageInfo.height > 0
    ? currentPageInfo.width / currentPageInfo.height
    : 0.75;

  // Fit the page inside the available viewport at zoom = 100%.
  const availableWidth = Math.max(300, viewportSize.width);
  const availableHeight = Math.max(300, viewportSize.height);
  const fitWidth = Math.min(availableWidth, availableHeight * aspectRatio);
  const fitHeight = fitWidth / aspectRatio;
  const baseWidth = Math.max(300, Math.min(fitWidth, fitHeight * aspectRatio));

  const pageWidth = Math.max(300, baseWidth * (zoom / 100));
  const pageHeight = pageWidth / aspectRatio;

  const activeEvidence = (evidence ?? [])
    .filter((e) => e.page_number === page)
    .concat(
      highlightedEvidence && highlightedEvidence.page_number === page ? [highlightedEvidence] : []
    );
  const activeWatermark = watermark === null ? undefined : watermark ?? DEFAULT_WATERMARK;
  const pageAnalytics = analytics.find((a) => a.pageNumber === page);

  return (
    <div className="flex min-h-0 flex-1 overflow-hidden">
      <ThumbnailNav
        pages={pageList}
        currentPage={page}
        onSelect={onSelectPage}
        className="hidden h-full w-48 shrink-0 md:flex"
      />

      <div
        ref={viewportRef}
        className="relative flex min-h-0 flex-1 items-start justify-center overflow-auto p-2 sm:p-4 lg:p-8"
      >
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
  );
}
