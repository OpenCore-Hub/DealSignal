import { useMemo, useState } from "react";
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  flexRender,
  type ColumnDef,
  type SortingState,
} from "@tanstack/react-table";
import { Copy, Export, DownloadSimple, Trash, ArrowRight, Link as LinkIcon } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { HeatBadge } from "@/components/common/HeatBadge";
import { RowActions } from "@/components/common/RowActions";
import { EmptyState } from "@/components/common/EmptyState";
import { formatDuration, formatRelativeTime } from "@/lib/api";
import { mockLinks } from "@/lib/mocks/data";
import type { Link } from "@/types";
import { useNavigate, useParams } from "react-router";

export function LinksTable() {
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const [data, setData] = useState<Link[]>(mockLinks);
  const [sorting, setSorting] = useState<SortingState>([]);

  const columns = useMemo<ColumnDef<Link>[]>(
    () => [
      {
        accessorKey: "shortUrl",
        header: "链接",
        cell: ({ row }) => {
          const link = row.original;
          return (
            <div className="flex min-w-0 items-center gap-2">
              <code className="truncate rounded bg-muted px-1.5 py-0.5 text-caption">
                {link.shortUrl}
              </code>
              <Button
                size="icon-xs"
                variant="ghost"
                aria-label="复制链接"
                onClick={(e) => {
                  e.stopPropagation();
                  navigator.clipboard.writeText(link.shortUrl);
                }}
              >
                <Copy size={14} />
              </Button>
            </div>
          );
        },
      },
      {
        accessorKey: "documentTitle",
        header: "文档",
        cell: ({ row }) => (
          <span className="truncate text-sm">{row.original.documentTitle}</span>
        ),
      },
      {
        accessorKey: "accessCount",
        header: "访问次数",
        cell: ({ row }) => (
          <span className="text-caption tabular-nums">
            {row.original.accessCount} views
          </span>
        ),
      },
      {
        accessorKey: "avgDurationSeconds",
        header: "平均时长",
        cell: ({ row }) => (
          <span className="text-caption tabular-nums">
            {formatDuration(row.original.avgDurationSeconds || 0)}
          </span>
        ),
      },
      {
        accessorKey: "lastViewedAt",
        header: "最近访问",
        cell: ({ row }) => (
          <span className="text-caption text-muted-foreground">
            {formatRelativeTime(row.original.lastViewedAt)}
          </span>
        ),
      },
      {
        accessorKey: "heatLevel",
        header: "热度",
        cell: ({ row }) => <HeatBadge level={row.original.heatLevel} />,
      },
      {
        accessorKey: "isActive",
        header: "启用",
        cell: ({ row }) => {
          const link = row.original;
          return (
            <div className="flex items-center gap-2">
              <Switch
                checked={link.isActive ?? true}
                onCheckedChange={(checked) => {
                  setData((prev) =>
                    prev.map((l) =>
                      l.id === link.id ? { ...l, isActive: checked } : l
                    )
                  );
                }}
                onClick={(e) => e.stopPropagation()}
                aria-label={link.isActive ? "停用链接" : "启用链接"}
              />
              <span className="text-caption text-muted-foreground">
                {link.isActive ? "Yes" : "No"}
              </span>
            </div>
          );
        },
      },
      {
        id: "actions",
        header: "",
        cell: ({ row }) => {
          const link = row.original;
          return (
            <RowActions
              actions={[
                {
                  label: "查看日志",
                  icon: <ArrowRight size={16} />,
                  onClick: () => navigate(`/${workspaceSlug}/links/${link.id}`),
                },
                {
                  label: "导出访问数据",
                  icon: <Export size={16} />,
                  onClick: () => {},
                  pro: true,
                },
                {
                  label: "仅允许下载",
                  icon: <DownloadSimple size={16} />,
                  onClick: () => {},
                  pro: true,
                },
                {
                  label: "删除",
                  icon: <Trash size={16} />,
                  onClick: () =>
                    setData((prev) => prev.filter((l) => l.id !== link.id)),
                  destructive: true,
                },
              ]}
            />
          );
        },
      },
    ],
    [navigate, workspaceSlug]
  );

  const table = useReactTable({
    data,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  });

  if (data.length === 0) {
    return (
      <EmptyState
        icon={<LinkIcon size={48} />}
        title="暂无链接"
        description="为文档配置权限并创建链接，即可追踪投资人/客户的访问行为。"
      />
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-h2">全部链接</h2>
        <span className="text-caption text-muted-foreground">{data.length} 个链接</span>
      </div>

      <div className="rounded-lg border border-border bg-card">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <TableHead
                    key={header.id}
                    className={header.id === "actions" ? "w-[60px]" : ""}
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
            {table.getRowModel().rows.map((row) => (
              <TableRow
                key={row.id}
                className="cursor-pointer"
                onClick={() => navigate(`/${workspaceSlug}/links/${row.original.id}`)}
              >
                {row.getVisibleCells().map((cell) => (
                  <TableCell key={cell.id}>
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
