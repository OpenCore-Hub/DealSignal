import { useState } from "react";
import { FileText, MagnifyingGlassPlus } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import type { PageAnalytics, Evidence } from "@/types";

interface DocumentContentProps {
  title: string;
  pageCount: number;
  analytics: PageAnalytics[];
  evidences?: Evidence[];
}

export function DocumentContent({ title, pageCount, analytics, evidences }: DocumentContentProps) {
  const { t } = useTranslation("documents");
  const [selectedPage, setSelectedPage] = useState<number | null>(null);

  const pages = Array.from({ length: pageCount }, (_, i) => {
    const pageNumber = i + 1;
    const analytic = analytics.find((a) => a.pageNumber === pageNumber);
    return {
      pageNumber,
      viewCount: analytic?.viewCount ?? 0,
      avgDurationSeconds: analytic?.avgDurationSeconds ?? 0,
      hasEvidence: evidences?.some((e) => e.page_number === pageNumber) ?? false,
    };
  });

  const maxViews = Math.max(...pages.map((p) => p.viewCount), 1);

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
        {pages.map((page) => {
          const heatRatio = page.viewCount / maxViews;
          return (
            <Card
              key={page.pageNumber}
              role="button"
              tabIndex={0}
              className={`cursor-pointer overflow-hidden transition-colors hover:bg-muted/50 hover:border-muted-foreground/20 ${
                selectedPage === page.pageNumber ? "ring-2 ring-primary" : ""
              }`}
              onClick={() => setSelectedPage(page.pageNumber)}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  setSelectedPage(page.pageNumber);
                }
              }}
            >
              <CardContent className="relative flex aspect-[3/4] flex-col items-center justify-center">
                <FileText size={32} className="text-muted-foreground/50" />
                <p className="mt-2 text-sm font-medium">{t("documents:content.pageLabel", { pageNumber: page.pageNumber })}</p>
                <div className="absolute bottom-0 left-0 right-0 h-1 bg-muted">
                  <div
                    className="h-full bg-hot-500"
                    style={{ width: `${heatRatio * 100}%` }}
                  />
                </div>
                {page.hasEvidence && (
                  <>
                    <Badge className="absolute right-2 top-2 bg-primary text-primary-foreground text-caption">
                      {t("documents:content.evidenceBadge")}
                    </Badge>
                    <span className="sr-only">{t("documents:content.evidenceSrOnly")}</span>
                  </>
                )}
              </CardContent>
            </Card>
          );
        })}
      </div>

      {selectedPage && (
        <Card>
          <CardContent>
            <div className="flex items-start justify-between">
              <div>
                <p className="text-h3 flex items-center gap-2">
                  <MagnifyingGlassPlus size={18} />
                  {t("documents:content.pageDetailTitle", { pageNumber: selectedPage })}
                </p>
                <p className="text-body mt-1 text-muted-foreground">{title}</p>
              </div>
            </div>
            <div className="mt-4 grid grid-cols-3 gap-4 text-sm">
              {(() => {
                const analytic = analytics.find((a) => a.pageNumber === selectedPage);
                return (
                  <>
                    <div>
                      <p className="text-caption text-muted-foreground">{t("documents:content.viewCount")}</p>
                      <p className="text-h2">{analytic?.viewCount ?? 0}</p>
                    </div>
                    <div>
                      <p className="text-caption text-muted-foreground">{t("documents:content.avgDuration")}</p>
                      <p className="text-h2">{analytic?.avgDurationSeconds ?? 0}s</p>
                    </div>
                    <div>
                      <p className="text-caption text-muted-foreground">{t("documents:content.exitRate")}</p>
                      <p className="text-h2">{Math.round((analytic?.exitRate ?? 0) * 100)}%</p>
                    </div>
                  </>
                );
              })()}
            </div>
            {evidences && evidences.filter((e) => e.page_number === selectedPage).length > 0 && (
              <div className="mt-4 rounded-md border border-border bg-muted/50 p-3">
                <p className="text-caption mb-2 text-muted-foreground">{t("documents:content.evidenceHighlight")}</p>
                {evidences
                  .filter((e) => e.page_number === selectedPage)
                  .map((ev) => (
                    <div key={ev.chunk_id} className="text-sm">
                      <span className="inline-block rounded bg-warning-500/20 px-1 py-0.5 text-xs font-medium text-warning-700">
                        {t("documents:content.sourceLocation")}
                      </span>
                      <p className="mt-1">{ev.quote}</p>
                    </div>
                  ))}
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
