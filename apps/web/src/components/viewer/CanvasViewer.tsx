import { useEffect, useState } from "react";
import { useParams } from "react-router";
import {
  Download,
  MagnifyingGlassPlus,
  MagnifyingGlassMinus,
  CaretLeft,
  CaretRight,
  FileText,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { formatFileSize, formatDuration } from "@/lib/formatters";
import type { Document, PageAnalytics } from "@/types";

export function CanvasViewer() {
  const { documentId } = useParams<{ documentId: string }>();
  const [doc, setDoc] = useState<Document | null>(null);
  const [analytics, setAnalytics] = useState<PageAnalytics[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [retryTick, setRetryTick] = useState(0);
  const [page, setPage] = useState(1);
  const [zoom, setZoom] = useState(100);

  useEffect(() => {
    let cancelled = false;
    const id = documentId;
    if (!id) return;
    async function load() {
      try {
        setLoading(true);
        setError(null);
        const [d, a] = await Promise.all([api.getDocumentById(id!), api.getPageAnalytics(id!)]);
        if (!cancelled) {
          setDoc(d);
          setAnalytics(a.data);
          setPage(1);
        }
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : "加载失败");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [documentId, retryTick]);

  if (loading) {
    return (
      <div className="flex flex-1 flex-col bg-neutral-50 dark:bg-background">
        <header className="flex h-14 items-center border-b border-border bg-background px-4">
          <Skeleton className="h-8 w-64" />
        </header>
        <div className="flex flex-1">
          <Skeleton className="m-8 h-full w-48" />
          <Skeleton className="m-8 h-full flex-1" />
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center gap-4 bg-neutral-50 dark:bg-background">
        <FileText size={48} className="text-muted-foreground/50" />
        <p className="text-body text-destructive">加载失败：{error}</p>
        <Button onClick={() => setRetryTick((t) => t + 1)}>重试</Button>
      </div>
    );
  }

  if (!doc) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center bg-neutral-50 dark:bg-background">
        <FileText size={48} className="text-muted-foreground/50" />
        <p className="mt-4 text-body text-muted-foreground">文档不存在或无法加载</p>
      </div>
    );
  }

  const totalPages = doc.pageCount;
  const pageAnalytics = analytics.find((a) => a.pageNumber === page);
  const pages = Array.from({ length: totalPages }, (_, i) => {
    const num = i + 1;
    const a = analytics.find((x) => x.pageNumber === num);
    return { pageNumber: num, viewCount: a?.viewCount ?? 0, avgDurationSeconds: a?.avgDurationSeconds ?? 0 };
  });

  return (
    <div className="flex flex-1 flex-col bg-neutral-50 dark:bg-background">
      <header className="flex h-14 items-center justify-between border-b border-border bg-background px-4">
        <div className="flex items-center gap-3">
          <div className="flex h-7 w-7 items-center justify-center rounded-md bg-primary text-xs font-bold text-primary-foreground">
            D
          </div>
          <div>
            <p className="text-sm font-medium">{doc.title}</p>
            <p className="text-caption text-muted-foreground">
              {doc.fileType.toUpperCase()} · {formatFileSize(doc.fileSize)} · {totalPages} 页
            </p>
          </div>
        </div>

        <div className="flex items-center gap-1">
          <Button
            size="icon-sm"
            variant="ghost"
            onClick={() => setZoom((z) => Math.max(50, z - 10))}
            aria-label="缩小"
          >
            <MagnifyingGlassMinus size={16} />
          </Button>
          <span className="min-w-[3rem] text-center text-sm tabular-nums">{zoom}%</span>
          <Button
            size="icon-sm"
            variant="ghost"
            onClick={() => setZoom((z) => Math.min(200, z + 10))}
            aria-label="放大"
          >
            <MagnifyingGlassPlus size={16} />
          </Button>
        </div>

        <div className="flex items-center gap-1">
          <Button
            size="icon-sm"
            variant="ghost"
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={page <= 1}
            aria-label="上一页"
          >
            <CaretLeft size={16} />
          </Button>
          <span className="min-w-[4rem] text-center text-sm tabular-nums">
            {page} / {totalPages}
          </span>
          <Button
            size="icon-sm"
            variant="ghost"
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            disabled={page >= totalPages}
            aria-label="下一页"
          >
            <CaretRight size={16} />
          </Button>
          <Button size="icon-sm" variant="ghost" aria-label="下载" disabled title="下载需后端签名 URL 支持" onClick={() => {}}>
            <Download size={16} />
          </Button>
        </div>
      </header>

      <div className="flex flex-1 overflow-hidden">
        {/* Thumbnail sidebar */}
        <aside className="hidden w-48 flex-col gap-2 overflow-y-auto border-r border-border bg-card p-3 md:flex">
          <p className="text-caption font-medium text-muted-foreground">页面热度</p>
          {pages.map((p) => {
            const heat = p.viewCount > 0 ? Math.min(100, (p.viewCount / Math.max(...pages.map((x) => x.viewCount), 1)) * 100) : 0;
            return (
              <button
                key={p.pageNumber}
                type="button"
                onClick={() => setPage(p.pageNumber)}
                className={`flex flex-col gap-1 rounded-md border p-2 text-left transition-colors ${
                  page === p.pageNumber
                    ? "border-primary bg-primary/5"
                    : "border-border bg-background hover:bg-muted"
                }`}
              >
                <span className="text-xs font-medium">第 {p.pageNumber} 页</span>
                <div className="h-1 w-full overflow-hidden rounded-full bg-muted">
                  <div className="h-full rounded-full bg-hot-500" style={{ width: `${heat}%` }} />
                </div>
                <span className="text-caption text-muted-foreground">
                  {p.viewCount} 次访问 · {formatDuration(p.avgDurationSeconds)}
                </span>
              </button>
            );
          })}
        </aside>

        {/* Canvas area */}
        <div className="relative flex flex-1 items-center justify-center overflow-auto p-8">
          <div
            className="relative overflow-hidden rounded-md bg-white shadow-card"
            style={{ width: `${zoom * 6}px`, height: `${zoom * 8}px`, minWidth: 300, minHeight: 400 }}
          >
            <div className="flex h-full w-full flex-col items-center justify-center gap-4 p-8 text-center text-muted-foreground">
              <div className="text-h1 text-muted-foreground">第 {page} 页</div>
              <p className="text-body text-muted-foreground">文档预览占位</p>
              <p className="text-caption max-w-xs">
                后端签名 URL 加载后将在此渲染真实页面内容。
                {pageAnalytics && (
                  <>
                    <br />
                    当前页浏览 {pageAnalytics.viewCount} 次，平均停留{" "}
                    {formatDuration(pageAnalytics.avgDurationSeconds)}。
                  </>
                )}
              </p>
            </div>

            <div className="pointer-events-none absolute inset-0 flex rotate-[-30deg] flex-wrap items-center justify-center gap-16 opacity-[0.08]">
              {Array.from({ length: 12 }).map((_, i) => (
                <span key={i} className="whitespace-nowrap text-2xl font-bold text-foreground">
                  viewer@dealsignal.com
                </span>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
