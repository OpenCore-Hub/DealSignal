import { useTranslation } from "react-i18next";
import { Eye } from "@phosphor-icons/react";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { useAsyncData } from "@/hooks/useAsyncData";
import { calculateHeatLevel } from "./DocumentsColumns";
import { cn } from "@/lib/utils";

interface PageCardProps {
  documentId: string;
  pageNumber: number;
  viewCount: number;
  avgDurationSeconds: number;
  exitRate?: number;
  hasEvidence: boolean;
  isSelected: boolean;
  onClick: () => void;
}

export function PageCard({
  documentId,
  pageNumber,
  viewCount,
  avgDurationSeconds,
  exitRate,
  hasEvidence,
  isSelected,
  onClick,
}: PageCardProps) {
  const { t } = useTranslation("documents");
  const { t: tc } = useTranslation("common");
  const heatLevel = calculateHeatLevel(viewCount);

  const { data: signedUrlData, loading } = useAsyncData(async () => {
    const res = await api.getPageSignedUrl(documentId, pageNumber);
    return res;
  }, [documentId, pageNumber]);

  const imageUrl = signedUrlData?.image_url;

  return (
    <Card
      role="button"
      tabIndex={0}
      className={cn(
        "group relative cursor-pointer overflow-hidden border-border bg-card shadow-sm transition-all hover:-translate-y-0.5 hover:shadow-md",
        isSelected && "ring-2 ring-primary"
      )}
      onClick={onClick}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onClick();
        }
      }}
    >
      {/* Thumbnail */}
      <div className="relative aspect-[3/4] bg-muted/40">
        {loading ? (
          <Skeleton className="h-full w-full" />
        ) : imageUrl ? (
          <img
            src={imageUrl}
            alt={t("documents:content.pageLabel", { pageNumber })}
            loading="lazy"
            decoding="async"
            className="h-full w-full object-cover object-top transition-transform duration-300 group-hover:scale-[1.02]"
          />
        ) : (
          <div className="flex h-full w-full flex-col items-center justify-center text-muted-foreground">
            <span className="text-4xl font-semibold text-muted-foreground/30">{pageNumber}</span>
          </div>
        )}

        {/* Evidence badge */}
        {hasEvidence && (
          <Badge className="absolute left-2 top-2 bg-primary text-primary-foreground">
            {t("documents:content.evidenceBadge")}
          </Badge>
        )}

        {/* Heat badge */}
        {viewCount > 0 && (
          <Badge variant={heatLevel} className="absolute right-2 top-2">
            {viewCount}
          </Badge>
        )}

        {/* Hover overlay */}
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 bg-black/60 opacity-0 transition-opacity duration-200 group-hover:opacity-100">
          <Button size="sm" variant="secondary" className="gap-1.5">
            <Eye size={14} />
            {tc("view")}
          </Button>
          <div className="text-center text-xs text-white/90">
            <p>
              {t("documents:content.viewCount")}: {viewCount}
            </p>
            <p>
              {t("documents:content.avgDuration")}: {avgDurationSeconds}s
            </p>
            {exitRate !== undefined && (
              <p>
                {t("documents:content.exitRate")}: {Math.round(exitRate * 100)}%
              </p>
            )}
          </div>
        </div>

        {/* Bottom page number strip */}
        <div className="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/70 to-transparent px-3 pb-2 pt-6">
          <p className="text-sm font-medium text-white">
            {t("documents:content.pageLabel", { pageNumber })}
          </p>
        </div>
      </div>
    </Card>
  );
}
