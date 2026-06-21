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
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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
import { api } from "@/lib/api";
import { formatDuration, formatRelativeTime } from "@/lib/formatters";
import { copyToClipboard } from "@/lib/clipboard";
import { useAsyncData } from "@/hooks/useAsyncData";
import { toast } from "sonner";
import type { Link } from "@/types";

export function LinksTable() {
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const { data: fetchedData, loading, error, refetch } = useAsyncData(async () => {
    const res = await api.getLinks();
    return res.data;
  }, []);
  const [data, setData] = useState<Link[]>([]);
  const [sorting, setSorting] = useState<SortingState>([]);
  const [linkToDelete, setLinkToDelete] = useState<Link | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  useEffect(() => {
    if (fetchedData) setData(fetchedData);
  }, [fetchedData]);

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
                  copyToClipboard(link.shortUrl, "链接已复制");
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
          <span className="text-caption tabular-nums">{row.original.accessCount} 次访问</span>
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
                  disabled: true,
                  title: "导出需后端支持",
                  pro: true,
                },
                {
                  label: "仅允许下载",
                  icon: <DownloadSimple size={16} />,
                  onClick: () => {},
                  disabled: true,
                  title: "仅允许下载需后端支持",
                  pro: true,
                },
                {
                  label: "删除",
                  icon: <Trash size={16} />,
                  onClick: () => setLinkToDelete(link),
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

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
        <p className="text-body text-muted-foreground">{error}</p>
        <Button onClick={refetch}>重试</Button>
      </div>
    );
  }

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
                role="link"
                tabIndex={0}
                onClick={() => navigate(`/${workspaceSlug}/links/${row.original.id}`)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    navigate(`/${workspaceSlug}/links/${row.original.id}`);
                  }
                }}
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

      <Dialog open={!!linkToDelete} onOpenChange={(open) => !open && setLinkToDelete(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>删除链接</DialogTitle>
            <DialogDescription>
              确定要删除「{linkToDelete?.shortUrl}」吗？关联的访问记录将不再可通过该链接查看。
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setLinkToDelete(null)} disabled={isDeleting}>
              取消
            </Button>
            <Button
              variant="destructive"
              disabled={isDeleting}
              onClick={async () => {
                if (!linkToDelete) return;
                setIsDeleting(true);
                try {
                  await api.updateLink(linkToDelete.id, { isActive: false });
                  setData((prev) => prev.map((l) => (l.id === linkToDelete.id ? { ...l, isActive: false } : l)));
                  toast.success("链接已删除");
                  setLinkToDelete(null);
                } catch (e) {
                  toast.error(e instanceof Error ? e.message : "删除失败");
                } finally {
                  setIsDeleting(false);
                }
              }}
            >
              {isDeleting ? "删除中..." : "删除"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
