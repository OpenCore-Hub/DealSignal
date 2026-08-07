import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";

export interface PublicThumbnailPage {
  pageNumber: number;
}

interface PublicViewerThumbnailRailProps {
  pages: PublicThumbnailPage[];
  currentPage: number;
  thumbnailUrls?: Record<number, string>;
  onSelect: (pageNumber: number) => void;
  className?: string;
}

export function PublicViewerThumbnailRail({
  pages,
  currentPage,
  thumbnailUrls = {},
  onSelect,
  className,
}: PublicViewerThumbnailRailProps) {
  const { t } = useTranslation("documents");

  return (
    <aside
      className={cn(
        "flex h-full min-h-0 w-[4.75rem] shrink-0 flex-col gap-2 overflow-y-auto px-2 py-3 sm:w-[5.5rem]",
        className,
      )}
      aria-label={t("viewer.publicPageRail")}
    >
      <p className="px-1 text-[10px] font-semibold uppercase tracking-[0.14em] text-muted-foreground/80">
        {t("viewer.publicPageRail")}
      </p>
      {pages.map((p) => {
        const isActive = currentPage === p.pageNumber;
        const previewUrl = thumbnailUrls[p.pageNumber];
        const pageLabel = t("viewer.pageLabelShort", { pageNumber: p.pageNumber });
        return (
          <button
            key={p.pageNumber}
            type="button"
            onClick={() => onSelect(p.pageNumber)}
            className={cn(
              "group relative flex flex-col items-center gap-1.5 rounded-xl p-1.5 transition-all duration-200",
              isActive
                ? "bg-background/90 shadow-sm ring-1 ring-emerald-500/30"
                : "hover:bg-background/60",
            )}
            aria-current={isActive ? "page" : undefined}
            aria-label={t("viewer.pageLabel", { pageNumber: p.pageNumber })}
          >
            <div
              className={cn(
                "relative aspect-[3/4] w-full overflow-hidden rounded-lg border bg-neutral-100 shadow-sm transition-transform duration-200 group-active:scale-[0.98] dark:bg-neutral-900",
                isActive ? "border-emerald-500/40" : "border-border/70",
              )}
            >
              {previewUrl ? (
                <img
                  src={previewUrl}
                  alt=""
                  aria-hidden
                  loading="eager"
                  decoding="async"
                  className="h-full w-full object-cover object-top"
                />
              ) : (
                <div
                  className="h-full w-full animate-pulse bg-gradient-to-br from-neutral-200/80 to-neutral-100 dark:from-neutral-800 dark:to-neutral-900"
                  aria-hidden
                />
              )}
            </div>
            <span
              className={cn(
                "text-[10px] font-medium tabular-nums",
                isActive ? "text-emerald-700 dark:text-emerald-300" : "text-muted-foreground",
              )}
            >
              {pageLabel}
            </span>
          </button>
        );
      })}
    </aside>
  );
}
