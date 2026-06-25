import { useTranslation } from "react-i18next";
import { formatDuration } from "@/lib/formatters";
import { cn } from "@/lib/utils";

export interface ThumbnailPage {
  pageNumber: number;
  viewCount: number;
  avgDurationSeconds: number;
}

interface ThumbnailNavProps {
  pages: ThumbnailPage[];
  currentPage: number;
  onSelect: (pageNumber: number) => void;
  className?: string;
}

export function ThumbnailNav({ pages, currentPage, onSelect, className }: ThumbnailNavProps) {
  const { t } = useTranslation("documents");
  const maxViews = Math.max(...pages.map((p) => p.viewCount), 1);

  return (
    <aside
      className={cn(
        "flex h-full min-h-0 flex-col gap-2 overflow-y-auto border-r border-border bg-card p-3",
        className
      )}
      aria-label={t("viewer.pageHeat")}
    >
      <p className="text-caption font-medium text-muted-foreground">{t("viewer.pageHeat")}</p>
      {pages.map((p) => {
        const heat = p.viewCount > 0 ? Math.min(100, (p.viewCount / maxViews) * 100) : 0;
        const isActive = currentPage === p.pageNumber;
        return (
          <button
            key={p.pageNumber}
            type="button"
            onClick={() => onSelect(p.pageNumber)}
            className={cn(
              "flex flex-col gap-1 rounded-md border p-2 text-left transition-colors",
              isActive
                ? "border-primary bg-primary/5"
                : "border-border bg-background hover:bg-muted"
            )}
            aria-current={isActive ? "page" : undefined}
          >
            <span className="text-xs font-medium">
              {t("viewer.pageLabel", { pageNumber: p.pageNumber })}
            </span>
            <div className="h-1 w-full overflow-hidden rounded-full bg-muted">
              <div className="h-full rounded-full bg-hot-500" style={{ width: `${heat}%` }} />
            </div>
            <span className="text-caption text-muted-foreground">
              {t("viewer.thumbnailViews", {
                count: p.viewCount,
                duration: formatDuration(p.avgDurationSeconds),
              })}
            </span>
          </button>
        );
      })}
    </aside>
  );
}
