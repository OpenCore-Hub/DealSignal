/* eslint-disable react-refresh/only-export-components */
import { useMemo } from "react";
import type { NavigateFunction } from "react-router";
import { Archive, ArrowCounterClockwise, Buildings, CaretDown, CaretUp, CaretUpDown, Copy, DownloadSimple, Eye, Link as LinkIcon, Trash } from "@phosphor-icons/react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { FileTypeIcon } from "@/components/common/FileTypeIcon";
import { HeatBadge } from "@/components/common/HeatBadge";
import { DocumentStatusBadge } from "./DocumentStatusBadge";
import { RowActions } from "@/components/common/RowActions";
import { formatDate, formatFileSize } from "@/lib/formatters";
import { copyToClipboard } from "@/lib/clipboard";
import type { Column, ColumnDef } from "@tanstack/react-table";
import type { Document, HeatLevel, Link } from "@/types";

export interface DocumentRow extends Document {
  links: Link[];
  totalViews: number;
  heatLevel: HeatLevel;
}

export function calculateHeatLevel(totalViews: number): HeatLevel {
  if (totalViews >= 30) return "hot";
  if (totalViews >= 5) return "warm";
  return "cold";
}

export function buildDocumentRows(documents: Document[], links: Link[]): DocumentRow[] {
  const linksByDoc = links.reduce<Record<string, Link[]>>((acc, link) => {
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
      heatLevel: calculateHeatLevel(totalViews),
    };
  });
}

interface UseDocumentColumnsOptions {
  workspaceSlug?: string;
  navigate: NavigateFunction;
  refetch?: () => void;
  onAddToDealRoom?: (doc: DocumentRow) => void;
}

function SortableHeader({ column, label }: { column: Column<DocumentRow>; label: string }) {
  const sorted = column.getIsSorted();
  return (
    <Button
      variant="ghost"
      size="sm"
      className="-ml-3 h-8 gap-1 px-2 font-medium"
      onClick={column.getToggleSortingHandler()}
    >
      {label}
      {sorted === "asc" ? (
        <CaretUp size={14} />
      ) : sorted === "desc" ? (
        <CaretDown size={14} />
      ) : (
        <CaretUpDown size={14} className="text-muted-foreground" />
      )}
    </Button>
  );
}

export function useDocumentColumns({ workspaceSlug, navigate, refetch, onAddToDealRoom }: UseDocumentColumnsOptions) {
  const { t } = useTranslation(["documents", "common"]);

  return useMemo<ColumnDef<DocumentRow>[]>(
    () => [
      {
        accessorKey: "title",
        header: t("documents:columns.file"),
        cell: ({ row }) => {
          const doc = row.original;
          return (
            <div className="flex items-center gap-3">
              <FileTypeIcon type={doc.fileType} showLabel />
              <div className="min-w-0">
                <p className="truncate text-sm font-medium">{doc.title}</p>
                <p className="text-caption text-muted-foreground">
                  {t("documents:columns.pages", { count: doc.pageCount })} · {formatFileSize(doc.fileSize)} · {formatDate(doc.createdAt)} ·{" "}
                  {t("documents:columns.links", { count: doc.links.length })}
                </p>
              </div>
            </div>
          );
        },
      },
      {
        accessorKey: "status",
        header: t("documents:columns.status"),
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
        header: ({ column }) => <SortableHeader column={column} label={t("documents:columns.heat")} />,
        sortingFn: (rowA, rowB) => {
          const rank = { hot: 2, warm: 1, cold: 0 } as const;
          const a = rank[rowA.original.heatLevel];
          const b = rank[rowB.original.heatLevel];
          if (a !== b) return a - b;
          return rowA.original.totalViews - rowB.original.totalViews;
        },
        cell: ({ row }) => <HeatBadge level={row.original.heatLevel} />,
      },
      {
        accessorKey: "totalViews",
        header: ({ column }) => <SortableHeader column={column} label={t("documents:columns.views")} />,
        sortingFn: "basic",
        cell: ({ row }) => (
          <span className="text-caption tabular-nums">
            {row.original.totalViews}
          </span>
        ),
      },
      {
        id: "shareLinks",
        header: t("documents:columns.shareLinks"),
        cell: ({ row }) => {
          const doc = row.original;
          return (
            <Button
              variant="ghost"
              size="sm"
              className="h-auto px-0 text-sm font-medium text-blue-600 hover:text-blue-700 hover:underline"
              onClick={(e) => {
                e.stopPropagation();
                navigate(`/${workspaceSlug}/links?documentId=${doc.id}&documentTitle=${encodeURIComponent(doc.title)}`);
              }}
            >
              {t("common:view")}
            </Button>
          );
        },
      },
      {
        id: "actions",
        header: t("documents:columns.actions"),
        cell: ({ row }) => {
          const doc = row.original;
          const firstLink = doc.links[0];

          const handleArchive = async () => {
            try {
              if (doc.status === "archived") {
                await api.unarchiveDocument(doc.id);
                toast.success(t("documents:columns.unarchived"));
              } else {
                await api.archiveDocument(doc.id);
                toast.success(t("documents:columns.archived"));
              }
              refetch?.();
            } catch (e) {
              toast.error(e instanceof Error ? e.message : t("documents:columns.archiveFailed"));
            }
          };

          return (
            <div className="flex items-center justify-start gap-1">
              <Button
                size="icon-sm"
                variant="ghost"
                aria-label={t("common:preview")}
                onClick={(e) => {
                  e.stopPropagation();
                  navigate(`/${workspaceSlug}/documents/${doc.id}`);
                }}
              >
                <Eye size={16} />
              </Button>
              <RowActions
                actions={[
                  {
                    label: t("common:createLink"),
                    icon: <LinkIcon size={16} />,
                    onClick: () => navigate(`/${workspaceSlug}/links/new?documentId=${doc.id}`),
                  },
                  ...(firstLink?.shortUrl
                    ? [
                        {
                          label: t("common:copyLink"),
                          icon: <Copy size={16} />,
                          onClick: () => copyToClipboard(firstLink.shortUrl, t("common:linkCopied")),
                        },
                      ]
                    : []),
                  {
                    label: t("common:addToDealRoom"),
                    icon: <Buildings size={16} />,
                    onClick: () => onAddToDealRoom?.(doc),
                    disabled: doc.status === "uploading" || doc.status === "processing" || doc.status === "failed",
                  },
                  {
                    label: doc.status === "archived" ? t("common:unarchive") : t("common:archive"),
                    icon: doc.status === "archived" ? <ArrowCounterClockwise size={16} /> : <Archive size={16} />,
                    onClick: handleArchive,
                    disabled: doc.status === "uploading" || doc.status === "processing" || doc.status === "failed",
                    title:
                      doc.status === "uploading" || doc.status === "processing" || doc.status === "failed"
                        ? t("documents:columns.archiveDisabled")
                        : undefined,
                  },
                  {
                    label: t("common:download"),
                    icon: <DownloadSimple size={16} />,
                    onClick: () => {},
                    disabled: true,
                    title: t("documents:columns.downloadDisabled"),
                    pro: true,
                  },
                  {
                    label: t("common:delete"),
                    icon: <Trash size={16} />,
                    onClick: () => {},
                    disabled: true,
                    title: t("documents:columns.deleteDisabled"),
                    destructive: true,
                    pro: true,
                  },
                ]}
              />
            </div>
          );
        },
      },
    ],
    [navigate, workspaceSlug, t, refetch, onAddToDealRoom]
  );
}
