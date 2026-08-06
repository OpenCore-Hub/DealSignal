import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";

export interface PublicThumbnailPage {
  pageNumber: number;
}

interface PublicViewerThumbnailRailProps {
  pages: PublicThumbnailPage[];
  currentPage: number;
  onSelect: (pageNumber: number) => void;
  className?: string;
}

export function PublicViewerThumbnailRail({
  pages,
  currentPage,
  onSelect,
  className,
}: PublicViewerThumbnailRailProps) {
  const { t } = useTranslation("documents");

  return (
    <aside
      className={cn(
        "flex h-full min-h-0 w-[4.75rem] shrink-0 flex-col gap-2 overflow-y-auto px-2 py-3 sm:w-[5.5rem]",
        className
      )}
      aria-label={t("viewer.publicPageRail")}
    >
      <p className="px-1 text-[10px] font-semibold uppercase tracking-[0.14em] text-muted-foreground/80">
        {t("viewer.publicPageRail")}
      </p>
      {pages.map((p) => {
        const isActive = currentPage === p.pageNumber;
        return (
          <button
            key={p.pageNumber}
            type="button"
            onClick={() => onSelect(p.pageNumber)}
            className={cn(
              "group relative flex flex-col items-center gap-1.5 rounded-xl p-1.5 transition-all duration-200",
              isActive
                ? "bg-background/90 shadow-sm ring-1 ring-emerald-500/30"
                : "hover:bg-background/60"
            )}
            aria-current={isActive ? "page" : undefined}
          >
            <div
              className={cn(
                "relative aspect-[3/4] w-full overflow-hidden rounded-lg border bg-gradient-to-br from-white to-neutral-100 shadow-sm transition-transform duration-200 group-active:scale-[0.98]",
                isActive ? "border-emerald-500/40" : "border-border/70"
              )}
            >
              <div className="absolute inset-0 bg-[linear-gradient(180deg,rgba(15,23,42,0.04),transparent_40%)]" />
              <span className="absolute inset-x-0 bottom-1 text-center text-[9px] font-semibold tabular-nums text-muted-foreground">
                {p.pageNumber}
              </span>
            </div>
            <span
              className={cn(
                "text-[10px] font-medium tabular-nums",
                isActive ? "text-emerald-700 dark:text-emerald-300" : "text-muted-foreground"
              )}
            >
              {t("viewer.pageLabelShort", { pageNumber: p.pageNumber })}
            </span>
          </button>
        );
      })}
    </aside>
  );
}
