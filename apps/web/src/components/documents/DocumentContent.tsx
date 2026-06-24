import { useEffect, useState } from "react";
import { FileText, MagnifyingGlassPlus, MagnifyingGlass } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import type { PageAnalytics, Evidence } from "@/types";

interface DocumentContentProps {
  title: string;
  pageCount: number;
  documentId: string;
  analytics: PageAnalytics[];
  evidences?: Evidence[];
}

export function DocumentContent({ title, pageCount, documentId, analytics, evidences: initialEvidences }: DocumentContentProps) {
  const { t } = useTranslation("documents");
  const [selectedPage, setSelectedPage] = useState<number | null>(null);
  const [pageImageUrl, setPageImageUrl] = useState<string | null>(null);
  const [loadingImageUrl, setLoadingImageUrl] = useState(false);
  const [highlightBoxes, setHighlightBoxes] = useState<Array<{ x: number; y: number; w: number; h: number }>>([]);
  const [evidences, setEvidences] = useState<Evidence[]>(initialEvidences ?? []);
  const [searchQuery, setSearchQuery] = useState("");
  const [searching, setSearching] = useState(false);
  const [imageSizeForPage, setImageSizeForPage] = useState<{
    page: number;
    size: { width: number; height: number };
  } | null>(null);

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

  // Search for evidence within this document
  const handleSearch = async () => {
    const q = searchQuery.trim();
    if (!q) return;
    setSearching(true);
    try {
      const res = await api.searchDocument({ query: q, document_id: documentId });
      setEvidences(res.results || []);
    } catch {
      setEvidences([]);
    } finally {
      setSearching(false);
    }
  };

  // Load signed URL for selected page (only depends on page + document, NOT evidences)
  useEffect(() => {
    if (!selectedPage || !documentId) return;
    let cancelled = false;
    // eslint-disable-next-line react-hooks/set-state-in-effect -- loading state for async fetch
    setLoadingImageUrl(true);
    api
      .getPageSignedUrl(documentId, selectedPage)
      .then((res) => {
        if (!cancelled) setPageImageUrl(res.image_url);
      })
      .catch(() => {
        if (!cancelled) setPageImageUrl(null);
      })
      .finally(() => {
        if (!cancelled) setLoadingImageUrl(false);
      });
    return () => {
      cancelled = true;
    };
  }, [selectedPage, documentId]);

  // Update highlight boxes when page or evidences change (separate from image loading)
  useEffect(() => {
    if (!selectedPage) return;
    const pageEvidence = evidences?.filter((e) => e.page_number === selectedPage) ?? [];
    // eslint-disable-next-line react-hooks/set-state-in-effect -- derived UI state from props
    setHighlightBoxes(pageEvidence.flatMap((e) => e.boxes || []));
  }, [selectedPage, evidences]);

  // Jump to a page with evidence and highlight its boxes
  const jumpToEvidence = (pageNumber: number, boxes?: Array<{ x: number; y: number; w: number; h: number }>) => {
    setSelectedPage(pageNumber);
    // highlightBoxes will be set by the useEffect above when evidences updates,
    // but we also set immediately for responsiveness
    if (boxes && boxes.length > 0) {
      setHighlightBoxes(boxes);
    }
  };

  return (
    <div className="space-y-4">
      {/* Search bar for evidence retrieval with bbox highlight */}
      <div className="flex gap-2">
        <Input
          placeholder={t("documents:content.searchPlaceholder")}
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          onKeyDown={(e) => { if (e.key === "Enter") handleSearch(); }}
          className="flex-1"
        />
        <Button onClick={handleSearch} disabled={searching} size="icon">
          <MagnifyingGlass size={16} />
        </Button>
      </div>

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

            {/* Real page image preview with bbox highlight overlay */}
            <div className="mt-4 flex justify-center rounded-md border border-border bg-muted/30 p-4">
              {loadingImageUrl ? (
                <Skeleton className="h-[400px] w-[300px]" />
              ) : pageImageUrl ? (
                <div className="relative inline-block">
                  <img
                    src={pageImageUrl}
                    alt={t("documents:content.pageLabel", { pageNumber: selectedPage })}
                    className="max-h-[600px] w-auto rounded-md shadow-sm"
                    onLoad={(e) => {
                      const img = e.currentTarget;
                      setImageSizeForPage({
                        page: selectedPage,
                        size: { width: img.clientWidth, height: img.clientHeight },
                      });
                    }}
                  />
                  {/* Bounding box highlights - only render after image is loaded and dimensions are known */}
                  {imageSizeForPage?.page === selectedPage &&
                    highlightBoxes.map((box, idx) => {
                      const { width: w, height: h } = imageSizeForPage.size;
                      return (
                        <div
                          key={idx}
                          className="pointer-events-none absolute border-2 border-primary bg-primary/20 animate-pulse rounded-sm"
                          style={{
                            left: `${box.x * w}px`,
                            top: `${box.y * h}px`,
                            width: `${box.w * w}px`,
                            height: `${box.h * h}px`,
                          }}
                        />
                      );
                    })}
                </div>
              ) : (
                <div className="flex h-[400px] w-[300px] flex-col items-center justify-center text-muted-foreground">
                  <FileText size={48} className="text-muted-foreground/50" />
                  <p className="mt-2 text-sm">{t("documents:content.pageLabel", { pageNumber: selectedPage })}</p>
                </div>
              )}
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
                      {ev.match_type && (
                        <p className="mt-1 text-caption text-muted-foreground">
                          {t("documents:content.matchType", { type: ev.match_type, score: (ev.score ?? 0).toFixed(4) })}
                        </p>
                      )}
                    </div>
                  ))}
              </div>
            )}

            {/* Evidence list with jump-to-page capability */}
            {evidences && evidences.length > 0 && (
              <div className="mt-4 rounded-md border border-border bg-muted/30 p-3">
                <p className="text-caption mb-2 text-muted-foreground">{t("documents:content.allEvidence")}</p>
                <div className="space-y-2">
                  {evidences.map((ev, idx) => (
                    <button
                      key={ev.chunk_id || idx}
                      onClick={() => jumpToEvidence(ev.page_number, ev.boxes)}
                      className="block w-full rounded border border-border p-2 text-left text-sm transition-colors hover:bg-muted"
                    >
                      <div className="flex items-center gap-2">
                        <Badge variant="secondary" className="text-caption">
                          {t("documents:content.pageLabel", { pageNumber: ev.page_number })}
                        </Badge>
                        {ev.match_type && (
                          <span className="text-caption text-muted-foreground">{ev.match_type}</span>
                        )}
                      </div>
                      <p className="mt-1 line-clamp-2 text-muted-foreground">{ev.quote}</p>
                    </button>
                  ))}
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
