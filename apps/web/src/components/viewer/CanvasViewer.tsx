import { useState } from "react";
import { Download, MagnifyingGlassPlus, MagnifyingGlassMinus, CaretLeft, CaretRight } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";

export function CanvasViewer() {
  const [page, setPage] = useState(1);
  const [zoom, setZoom] = useState(100);
  const totalPages = 18;

  return (
    <div className="flex flex-1 flex-col bg-neutral-50 dark:bg-background">
      {/* Toolbar */}
      <header className="flex h-14 items-center justify-between border-b border-border bg-background px-4">
        <div className="flex items-center gap-3">
          <div className="flex h-7 w-7 items-center justify-center rounded-md bg-primary text-primary-foreground text-xs font-bold">
            D
          </div>
          <div>
            <p className="text-sm font-medium">Acme Pitch Deck.pdf</p>
            <p className="text-caption text-muted-foreground">机密 · 仅供查看</p>
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
          <Button size="icon-sm" variant="ghost" aria-label="下载">
            <Download size={16} />
          </Button>
        </div>
      </header>

      {/* Canvas area with watermark */}
      <div className="relative flex flex-1 items-center justify-center overflow-auto p-8">
        <div
          className="relative overflow-hidden rounded-md bg-white shadow-lg"
          style={{ width: `${zoom * 6}px`, height: `${zoom * 8}px`, minWidth: 300, minHeight: 400 }}
        >
          {/* Placeholder page content */}
          <div className="flex h-full w-full flex-col items-center justify-center gap-4 p-8 text-center text-slate-400">
            <div className="text-h1 text-slate-300">第 {page} 页</div>
            <p className="text-body">Canvas 渲染区</p>
            <p className="text-caption">实际文档内容将由后端签名 URL 加载并绘制</p>
          </div>

          {/* Watermark overlay */}
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
  );
}
