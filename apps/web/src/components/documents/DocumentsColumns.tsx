import { useMemo } from "react";
import { apiErrorMessage } from "@/lib/apiErrors";
import type { NavigateFunction } from "react-router";
import {
  Archive,
  ArrowCounterClockwise,
  Buildings,
  CaretDown,
  CaretUp,
  CaretUpDown,
  DownloadSimple,
  Eye,
  Link as LinkIcon,
  ShareNetwork,
  Trash,
} from "@phosphor-icons/react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { FileTypeIcon } from "@/components/common/FileTypeIcon";
import { HeatBadge } from "@/components/common/HeatBadge";
import { DocumentStatusBadge } from "./DocumentStatusBadge";
import { DocumentCategoryBadge } from "./DocumentCategoryBadge";
import { RowActions } from "@/components/common/RowActions";
import { formatDate, formatFileSize } from "@/lib/formatters";
import { documentsSharePath } from "@/lib/documentsSharePath";
import { canAddDocumentToDealRoom } from "@/lib/documentCategory";
import { cn } from "@/lib/utils";
import type { Column, ColumnDef } from "@tanstack/react-table";
import { documentHeatFromLinks } from "@/lib/heat/documentHeat";
import type { Document, HeatLevel, Link } from "@/types";

export interface DocumentRow extends Document {
  links: Link[];
  totalViews: number;
  heatLevel: HeatLevel;
}

export function buildDocumentRows(documents: Document[], links: Link[]): DocumentRow[] {
  const linksByDoc = links.reduce<Record<string, Link[]>>((acc, link) => {
    if (!link.documentId || link.dealRoomId) return acc;
    if (!acc[link.documentId]) acc[link.documentId] = [];
    acc[link.documentId].push(link);
    return acc;
  }, {});

  return documents.map((doc) => {
    const docLinks = linksByDoc[doc.id] ?? [];
    const totalViews = docLinks.reduce((sum, l) => sum + l.accessCount, 0);
    return {
      ...doc,
      links: docLinks,
      totalViews,
      // Same contract as Insights: document heat = max link heat.Compute.
      heatLevel: documentHeatFromLinks(docLinks),
    };
  });
}

interface UseDocumentColumnsOptions {
  workspaceSlug?: string;
  navigate: NavigateFunction;
  refetch?: () => void;
  onAddToDealRoom?: (doc: DocumentRow) => void;
  /** Opens archive confirm (link count + visitor revoke copy). Unarchive stays inline. */
  onArchive?: (doc: DocumentRow) => void;
  /** Opens library Share dialog (create + copy). Agreements omit this. */
  onShare?: (doc: DocumentRow) => void;
  onDelete?: (doc: DocumentRow) => void;
  returnTo?: string;
  returnLabel?: string;
}

function SortableHeader({ column, label }: { column: Column<DocumentRow>; label: string }) {
  const sorted = column.getIsSorted();
  return (
    <Button
      variant="ghost"
      size="sm"
      className={cn(
        "-ml-2 h-8 gap-1 px-2",
        "text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground/70",
        "hover:bg-transparent hover:text-foreground",
      )}
      onClick={column.getToggleSortingHandler()}
    >
      {label}
      {sorted === "asc" ? (
        <CaretUp size={12} />
      ) : sorted === "desc" ? (
        <CaretDown size={12} />
      ) : (
        <CaretUpDown size={12} className="opacity-50" />
      )}
    </Button>
  );
}

export function useDocumentColumns({
  workspaceSlug,
  navigate,
  refetch,
  onAddToDealRoom,
  onArchive,
  onShare,
  onDelete,
  returnTo,
  returnLabel,
}: UseDocumentColumnsOptions) {
  const { t } = useTranslation(["documents", "common"]);

  return useMemo<ColumnDef<DocumentRow>[]>(
    () => [
      {
        accessorKey: "title",
        header: () => (
          <span className="px-1 text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground/70">
            {t("documents:columns.file")}
          </span>
        ),
        cell: ({ row }) => {
          const doc = row.original;
          return (
            <div className="flex min-w-0 items-center gap-3.5 py-0.5">
              <FileTypeIcon
                type={doc.fileType}
                showLabel
                className="transition-transform duration-200 group-hover/doc-row:scale-[1.04]"
              />
              <div className="min-w-0">
                <div className="flex min-w-0 items-center gap-2">
                  <p className="truncate text-[13.5px] font-medium tracking-[-0.015em] text-foreground">
                    {doc.title}
                  </p>
                  <DocumentCategoryBadge category={doc.category} />
                </div>
                <p className="mt-0.5 truncate text-[11.5px] text-muted-foreground">
                  <span>{t("documents:columns.pages", { count: doc.pageCount })}</span>
                  <span className="mx-1.5 text-border">·</span>
                  <span>{formatFileSize(doc.fileSize)}</span>
                  <span className="mx-1.5 text-border">·</span>
                  <span>{formatDate(doc.createdAt)}</span>
                  <span className="mx-1.5 text-border">·</span>
                  <span>{t("documents:columns.links", { count: doc.links.length })}</span>
                </p>
              </div>
            </div>
          );
        },
      },
      {
        accessorKey: "status",
        header: () => (
          <span className="text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground/70">
            {t("documents:columns.status")}
          </span>
        ),
        cell: ({ row }) => (
          <DocumentStatusBadge
            status={row.original.status}
            progress={row.original.progress}
            errorMessage={row.original.ingestionJob?.errorMessage}
          />
        ),
      },
      {
        accessorKey: "heatLevel",
        header: ({ column }) => (
          <SortableHeader column={column} label={t("documents:columns.heat")} />
        ),
        sortingFn: (rowA, rowB) => {
          const rank = { hot: 2, warm: 1, cold: 0 } as const;
          const a = rank[rowA.original.heatLevel];
          const b = rank[rowB.original.heatLevel];
          if (a !== b) return a - b;
          return rowA.original.totalViews - rowB.original.totalViews;
        },
        cell: ({ row }) => (
          <HeatBadge level={row.original.heatLevel} className="font-medium" />
        ),
      },
      {
        accessorKey: "totalViews",
        header: ({ column }) => (
          <SortableHeader column={column} label={t("documents:columns.views")} />
        ),
        sortingFn: "basic",
        cell: ({ row }) => (
          <span className="font-mono text-[12px] tabular-nums tracking-tight text-muted-foreground">
            {row.original.totalViews}
          </span>
        ),
      },
      {
        id: "shareLinks",
        header: () => (
          <span className="text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground/70">
            {t("documents:columns.shareLinks")}
          </span>
        ),
        cell: ({ row }) => {
          const doc = row.original;
          return (
            <Button
              variant="ghost"
              size="sm"
              className={cn(
                "h-7 rounded-full px-2.5 text-xs font-medium",
                "text-muted-foreground hover:bg-foreground/[0.05] hover:text-foreground",
              )}
              onClick={(e) => {
                e.stopPropagation();
                navigate(
                  documentsSharePath(workspaceSlug!, {
                    documentId: doc.id,
                    documentTitle: doc.title,
                  }),
                );
              }}
            >
              {t("common:view")}
            </Button>
          );
        },
      },
      {
        id: "actions",
        header: () => (
          <span className="sr-only">{t("documents:columns.actions")}</span>
        ),
        cell: ({ row }) => {
          const doc = row.original;

          const busy = doc.status === "uploading" || doc.status === "processing";
          const archived = doc.status === "archived";
          const downloadReady = doc.status === "ready";
          const shareReady = doc.status === "ready" && !archived && Boolean(onShare);
          const showAddToDealRoom = Boolean(onAddToDealRoom) && canAddDocumentToDealRoom(doc.category);

          const handleArchive = async () => {
            if (!archived && onArchive) {
              onArchive(doc);
              return;
            }
            try {
              if (archived) {
                await api.unarchiveDocument(doc.id);
                toast.success(t("documents:columns.unarchived"));
              } else {
                // Fallback when parent omits confirm (should not happen in DocumentsTable).
                await api.archiveDocument(doc.id);
                toast.success(t("documents:columns.archived"));
              }
              refetch?.();
            } catch (e) {
              toast.error(apiErrorMessage(e, { messageKey: "documents:columns.archiveFailed" }));
            }
          };

          const handleDownload = async () => {
            try {
              const res = await api.getDocumentDownloadUrl(doc.id);
              const a = document.createElement("a");
              a.href = res.download_url;
              a.download = res.filename || doc.title;
              a.target = "_blank";
              a.rel = "noopener noreferrer";
              document.body.appendChild(a);
              a.click();
              a.remove();
            } catch (e) {
              toast.error(apiErrorMessage(e, { messageKey: "documents:columns.downloadFailed" }));
            }
          };

          return (
            <div
              className="flex items-center justify-end gap-0.5"
              onClick={(e) => e.stopPropagation()}
            >
              {shareReady ? (
                <Button
                  size="sm"
                  variant="ghost"
                  aria-label={t("documents:share.cta")}
                  className={cn(
                    "h-7 gap-1 rounded-full px-2.5 text-xs font-medium",
                    "text-muted-foreground hover:bg-foreground/[0.06] hover:text-foreground",
                    "opacity-80 group-hover/doc-row:opacity-100",
                  )}
                  onClick={(e) => {
                    e.stopPropagation();
                    onShare?.(doc);
                  }}
                  data-testid="document-row-share"
                >
                  <ShareNetwork size={14} />
                  {t("documents:share.cta")}
                </Button>
              ) : null}
              <Button
                size="icon-sm"
                variant="ghost"
                aria-label={t("common:preview")}
                className={cn(
                  "rounded-full text-muted-foreground/80 opacity-70",
                  "transition-[opacity,background-color,color,transform] duration-200",
                  "hover:bg-foreground/[0.06] hover:text-foreground hover:opacity-100",
                  "active:scale-[0.94] group-hover/doc-row:opacity-100",
                )}
                onClick={(e) => {
                  e.stopPropagation();
                  navigate(`/${workspaceSlug}/documents/${doc.id}`, {
                    state: returnTo ? { returnTo, returnLabel } : undefined,
                  });
                }}
              >
                <Eye size={16} />
              </Button>
              <RowActions
                actions={[
                  ...(onShare
                    ? [
                        {
                          label: t("documents:share.cta"),
                          icon: <ShareNetwork size={16} />,
                          onClick: () => onShare(doc),
                          disabled: busy || archived || doc.status !== "ready",
                          title: archived
                            ? t("documents:columns.archivedActionDisabled")
                            : doc.status !== "ready"
                              ? t("documents:share.notReady")
                              : undefined,
                        },
                      ]
                    : [
                        {
                          label: t("common:createLink"),
                          icon: <LinkIcon size={16} />,
                          onClick: () =>
                            navigate(`/${workspaceSlug}/links/new?documentId=${doc.id}`),
                          disabled: busy || archived,
                          title: archived
                            ? t("documents:columns.archivedActionDisabled")
                            : undefined,
                        },
                      ]),
                  ...(showAddToDealRoom
                    ? [
                        {
                          label: t("common:addToDealRoom"),
                          icon: <Buildings size={16} />,
                          onClick: () => onAddToDealRoom?.(doc),
                          disabled: busy || archived || doc.status === "failed",
                          title: archived ? t("documents:columns.archivedActionDisabled") : undefined,
                        },
                      ]
                    : []),
                  {
                    label: archived ? t("common:unarchive") : t("common:archive"),
                    icon: archived ? (
                      <ArrowCounterClockwise size={16} />
                    ) : (
                      <Archive size={16} />
                    ),
                    onClick: () => {
                      void handleArchive();
                    },
                    disabled: busy || doc.status === "failed",
                    title:
                      busy || doc.status === "failed"
                        ? t("documents:columns.archiveDisabled")
                        : undefined,
                  },
                  {
                    label: t("common:download"),
                    icon: <DownloadSimple size={16} />,
                    onClick: () => {
                      void handleDownload();
                    },
                    disabled: !downloadReady,
                    title: archived
                      ? t("documents:columns.archivedActionDisabled")
                      : downloadReady
                        ? undefined
                        : t("documents:columns.downloadNotReady"),
                  },
                  {
                    label: t("common:delete"),
                    icon: <Trash size={16} />,
                    onClick: () => onDelete?.(doc),
                    disabled: busy || !onDelete,
                    title: busy ? t("documents:columns.deleteBusy") : undefined,
                    destructive: true,
                  },
                ]}
              />
            </div>
          );
        },
      },
    ],
    [
      navigate,
      workspaceSlug,
      t,
      refetch,
      onAddToDealRoom,
      onArchive,
      onShare,
      onDelete,
      returnTo,
      returnLabel,
    ],
  );
}
