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
import { useTranslation } from "react-i18next";
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
  "use no memo";
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const { t } = useTranslation("links");
  const { t: tc } = useTranslation("common");
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
        header: t("table.link"),
        cell: ({ row }) => {
          const link = row.original;
          return (
            <div className="flex min-w-0 items-center gap-2">
              <code className="truncate rounded bg-muted px-1.5 py-0.5 text-caption">{link.shortUrl}</code>
              <Button
                size="icon-sm"
                variant="ghost"
                aria-label={t("table.copyLink")}
                onClick={(e) => {
                  e.stopPropagation();
                  void copyToClipboard(link.shortUrl, t("detail.copySuccess"));
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
        header: t("table.document"),
        cell: ({ row }) => <span className="truncate text-sm">{row.original.documentTitle}</span>,
      },
      {
        accessorKey: "accessCount",
        header: t("table.accessCount"),
        cell: ({ row }) => (
          <span className="text-caption tabular-nums">{t("table.accessCountValue", { count: row.original.accessCount })}</span>
        ),
      },
      {
        accessorKey: "avgDurationSeconds",
        header: t("table.avgDuration"),
        cell: ({ row }) => (
          <span className="text-caption tabular-nums">
            {formatDuration(row.original.avgDurationSeconds || 0)}
          </span>
        ),
      },
      {
        accessorKey: "lastViewedAt",
        header: t("table.lastViewed"),
        cell: ({ row }) => (
          <span className="text-caption text-muted-foreground">
            {formatRelativeTime(row.original.lastViewedAt)}
          </span>
        ),
      },
      {
        accessorKey: "heatLevel",
        header: t("table.heat"),
        cell: ({ row }) => <HeatBadge level={row.original.heatLevel} />,
      },
      {
        accessorKey: "isActive",
        header: t("table.status"),
        cell: ({ row }) => {
          const active = row.original.isActive ?? true;
          return (
            <Badge variant="outline" className={active ? "text-success-500" : "text-muted-foreground"}>
              {active ? tc("status.enabled") : tc("status.inactive")}
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
                  label: t("actions.viewLog"),
                  icon: <ArrowRight size={16} />,
                  onClick: () => navigate(`/${workspaceSlug}/links/${link.id}`),
                },
                {
                  label: t("actions.exportData"),
                  icon: <Export size={16} />,
                  onClick: () => {},
                  disabled: true,
                  title: t("actions.exportProTooltip"),
                  pro: true,
                },
                {
                  label: t("actions.downloadOnly"),
                  icon: <DownloadSimple size={16} />,
                  onClick: () => {},
                  disabled: true,
                  title: t("actions.downloadProTooltip"),
                  pro: true,
                },
                {
                  label: tc("delete"),
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
    [navigate, workspaceSlug, t, tc]
  );

  // eslint-disable-next-line react-hooks/incompatible-library
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
        <Button onClick={refetch}>{tc("retry")}</Button>
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
        title={t("empty.title")}
        description={t("empty.description")}
        action={{
          label: t("empty.createLink"),
          onClick: () => navigate(`/${workspaceSlug}/links/new`),
        }}
      />
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-h2">{t("title.allLinks")}</h2>
        <span className="text-caption text-muted-foreground">{t("table.totalLinks", { count: data.length })}</span>
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
            <DialogTitle>{t("delete.title")}</DialogTitle>
            <DialogDescription>
              {t("delete.description", { url: linkToDelete?.shortUrl })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setLinkToDelete(null)} disabled={isDeleting}>
              {tc("cancel")}
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
                  toast.success(t("delete.success"));
                  setLinkToDelete(null);
                } catch (e) {
                  toast.error(e instanceof Error ? e.message : tc("error.deleteFailed"));
                } finally {
                  setIsDeleting(false);
                }
              }}
            >
              {isDeleting ? t("delete.confirmLoading") : tc("delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
