import { useLayoutEffect, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useTranslation } from "react-i18next";
import { PageCard } from "./PageCard";
import {
  DEFAULT_PAGE_ASPECT_RATIO,
  PAGE_GRID_GAP_PX,
  PAGE_GRID_OVERSCAN_ROWS,
  columnCountForWidth,
  estimatePageCardRowHeight,
  pageGridRowCount,
  shouldVirtualizePageGrid,
  type PageGridItem,
} from "@/lib/projectPageGrid";
import { cn } from "@/lib/utils";

interface DocumentPageGridProps {
  items: PageGridItem[];
  documentId: string;
  selectedPage: number | null;
  focusPage?: number | null;
  aspectRatio?: number;
  onSelectPage: (pageNumber: number) => void;
}

const GRID_CLASS =
  "grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6";

export function DocumentPageGrid({
  items,
  documentId,
  selectedPage,
  focusPage = null,
  aspectRatio = DEFAULT_PAGE_ASPECT_RATIO,
  onSelectPage,
}: DocumentPageGridProps) {
  const { t } = useTranslation("documents");
  const scrollRef = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(0);

  useLayoutEffect(() => {
    const el = scrollRef.current;
    if (!el || typeof ResizeObserver === "undefined") {
      setWidth(el?.clientWidth ?? 720);
      return;
    }
    const ro = new ResizeObserver((entries) => {
      const next = entries[0]?.contentRect.width ?? el.clientWidth;
      setWidth(next);
    });
    ro.observe(el);
    setWidth(el.clientWidth);
    return () => ro.disconnect();
  }, []);

  const virtualize = shouldVirtualizePageGrid(items.length);
  const columns = columnCountForWidth(width || 720);
  const rowCount = pageGridRowCount(items.length, columns);
  const estimateSize = estimatePageCardRowHeight(width || 720, columns, aspectRatio);

  const virtualizer = useVirtualizer({
    count: virtualize ? rowCount : 0,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => estimateSize,
    overscan: PAGE_GRID_OVERSCAN_ROWS,
    getItemKey: (index) => `row-${columns}-${index}`,
  });

  useLayoutEffect(() => {
    if (!focusPage || focusPage < 1) return;
    if (virtualize) {
      const rowIndex = Math.floor((focusPage - 1) / Math.max(columns, 1));
      virtualizer.scrollToIndex(rowIndex, { align: "center" });
      return;
    }
    const el = scrollRef.current?.querySelector<HTMLElement>(`[data-page="${focusPage}"]`);
    el?.scrollIntoView({ block: "nearest", inline: "nearest", behavior: "smooth" });
  }, [columns, focusPage, virtualize, virtualizer]);

  if (items.length === 0) return null;

  if (!virtualize) {
    return (
      <div
        ref={scrollRef}
        className={GRID_CLASS}
        data-testid="document-page-grid"
        data-virtualized="false"
      >
        {items.map((page) => (
          <div key={page.pageNumber} data-page={page.pageNumber}>
            <PageCard
              documentId={documentId}
              pageNumber={page.pageNumber}
              viewCount={page.viewCount}
              avgDurationSeconds={page.avgDurationSeconds}
              exitRate={page.exitRate}
              aspectRatio={aspectRatio}
              isSelected={selectedPage === page.pageNumber}
              onClick={() => onSelectPage(page.pageNumber)}
            />
          </div>
        ))}
      </div>
    );
  }

  const virtualRows = virtualizer.getVirtualItems();

  return (
    <div className="space-y-3">
      <p
        className="font-mono text-[11px] tabular-nums text-muted-foreground"
        data-testid="document-page-grid-hint"
      >
        {t("documents:content.virtualizedHint", { count: items.length })}
      </p>
      <div
        ref={scrollRef}
        className="scrollbar-auto max-h-[min(72vh,880px)] overflow-auto overscroll-contain pr-1"
        data-testid="document-page-grid"
        data-virtualized="true"
        data-columns={columns}
      >
        <div
          className="relative w-full"
          style={{ height: virtualizer.getTotalSize() }}
        >
          {virtualRows.map((virtualRow) => {
            const start = virtualRow.index * columns;
            const rowItems = items.slice(start, start + columns);
            return (
              <div
                key={virtualRow.key}
                data-index={virtualRow.index}
                ref={virtualizer.measureElement}
                className={cn("absolute left-0 top-0 w-full")}
                style={{
                  transform: `translateY(${virtualRow.start}px)`,
                  paddingBottom: PAGE_GRID_GAP_PX,
                }}
              >
                <div
                  className="grid gap-4"
                  style={{
                    gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))`,
                  }}
                >
                  {rowItems.map((page) => (
                    <div key={page.pageNumber} data-page={page.pageNumber}>
                      <PageCard
                        documentId={documentId}
                        pageNumber={page.pageNumber}
                        viewCount={page.viewCount}
                        avgDurationSeconds={page.avgDurationSeconds}
                        exitRate={page.exitRate}
                        aspectRatio={aspectRatio}
                        isSelected={selectedPage === page.pageNumber}
                        onClick={() => onSelectPage(page.pageNumber)}
                      />
                    </div>
                  ))}
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
