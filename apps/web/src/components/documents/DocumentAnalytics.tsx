import { useMemo, useState } from "react";
import { ChartBar } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/common/EmptyState";
import {
  barDuration,
  defaultPageAnalyticsFilter,
  projectPageAnalytics,
  type PageAnalyticsBar,
  type PageAnalyticsFilter,
  type PageAnalyticsFocusRange,
  type PageAnalyticsVariant,
} from "@/lib/projectPageAnalytics";
import type { PageAnalytics } from "@/types";

interface DocumentAnalyticsProps {
  analytics: PageAnalytics[];
  /** overview = glanceable summary; detail = exploratory (数据 tab / insights). */
  variant?: PageAnalyticsVariant;
  /** Open a single page in the document content surface (detail tab / deep link). */
  onOpenPage?: (pageNumber: number) => void;
  className?: string;
}

function barLabel(bar: PageAnalyticsBar): string {
  if (bar.kind === "page") return String(bar.pageNumber);
  return bar.startPage === bar.endPage
    ? String(bar.startPage)
    : `${bar.startPage}–${bar.endPage}`;
}

function TopPagesList({
  pages,
  onOpenPage,
}: {
  pages: { pageNumber: number; avgDurationSeconds: number }[];
  onOpenPage?: (pageNumber: number) => void;
}) {
  const { t } = useTranslation("documents");
  if (pages.length === 0) return null;
  return (
    <div className="mt-5 border-t border-border/60 pt-4">
      <p className="text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground/80">
        {t("documents:analytics.topPages")}
      </p>
      <ol className="mt-3 divide-y divide-border/50">
        {pages.map((page, index) => (
          <li
            key={page.pageNumber}
            className="flex items-center justify-between gap-3 py-2 first:pt-0 last:pb-0"
          >
            <div className="flex min-w-0 items-baseline gap-2">
              <span className="font-mono text-[11px] tabular-nums text-muted-foreground">
                {index + 1}
              </span>
              {onOpenPage ? (
                <button
                  type="button"
                  className="truncate text-left text-[13px] font-medium text-foreground underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  data-testid={`document-analytics-top-page-${page.pageNumber}`}
                  onClick={() => onOpenPage(page.pageNumber)}
                >
                  {t("documents:analytics.topPageLabel", { pageNumber: page.pageNumber })}
                </button>
              ) : (
                <span className="text-[13px] font-medium text-foreground">
                  {t("documents:analytics.topPageLabel", { pageNumber: page.pageNumber })}
                </span>
              )}
            </div>
            <span className="font-mono text-[12px] tabular-nums text-muted-foreground">
              {t("documents:analytics.topPageSeconds", { seconds: page.avgDurationSeconds })}
            </span>
          </li>
        ))}
      </ol>
    </div>
  );
}

export function DocumentAnalytics({
  analytics,
  variant = "detail",
  onOpenPage,
  className,
}: DocumentAnalyticsProps) {
  const { t } = useTranslation("documents");
  const [filter, setFilter] = useState<PageAnalyticsFilter>(() =>
    defaultPageAnalyticsFilter(analytics, variant),
  );
  const [expandPerPage, setExpandPerPage] = useState(false);
  const [focusRange, setFocusRange] = useState<PageAnalyticsFocusRange | null>(null);

  const projection = useMemo(
    () =>
      projectPageAnalytics(analytics, {
        variant,
        filter,
        expandPerPage,
        focusRange,
      }),
    [analytics, expandPerPage, filter, focusRange, variant],
  );

  if (analytics.length === 0) {
    return (
      <section
        className={cn("rounded-2xl border border-border/70 bg-background px-5 py-5", className)}
        data-testid="document-analytics"
        data-variant={variant}
      >
        <h2 className="text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground/80">
          {t("documents:analytics.title")}
        </h2>
        <div className="mt-4">
          <EmptyState
            icon={<ChartBar size={40} />}
            title={t("documents:analytics.emptyTitle")}
            description={t("documents:analytics.emptyDescription")}
          />
        </div>
      </section>
    );
  }

  const maxDuration = Math.max(...projection.bars.map(barDuration), 1);
  const scrollable = projection.strategy === "scroll";
  const showFilter = variant === "detail" && projection.zeroCount > 0 && !projection.focusRange;
  const showExpand = variant === "detail" && projection.canExpandPerPage;
  const showTopPages =
    projection.topPages.length > 0 &&
    (variant === "overview" ||
      projection.strategy === "bucketed" ||
      projection.totalPages > 24 ||
      Boolean(projection.focusRange));

  const drillIntoBucket = (bar: Extract<PageAnalyticsBar, { kind: "bucket" }>) => {
    // Single-page buckets open Content directly (no intermediate drill).
    if (bar.startPage === bar.endPage && onOpenPage) {
      onOpenPage(bar.startPage);
      return;
    }
    setExpandPerPage(false);
    setFocusRange({ startPage: bar.startPage, endPage: bar.endPage });
  };

  const clearFocus = () => {
    setExpandPerPage(false);
    setFocusRange(null);
  };

  return (
    <section
      className={cn("rounded-2xl border border-border/70 bg-background px-5 py-5", className)}
      data-testid="document-analytics"
      data-variant={variant}
      data-strategy={projection.strategy}
      data-focus-start={projection.focusRange?.startPage ?? undefined}
      data-focus-end={projection.focusRange?.endPage ?? undefined}
    >
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div className="space-y-1">
          <h2 className="text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground/80">
            {t("documents:analytics.title")}
          </h2>
          <p className="font-mono text-[11px] tabular-nums text-muted-foreground">
            {projection.focusRange ? (
              t("documents:analytics.focusRange", {
                start: projection.focusRange.startPage,
                end: projection.focusRange.endPage,
              })
            ) : (
              <>
                {t("documents:analytics.firstPage")}
                <span className="mx-1.5 text-border">·</span>
                {t("documents:analytics.lastPage", { count: projection.totalPages })}
              </>
            )}
            {projection.strategy === "bucketed" ? (
              <>
                <span className="mx-1.5 text-border">·</span>
                {t("documents:analytics.bucketedHint", { count: projection.bars.length })}
              </>
            ) : null}
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          {projection.focusRange ? (
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="h-7 px-2.5 text-xs"
              onClick={clearFocus}
              data-testid="document-analytics-clear-focus"
            >
              {t("documents:analytics.clearFocus")}
            </Button>
          ) : null}
          {showFilter ? (
            <div className="flex items-center gap-1 rounded-lg bg-muted/60 p-0.5">
              <Button
                type="button"
                size="sm"
                variant={filter === "all" ? "secondary" : "ghost"}
                className="h-7 px-2.5 text-xs"
                onClick={() => setFilter("all")}
              >
                {t("documents:analytics.filterAll")}
              </Button>
              <Button
                type="button"
                size="sm"
                variant={filter === "engaged" ? "secondary" : "ghost"}
                className="h-7 px-2.5 text-xs"
                onClick={() => setFilter("engaged")}
              >
                {t("documents:analytics.filterEngaged")}
              </Button>
            </div>
          ) : null}
          {showExpand ? (
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="h-7 px-2.5 text-xs"
              onClick={() => setExpandPerPage((v) => !v)}
              data-testid="document-analytics-expand"
            >
              {expandPerPage
                ? t("documents:analytics.collapsePerPage")
                : t("documents:analytics.expandPerPage", { count: projection.sourceCount })}
            </Button>
          ) : null}
        </div>
      </div>

      {projection.hiddenZeroCount > 0 ? (
        <p className="mt-3 text-[12px] text-muted-foreground">
          {t("documents:analytics.hiddenZeros", { count: projection.hiddenZeroCount })}
        </p>
      ) : null}

      {projection.strategy === "bucketed" && !projection.focusRange ? (
        <p className="mt-2 text-[12px] text-muted-foreground">
          {t("documents:analytics.drillHint")}
        </p>
      ) : onOpenPage && projection.strategy !== "bucketed" ? (
        <p className="mt-2 text-[12px] text-muted-foreground">
          {t("documents:analytics.openPageHint")}
        </p>
      ) : null}

      {projection.bars.length === 0 ? (
        <div className="mt-4">
          <EmptyState
            icon={<ChartBar size={36} />}
            title={t("documents:analytics.filterEmptyTitle")}
            description={t("documents:analytics.filterEmptyDescription")}
          />
        </div>
      ) : (
        <div
          className={cn(
            "mt-5",
            scrollable && "scrollbar-auto overflow-x-auto overscroll-x-contain pb-1",
          )}
        >
          <div
            className={cn(
              "flex h-44 items-end gap-1.5",
              scrollable ? "min-w-max gap-1" : "w-full sm:gap-2",
            )}
            role="img"
            aria-label={t("documents:analytics.title")}
          >
            {projection.bars.map((bar) => {
              const duration = barDuration(bar);
              const height = Math.max(
                bar.kind === "page" && bar.zero ? 4 : 8,
                (duration / maxDuration) * 100,
              );
              const tooltip =
                bar.kind === "page"
                  ? t("documents:analytics.pageTooltip", {
                      pageNumber: bar.pageNumber,
                      seconds: bar.avgDurationSeconds,
                    })
                  : t("documents:analytics.bucketTooltip", {
                      start: bar.startPage,
                      end: bar.endPage,
                      seconds: bar.maxDurationSeconds,
                    });
              const tickVisible =
                bar.kind === "bucket" ||
                !scrollable ||
                (bar.kind === "page" &&
                  (bar.pageNumber === 1 ||
                    bar.pageNumber === projection.totalPages ||
                    (projection.focusRange &&
                      (bar.pageNumber === projection.focusRange.startPage ||
                        bar.pageNumber === projection.focusRange.endPage)) ||
                    bar.pageNumber % 5 === 0));
              const drillable = bar.kind === "bucket";
              const openable = bar.kind === "page" && Boolean(onOpenPage);

              return (
                <div
                  key={bar.key}
                  className={cn(
                    "group relative flex h-full flex-col justify-end",
                    scrollable ? "w-3 shrink-0 sm:w-3.5" : "min-w-0 flex-1",
                    (drillable || openable) && "cursor-pointer",
                  )}
                  title={tooltip}
                >
                  {drillable ? (
                    <button
                      type="button"
                      className="flex h-full w-full flex-col justify-end rounded-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                      aria-label={t("documents:analytics.drillBucket", {
                        start: bar.startPage,
                        end: bar.endPage,
                      })}
                      data-testid={`document-analytics-bucket-${bar.startPage}-${bar.endPage}`}
                      onClick={() => drillIntoBucket(bar)}
                    >
                      <div
                        className={cn(
                          "w-full rounded-sm origin-bottom transition-[background-color,transform] duration-200",
                          "motion-reduce:transition-none motion-reduce:group-hover:scale-y-100",
                          "bg-foreground/[0.14] group-hover:bg-foreground/30 group-hover:scale-y-[1.02]",
                        )}
                        style={{ height: `${height}%` }}
                      />
                      <span
                        className={cn(
                          "mt-2 text-center font-mono text-[10px] tabular-nums text-muted-foreground/80",
                          !tickVisible && "invisible",
                        )}
                      >
                        {barLabel(bar)}
                      </span>
                    </button>
                  ) : openable && bar.kind === "page" ? (
                    <button
                      type="button"
                      className="flex h-full w-full flex-col justify-end rounded-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                      aria-label={t("documents:analytics.openPage", {
                        pageNumber: bar.pageNumber,
                      })}
                      data-testid={`document-analytics-page-${bar.pageNumber}`}
                      onClick={() => onOpenPage?.(bar.pageNumber)}
                    >
                      <div
                        className={cn(
                          "w-full rounded-sm origin-bottom transition-[background-color,transform] duration-200",
                          "motion-reduce:transition-none motion-reduce:group-hover:scale-y-100",
                          bar.zero
                            ? "bg-foreground/[0.06]"
                            : "bg-foreground/[0.14] group-hover:bg-foreground/30 group-hover:scale-y-[1.02]",
                        )}
                        style={{ height: `${height}%` }}
                      />
                      <span
                        className={cn(
                          "mt-2 text-center font-mono text-[10px] tabular-nums text-muted-foreground/80",
                          !tickVisible && "invisible",
                        )}
                      >
                        {barLabel(bar)}
                      </span>
                    </button>
                  ) : (
                    <>
                      <div
                        className={cn(
                          "w-full rounded-sm origin-bottom transition-[background-color,transform] duration-200",
                          "motion-reduce:transition-none motion-reduce:group-hover:scale-y-100",
                          bar.kind === "page" && bar.zero
                            ? "bg-foreground/[0.06]"
                            : "bg-foreground/[0.14] group-hover:bg-foreground/30 group-hover:scale-y-[1.02]",
                        )}
                        style={{ height: `${height}%` }}
                        aria-label={tooltip}
                      />
                      <span
                        className={cn(
                          "mt-2 text-center font-mono text-[10px] tabular-nums text-muted-foreground/80",
                          !tickVisible && "invisible",
                        )}
                      >
                        {barLabel(bar)}
                      </span>
                    </>
                  )}
                  <div className="pointer-events-none absolute -top-9 left-1/2 z-10 hidden -translate-x-1/2 whitespace-nowrap rounded-md bg-foreground px-2 py-1 text-[11px] text-background group-hover:block">
                    {tooltip}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {showTopPages ? (
        <TopPagesList pages={projection.topPages} onOpenPage={onOpenPage} />
      ) : null}
    </section>
  );
}
