import { useMemo } from "react";
import type { NavigateFunction } from "react-router";
import { Copy, DownloadSimple, Eye, Link as LinkIcon, Trash } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { FileTypeIcon } from "@/components/common/FileTypeIcon";
import { HeatBadge } from "@/components/common/HeatBadge";
import { DocumentStatusBadge } from "./DocumentStatusBadge";
import { RowActions } from "@/components/common/RowActions";
import { formatDate, formatFileSize } from "@/lib/formatters";
import { copyToClipboard } from "@/lib/clipboard";
import type { ColumnDef } from "@tanstack/react-table";
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
}

export function useDocumentColumns({ workspaceSlug, navigate }: UseDocumentColumnsOptions) {
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
        header: t("documents:columns.heat"),
        cell: ({ row }) => <HeatBadge level={row.original.heatLevel} />,
      },
      {
        accessorKey: "totalViews",
        header: t("documents:columns.views"),
        cell: ({ row }) => (
          <span className="text-caption tabular-nums">
            {t("documents:columns.viewCount", { count: row.original.totalViews })}
          </span>
        ),
      },
      {
        id: "actions",
        header: "",
        cell: ({ row }) => {
          const doc = row.original;
          const firstLink = doc.links[0];
          return (
            <div className="flex items-center justify-end gap-1">
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
    [navigate, workspaceSlug, t]
  );
}
