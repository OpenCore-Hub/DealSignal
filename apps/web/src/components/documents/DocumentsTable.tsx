import { useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router";
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  getFilteredRowModel,
  flexRender,
  type ColumnDef,
  type SortingState,
} from "@tanstack/react-table";
import {
  Copy,
  DownloadSimple,
  Eye,
  Link as LinkIcon,
  MagnifyingGlass,
  Plus,
  Trash,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { FileTypeIcon } from "@/components/common/FileTypeIcon";
import { HeatBadge } from "@/components/common/HeatBadge";
import { RowActions } from "@/components/common/RowActions";
import { EmptyState } from "@/components/common/EmptyState";
import { formatFileSize, formatDate } from "@/lib/api";
import { mockDocuments, mockLinks } from "@/lib/mocks/data";
import type { Document, HeatLevel } from "@/types";

interface DocumentRow extends Document {
  linkCount: number;
  totalViews: number;
  heatLevel: HeatLevel;
}

function useDocumentRows(): DocumentRow[] {
  return useMemo(() => {
    return mockDocuments.map((doc) => {
      const links = mockLinks.filter((l) => l.documentId === doc.id);
      const totalViews = links.reduce((sum, l) => sum + l.accessCount, 0);
      const heatLevel: HeatLevel =
        totalViews > 30 ? "hot" : totalViews > 5 ? "warm" : "cold";
      return {
        ...doc,
        linkCount: links.length,
        totalViews,
        heatLevel,
      };
    });
  }, []);
}

export function DocumentsTable() {
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const [sorting, setSorting] = useState<SortingState>([]);
  const [globalFilter, setGlobalFilter] = useState("");
  const data = useDocumentRows();

  const columns = useMemo<ColumnDef<DocumentRow>[]>(
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
                  {doc.pageCount} 页 · {formatFileSize(doc.fileSize)} ·{" "}
                  {formatDate(doc.createdAt)} · {doc.linkCount} 个链接
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
          <span className="text-caption tabular-nums">
            {row.original.totalViews} views
          </span>
        ),
      },
      {
        id: "actions",
        header: "",
        cell: ({ row }) => {
          const doc = row.original;
          return (
            <div className="flex items-center justify-end gap-1">
              <Button
                size="icon-xs"
                variant="ghost"
                aria-label="预览"
                onClick={() => navigate(`/${workspaceSlug}/documents/${doc.id}`)}
              >
                <Eye size={16} />
              </Button>
              <RowActions
                actions={[
                  {
                    label: "创建链接",
                    icon: <LinkIcon size={16} />,
                    onClick: () => navigate(`/${workspaceSlug}/links`),
                  },
                  {
                    label: "复制链接",
                    icon: <Copy size={16} />,
                    onClick: () =>
                      navigator.clipboard.writeText(
                        `https://invest.${workspaceSlug}.capital/d/${doc.id}`
                      ),
                  },
                  {
                    label: "下载",
                    icon: <DownloadSimple size={16} />,
                    onClick: () => {},
                    pro: true,
                  },
                  {
                    label: "删除",
                    icon: <Trash size={16} />,
                    onClick: () => {},
                    destructive: true,
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

  const table = useReactTable({
    data,
    columns,
    state: { sorting, globalFilter },
    onSortingChange: setSorting,
    onGlobalFilterChange: setGlobalFilter,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
  });

  if (data.length === 0) {
    return (
      <EmptyState
        icon={<LinkIcon size={48} />}
        title="文档库为空"
        description="上传第一份文档，即可创建安全分享链接并追踪投资人/客户的阅读热度。"
        action={{
          label: "上传文档",
          onClick: () => navigate(`/${workspaceSlug}/documents/upload`),
        }}
      />
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="relative max-w-sm flex-1">
          <MagnifyingGlass
            size={18}
            className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"
          />
          <Input
            placeholder="搜索文档..."
            value={globalFilter}
            onChange={(e) => setGlobalFilter(e.target.value)}
            className="pl-9"
          />
        </div>
        <Button
          onClick={() => navigate(`/${workspaceSlug}/documents/upload`)}
          className="gap-1.5"
        >
          <Plus size={16} weight="bold" />
          上传文档
        </Button>
      </div>

      <p className="text-caption text-muted-foreground">
        {data.length} 个文档
        {globalFilter && ` · 筛选后 ${table.getRowModel().rows.length} 个`}
      </p>

      <div className="rounded-lg border border-border bg-card">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <TableHead
                    key={header.id}
                    className={
                      header.id === "actions" ? "w-[100px] text-right" : ""
                    }
                  >
                    {header.isPlaceholder
                      ? null
                      : flexRender(header.column.columnDef.header, header.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows.length === 0 ? (
              <TableRow>
                <TableCell colSpan={columns.length} className="h-32 text-center text-muted-foreground">
                  没有找到匹配的文档
                </TableCell>
              </TableRow>
            ) : (
              table.getRowModel().rows.map((row) => (
                <TableRow
                  key={row.id}
                  className="cursor-pointer"
                  onClick={() => navigate(`/${workspaceSlug}/documents/${row.original.id}`)}
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id}>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
