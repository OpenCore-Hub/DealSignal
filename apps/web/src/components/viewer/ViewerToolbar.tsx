import {
  Download,
  MagnifyingGlassPlus,
  MagnifyingGlassMinus,
  CaretLeft,
  CaretRight,
} from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { formatFileSize } from "@/lib/formatters";
import type { Document } from "@/types";

interface ViewerToolbarProps {
  doc: Document;
  page: number;
  totalPages: number;
  zoom: number;
  onZoomOut: () => void;
  onZoomIn: () => void;
  onPreviousPage: () => void;
  onNextPage: () => void;
  onDownload: () => void;
}

export function ViewerToolbar({
  doc,
  page,
  totalPages,
  zoom,
  onZoomOut,
  onZoomIn,
  onPreviousPage,
  onNextPage,
  onDownload,
}: ViewerToolbarProps) {
  const { t } = useTranslation(["documents", "common"]);

  return (
    <header className="flex h-14 items-center justify-between border-b border-border bg-background px-4">
      <div className="flex items-center gap-3">
        <div className="flex h-7 w-7 items-center justify-center rounded-md bg-primary text-xs font-bold text-primary-foreground">
          D
        </div>
        <div>
          <p className="text-sm font-medium">{doc.title}</p>
          <p className="text-caption text-muted-foreground">
            {t("documents:viewer.meta", {
              fileType: doc.fileType.toUpperCase(),
              fileSize: formatFileSize(doc.fileSize),
              pageCount: totalPages,
            })}
          </p>
        </div>
      </div>

      <div className="flex items-center gap-1">
        <Button
          size="icon-sm"
          variant="ghost"
          onClick={onZoomOut}
          aria-label={t("documents:viewer.zoomOut")}
        >
          <MagnifyingGlassMinus size={16} />
        </Button>
        <span className="min-w-[3rem] text-center text-sm tabular-nums">{zoom}%</span>
        <Button
          size="icon-sm"
          variant="ghost"
          onClick={onZoomIn}
          aria-label={t("documents:viewer.zoomIn")}
        >
          <MagnifyingGlassPlus size={16} />
        </Button>
      </div>

      <div className="flex items-center gap-1">
        <Button
          size="icon-sm"
          variant="ghost"
          onClick={onPreviousPage}
          disabled={page <= 1}
          aria-label={t("documents:viewer.previousPage")}
        >
          <CaretLeft size={16} />
        </Button>
        <span className="min-w-[4rem] text-center text-sm tabular-nums">
          {page} / {totalPages}
        </span>
        <Button
          size="icon-sm"
          variant="ghost"
          onClick={onNextPage}
          disabled={page >= totalPages}
          aria-label={t("documents:viewer.nextPage")}
        >
          <CaretRight size={16} />
        </Button>
        <Button
          size="icon-sm"
          variant="ghost"
          aria-label={t("common:download")}
          onClick={onDownload}
        >
          <Download size={16} />
        </Button>
      </div>
    </header>
  );
}
