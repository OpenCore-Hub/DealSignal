import { useMemo } from "react";
import type { NavigateFunction } from "react-router";
import { Copy, DownloadSimple, Eye, Link as LinkIcon, Trash } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { FileTypeIcon } from "@/components/common/FileTypeIcon";
import { HeatBadge } from "@/components/common/HeatBadge";
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
  return useMemo<ColumnDef<DocumentRow>[]>(
    () => [
      {
        accessorKey: "title",
        header: "文件",
        cell: ({ row }) => {
          const doc = row.original;
          return (
            <div className="flex items-center gap-3">
              <FileTypeIcon type={doc.fileType} showLabel />
              <div className="min-w-0">
                <p className="truncate text-sm font-medium">{doc.title}</p>
                <p className="text-caption text-muted-foreground">
                  {doc.pageCount} 页 · {formatFileSize(doc.fileSize)} · {formatDate(doc.createdAt)} ·{" "}
                  {doc.links.length} 个链接
                </p>
              </div>
            </div>
          );
        },
      },
      {
        accessorKey: "heatLevel",
        header: "热度",
        cell: ({ row }) => <HeatBadge level={row.original.heatLevel} />,
      },
      {
        accessorKey: "totalViews",
        header: "访问次数",
        cell: ({ row }) => (
          <span className="text-caption tabular-nums">{row.original.totalViews} 次访问</span>
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
                aria-label="预览"
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
                    label: "创建链接",
                    icon: <LinkIcon size={16} />,
                    onClick: () => navigate(`/${workspaceSlug}/links/new?documentId=${doc.id}`),
                  },
                  ...(firstLink?.shortUrl
                    ? [
                        {
                          label: "复制链接",
                          icon: <Copy size={16} />,
                          onClick: () => copyToClipboard(firstLink.shortUrl, "链接已复制"),
                        },
                      ]
                    : []),
                  {
                    label: "下载",
                    icon: <DownloadSimple size={16} />,
                    onClick: () => {},
                    disabled: true,
                    title: "下载需后端签名 URL 支持",
                    pro: true,
                  },
                  {
                    label: "删除",
                    icon: <Trash size={16} />,
                    onClick: () => {},
                    disabled: true,
                    title: "删除文档需后端支持",
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
    [navigate, workspaceSlug]
  );
}
