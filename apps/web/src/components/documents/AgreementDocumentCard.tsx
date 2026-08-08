import { FilePdf, Trash } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useAsyncData } from "@/hooks/useAsyncData";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";
import { formatDate } from "@/lib/formatters";
import { pageAspectRatioCSS } from "@/lib/projectPageGrid";
import { DocumentStatusBadge } from "./DocumentStatusBadge";
import type { DocumentRow } from "./DocumentsColumns";

interface AgreementDocumentCardProps {
  doc: DocumentRow;
  onOpen: () => void;
  onDelete?: () => void;
}

export function AgreementDocumentCard({
  doc,
  onOpen,
  onDelete,
}: AgreementDocumentCardProps) {
  const { t } = useTranslation(["agreementDocuments", "common", "documents"]);
  const previewReady = doc.status === "ready";
  const showStatus = doc.status !== "ready";

  const { data: signedUrlData, loading } = useAsyncData(async () => {
    if (!previewReady) return null;
    try {
      return await api.getPageSignedUrl(doc.id, 1);
    } catch {
      return null;
    }
  }, [doc.id, previewReady]);

  const imageUrl = signedUrlData?.image_url;
  const pageAspect = pageAspectRatioCSS(signedUrlData?.width, signedUrlData?.height);

  return (
    <div
      role="button"
      tabIndex={0}
      data-testid={`agreement-doc-card-${doc.id}`}
      className={cn(
        "group flex cursor-pointer flex-col text-left outline-none",
        "transition-transform duration-200 ease-out",
        "hover:-translate-y-0.5 active:translate-y-0 active:scale-[0.99]",
        "focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
      )}
      onClick={onOpen}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onOpen();
        }
      }}
    >
      <div
        className={cn(
          "relative overflow-hidden rounded-xl border border-border/80 bg-muted/40",
          "p-3 shadow-[0_1px_2px_rgba(15,23,42,0.04)]",
          "transition-[border-color,box-shadow,background-color] duration-200",
          "group-hover:border-border group-hover:bg-muted/55",
          "group-hover:shadow-[0_8px_24px_rgba(15,23,42,0.08)]",
        )}
        style={{ aspectRatio: pageAspect }}
      >
        <div className="relative h-full w-full overflow-hidden rounded-md bg-background shadow-sm ring-1 ring-black/[0.06]">
          {previewReady && loading ? (
            <Skeleton className="h-full w-full rounded-none" />
          ) : imageUrl ? (
            <img
              src={imageUrl}
              alt={doc.title}
              loading="lazy"
              decoding="async"
              className="h-full w-full object-cover object-top transition-transform duration-300 ease-out group-hover:scale-[1.015]"
            />
          ) : (
            <div className="flex h-full w-full flex-col items-center justify-center gap-2 bg-muted/20 px-3 text-muted-foreground">
              <FilePdf size={36} weight="duotone" className="text-muted-foreground/45" />
              <span className="text-center text-[11px] leading-snug tracking-wide">
                {previewReady
                  ? t("agreementDocuments:page.previewUnavailable")
                  : t(`documents:status.${doc.status}`)}
              </span>
            </div>
          )}
        </div>

        {showStatus ? (
          <div className="absolute left-2.5 top-2.5">
            <DocumentStatusBadge status={doc.status} progress={doc.progress} />
          </div>
        ) : null}

        {onDelete ? (
          <Button
            type="button"
            size="icon-sm"
            variant="secondary"
            className={cn(
              "absolute right-2.5 top-2.5 border border-border/70 bg-background/95 shadow-sm backdrop-blur-sm",
              "opacity-0 transition-opacity duration-150",
              "group-hover:opacity-100 group-focus-within:opacity-100",
            )}
            aria-label={t("common:delete")}
            data-testid={`agreement-doc-delete-${doc.id}`}
            onClick={(e) => {
              e.stopPropagation();
              onDelete();
            }}
          >
            <Trash size={14} />
          </Button>
        ) : null}
      </div>

      <div className="mt-3 space-y-1 px-0.5">
        <p
          className="truncate text-sm font-medium tracking-tight text-foreground"
          title={doc.title}
        >
          {doc.title}
        </p>
        <p className="truncate text-xs text-muted-foreground">
          {formatDate(doc.updatedAt || doc.createdAt)}
          {doc.pageCount > 0
            ? ` · ${t("documents:columns.pages", { count: doc.pageCount })}`
            : null}
        </p>
      </div>
    </div>
  );
}
