import { useState } from "react";
import { FileText, MagnifyingGlassPlus } from "@phosphor-icons/react";
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
  const [selectedPage, setSelectedPage] = useState<number | null>(null);

  const pages = Array.from({ length: pageCount }, (_, i) => {
    const pageNumber = i + 1;
    const analytic = analytics.find((a) => a.pageNumber === pageNumber);
    return {
      pageNumber,
      viewCount: analytic?.viewCount ?? 0,
      avgDurationSeconds: analytic?.avgDurationSeconds ?? 0,
      hasEvidence: evidences?.some((e) => e.pageNumber === pageNumber) ?? false,
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
              className={`cursor-pointer overflow-hidden transition-all hover:shadow-md ${
                selectedPage === page.pageNumber ? "ring-2 ring-primary" : ""
              }`}
              onClick={() => setSelectedPage(page.pageNumber)}
            >
              <CardContent className="relative flex aspect-[3/4] flex-col items-center justify-center">
                <FileText size={32} className="text-muted-foreground/50" />
                <p className="mt-2 text-sm font-medium">第 {page.pageNumber} 页</p>
                <div className="absolute bottom-0 left-0 right-0 h-1 bg-muted">
                  <div
                    className="h-full bg-hot-500"
                    style={{ width: `${heatRatio * 100}%` }}
                  />
                </div>
                {page.hasEvidence && (
                  <>
                    <Badge className="absolute right-2 top-2 bg-primary text-primary-foreground text-[10px]">
                      证据
                    </Badge>
                    <span className="sr-only">此页包含 AI 证据高亮</span>
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
                  第 {selectedPage} 页详情
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
                      <p className="text-caption text-muted-foreground">浏览次数</p>
                      <p className="text-h2">{analytic?.viewCount ?? 0}</p>
                    </div>
                    <div>
                      <p className="text-caption text-muted-foreground">平均停留</p>
                      <p className="text-h2">{analytic?.avgDurationSeconds ?? 0}s</p>
                    </div>
                    <div>
                      <p className="text-caption text-muted-foreground">退出率</p>
                      <p className="text-h2">{Math.round((analytic?.exitRate ?? 0) * 100)}%</p>
                    </div>
                  </>
                );
              })()}
            </div>
            {evidences && evidences.filter((e) => e.pageNumber === selectedPage).length > 0 && (
              <div className="mt-4 rounded-md border border-border bg-muted/50 p-3">
                <p className="text-caption mb-2 text-muted-foreground">AI 证据高亮</p>
                {evidences
                  .filter((e) => e.pageNumber === selectedPage)
                  .map((ev) => (
                    <div key={ev.id} className="text-sm">
                      <span className="inline-block rounded bg-warning-500/20 px-1 py-0.5 text-xs font-medium text-warning-700">
                        原文定位
                      </span>
                      <p className="mt-1">{ev.text}</p>
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
