import {
  CaretLeft,
  CaretRight,
  DownloadSimple,
  MagnifyingGlassMinus,
  MagnifyingGlassPlus,
  Sparkle,
  SquaresFour,
} from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { formatFileSize } from "@/lib/formatters";
import { cn } from "@/lib/utils";
import type { Document } from "@/types";

interface PublicViewerToolbarProps {
  doc: Document;
  page: number;
  totalPages: number;
  zoom: number;
  onZoomOut: () => void;
  onZoomIn: () => void;
  onPreviousPage: () => void;
  onNextPage: () => void;
  onDownload: () => void;
  sidebarOpen?: boolean;
  onToggleSidebar?: () => void;
  linkName?: string;
}

export function PublicViewerToolbar({
  doc,
  page,
  totalPages,
  zoom,
  onZoomOut,
  onZoomIn,
  onPreviousPage,
  onNextPage,
  onDownload,
  sidebarOpen = false,
  onToggleSidebar,
  linkName,
}: PublicViewerToolbarProps) {
  const { t } = useTranslation(["documents", "common"]);

  return (
    <header className="public-viewer-glass relative z-10 mx-3 mt-3 flex shrink-0 items-center gap-3 rounded-2xl px-3 py-2.5 sm:mx-4 sm:px-4">
      <div className="flex min-w-0 flex-1 items-center gap-3">
        <div className="relative flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-xl bg-gradient-to-br from-emerald-600 to-slate-900 text-white shadow-sm">
          <Sparkle size={16} weight="fill" className="opacity-90" />
        </div>
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold tracking-tight text-foreground">{doc.title}</p>
          <p className="truncate text-[11px] text-muted-foreground">
            {linkName ? `${linkName} · ` : ""}
            {t("documents:viewer.meta", {
              fileType: doc.fileType.toUpperCase(),
              fileSize: formatFileSize(doc.fileSize),
              pageCount: totalPages,
            })}
          </p>
        </div>
      </div>

      <div className="hidden items-center gap-0.5 rounded-full border border-border/60 bg-background/70 p-0.5 sm:flex">
        <Button
          size="icon-sm"
          variant="ghost"
          className="h-8 w-8 rounded-full"
          onClick={onZoomOut}
          aria-label={t("documents:viewer.zoomOut")}
        >
          <MagnifyingGlassMinus size={15} />
        </Button>
        <span className="min-w-[3rem] px-1 text-center text-xs font-medium tabular-nums text-muted-foreground">
          {zoom}%
        </span>
        <Button
          size="icon-sm"
          variant="ghost"
          className="h-8 w-8 rounded-full"
          onClick={onZoomIn}
          aria-label={t("documents:viewer.zoomIn")}
        >
          <MagnifyingGlassPlus size={15} />
        </Button>
      </div>

      <div className="flex shrink-0 items-center gap-1 rounded-full border border-border/60 bg-background/70 p-0.5">
        <Button
          size="icon-sm"
          variant="ghost"
          className="h-8 w-8 rounded-full"
          onClick={onPreviousPage}
          disabled={page <= 1}
          aria-label={t("documents:viewer.previousPage")}
        >
          <CaretLeft size={15} />
        </Button>
        <span className="min-w-[4.5rem] px-1 text-center text-xs font-semibold tabular-nums">
          {page}
          <span className="mx-0.5 font-normal text-muted-foreground">/</span>
          {totalPages}
        </span>
        <Button
          size="icon-sm"
          variant="ghost"
          className="h-8 w-8 rounded-full"
          onClick={onNextPage}
          disabled={page >= totalPages}
          aria-label={t("documents:viewer.nextPage")}
        >
          <CaretRight size={15} />
        </Button>
      </div>

      <div className="flex items-center gap-1">
        <Button
          size="icon-sm"
          variant="ghost"
          className="h-9 w-9 rounded-xl"
          aria-label={t("common:download")}
          onClick={onDownload}
        >
          <DownloadSimple size={16} />
        </Button>
        {onToggleSidebar && (
          <Button
            size="icon-sm"
            variant={sidebarOpen ? "default" : "ghost"}
            className={cn("h-9 w-9 rounded-xl", sidebarOpen && "shadow-sm")}
            onClick={onToggleSidebar}
            aria-label={
              sidebarOpen
                ? t("documents:viewer.sidebarClose")
                : t("documents:viewer.sidebarOpen")
            }
            title={
              sidebarOpen
                ? t("documents:viewer.sidebarClose")
                : t("documents:viewer.workspaceOpen")
            }
          >
            <SquaresFour size={16} />
          </Button>
        )}
      </div>
    </header>
  );
}
