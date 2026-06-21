import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router";
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
import { Badge } from "@/components/ui/badge";
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
import { SkeletonList } from "@/components/common/SkeletonLayout";
import { api, formatDuration, formatRelativeTime } from "@/lib/api";
import type { Link } from "@/types";

export function LinksTable() {
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const [data, setData] = useState<Link[]>([]);
  const [loading, setLoading] = useState(true);
  const [sorting, setSorting] = useState<SortingState>([]);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        setLoading(true);
        const res = await api.getLinks();
        if (!cancelled) setData(res.data);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, []);

  const columns = useMemo<ColumnDef<Link>[]>(
    () => [
      {
        accessorKey: "shortUrl",
        header: "链接",
        cell: ({ row }) => {
          const link = row.original;
          return (
            <div className="flex min-w-0 items-center gap-2">
              <code className="truncate rounded bg-muted px-1.5 py-0.5 text-caption">{link.shortUrl}</code>
              <Button
                size="icon-sm"
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
        cell: ({ row }) => <span className="truncate text-sm">{row.original.documentTitle}</span>,
      },
      {
        accessorKey: "accessCount",
        header: "访问次数",
        cell: ({ row }) => (
          <span className="text-caption tabular-nums">{row.original.accessCount} views</span>
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
        header: "状态",
        cell: ({ row }) => {
          const active = row.original.isActive ?? true;
          return (
            <Badge variant="outline" className={active ? "text-success-500" : "text-muted-foreground"}>
              {active ? "启用" : "已停用"}
            </Badge>
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
                  onClick: () => setData((prev) => prev.filter((l) => l.id !== link.id)),
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

  if (loading) {
    return <SkeletonList rows={6} />;
  }

  if (data.length === 0) {
    return (
      <EmptyState
        icon={<LinkIcon size={64} />}
        title="暂无链接"
        description="为文档配置权限并创建链接，即可追踪投资人/客户的访问行为。"
        action={{
          label: "创建链接",
          onClick: () => navigate(`/${workspaceSlug}/links/new`),
        }}
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
                  <TableHead key={header.id} className={header.id === "actions" ? "w-[60px]" : ""}>
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
