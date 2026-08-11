import { useCallback, useMemo, useState } from "react";
import { apiErrorMessage } from "@/lib/apiErrors";
import { useLocation, useNavigate, useParams } from "react-router";
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  flexRender,
  type ColumnDef,
  type SortingState,
} from "@tanstack/react-table";
import {
  Copy,
  Export,
  DownloadSimple,
  Trash,
  ArrowRight,
  Link as LinkIcon,
  MagnifyingGlass,
  X,
  PencilSimple,
} from "@phosphor-icons/react";
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
import { SortableColumnHeader } from "@/components/common/SortableColumnHeader";
import { ShareAccessRequestsPanel } from "@/components/links/ShareAccessRequestsPanel";
import { api } from "@/lib/api";
import { documentsCreateLinkPath, documentsSharePath } from "@/lib/documentsSharePath";
import { exportLinkAccessLogsCsv } from "@/lib/exportLinkAccessLogs";
import { formatDuration, formatRelativeTime } from "@/lib/formatters";
import { copyToClipboard } from "@/lib/clipboard";
import {
  filterLinksForShareView,
  hasActiveShareListFilters,
  type LinkCreatedWithin,
} from "@/lib/shareLinksFilter";
import { useAsyncData } from "@/hooks/useAsyncData";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import type { Link } from "@/types";

interface LinksTableProps {
  documentId?: string;
  documentTitle?: string;
  /** Compact layout when embedded in Document Library → Share tab. */
  embedded?: boolean;
  /** Client-side search over URL / document title / link name. */
  searchQuery?: string;
  /** Filter by link create time window. */
  createdWithin?: LinkCreatedWithin;
  /** Clears Share-tab search + create-time filters (parent owns URL/state). */
  onClearListFilters?: () => void;
}

/** Column layout: long URL/title truncate; table scrolls horizontally when narrow. */
function linksColumnClass(columnId: string): string {
  switch (columnId) {
    case "shortUrl":
      // Fixed width so truncate works under table-fixed layout.
      return "w-[20rem] max-w-[20rem] overflow-hidden";
    case "documentTitle":
      return "w-[16rem] max-w-[16rem] overflow-hidden";
    case "accessCount":
      return "w-[6.5rem] whitespace-nowrap";
    case "avgDurationSeconds":
      return "w-[4rem] whitespace-nowrap";
    case "createdAt":
      return "w-[8.5rem] whitespace-nowrap";
    case "heatLevel":
      return "w-[5.5rem] whitespace-nowrap";
    case "isActive":
      return "w-[6.5rem] whitespace-nowrap";
    case "actions":
      return "w-[3.75rem]";
    default:
      return "whitespace-nowrap";
  }
}

export function LinksTable({
  documentId,
  documentTitle,
  embedded = false,
  searchQuery = "",
  createdWithin = "all",
  onClearListFilters,
}: LinksTableProps) {
  "use no memo";
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const location = useLocation();
  const { t } = useTranslation("links");
  const { t: tc } = useTranslation("common");

  const openLink = (linkId: string) => {
    navigate(`/${workspaceSlug}/links/${linkId}`, {
      state: {
        returnTo: location.pathname + location.search,
        returnLabel: t("backToLinks"),
      },
    });
  };
  const isFiltered = !!documentId;
  const { data: fetchedData, loading, error, refetch } = useAsyncData(async () => {
    const res = isFiltered && documentId
      ? await api.getLinksByDocumentId(documentId)
      : await api.getLinks();
    // Defense in depth: Document Library must never render deal-room shares.
    return res.data.filter((link) => !link.dealRoomId);
  }, [documentId, isFiltered]);
  const allLinks = useMemo(() => fetchedData ?? [], [fetchedData]);
  const listFiltersActive = hasActiveShareListFilters({ searchQuery, createdWithin });
  const data = useMemo(
    () =>
      filterLinksForShareView(allLinks, {
        searchQuery,
        createdWithin,
      }),
    [allLinks, searchQuery, createdWithin],
  );
  const [sorting, setSorting] = useState<SortingState>([{ id: "createdAt", desc: true }]);
  const [linkToDelete, setLinkToDelete] = useState<Link | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [busyLinkId, setBusyLinkId] = useState<string | null>(null);

  const handleExportAccessData = useCallback(
    async (link: Link) => {
      setBusyLinkId(link.id);
      try {
        const count = await exportLinkAccessLogsCsv(
          link.id,
          link.documentTitle || link.name || link.id,
          [
            t("export.csv.timestamp"),
            t("export.csv.visitorEmail"),
            t("export.csv.visitorName"),
            t("export.csv.pageNumber"),
            t("export.csv.durationSeconds"),
            t("export.csv.device"),
            t("export.csv.location"),
          ],
        );
        if (count === 0) {
          toast.message(t("actions.exportEmpty"));
        } else {
          toast.success(t("actions.exportSuccess", { count }));
        }
      } catch {
        toast.error(t("actions.exportError"));
      } finally {
        setBusyLinkId(null);
      }
    },
    [t],
  );

  const handleToggleDownload = useCallback(
    async (link: Link) => {
      const next = !(link.downloadEnabled ?? false);
      setBusyLinkId(link.id);
      try {
        await api.updateLink(link.id, { downloadEnabled: next });
        toast.success(
          next ? t("actions.downloadEnabledSuccess") : t("actions.downloadDisabledSuccess"),
        );
        await refetch();
      } catch {
        toast.error(t("actions.downloadToggleError"));
      } finally {
        setBusyLinkId(null);
      }
    },
    [refetch, t],
  );

  const columns = useMemo<ColumnDef<Link>[]>(
    () => [
      {
        accessorKey: "shortUrl",
        header: t("table.link"),
        cell: ({ row }) => {
          const link = row.original;
          return (
            <div className="flex min-w-0 max-w-full items-center gap-2">
              <code
                className="block min-w-0 flex-1 truncate rounded bg-muted px-1.5 py-0.5 text-caption"
                title={link.shortUrl}
              >
                {link.shortUrl}
              </code>
              <Button
                size="icon-sm"
                variant="ghost"
                className="shrink-0"
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
        cell: ({ row }) => (
          <span className="block truncate text-sm" title={row.original.documentTitle}>
            {row.original.documentTitle}
          </span>
        ),
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
        accessorKey: "createdAt",
        enableSorting: true,
        header: ({ column }) => (
          <SortableColumnHeader column={column} label={t("table.createTime")} />
        ),
        cell: ({ row }) => (
          <span className="text-caption text-muted-foreground">
            {formatRelativeTime(row.original.createdAt)}
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
          const busy = busyLinkId === link.id;
          const downloadOn = link.downloadEnabled ?? false;
          return (
            <RowActions
              actions={[
                {
                  label: t("actions.viewLog"),
                  icon: <ArrowRight size={16} />,
                  onClick: () => navigate(`/${workspaceSlug}/links/${link.id}`),
                },
                {
                  label: t("actions.editLink"),
                  icon: <PencilSimple size={16} />,
                  onClick: () => navigate(`/${workspaceSlug}/links/${link.id}/edit`),
                },
                {
                  label: t("actions.exportData"),
                  icon: <Export size={16} />,
                  onClick: () => { void handleExportAccessData(link); },
                  disabled: busy,
                },
                {
                  label: downloadOn
                    ? t("actions.disallowDownload")
                    : t("actions.allowDownload"),
                  icon: <DownloadSimple size={16} />,
                  onClick: () => { void handleToggleDownload(link); },
                  disabled: busy,
                },
                {
                  label: tc("delete"),
                  icon: <Trash size={16} />,
                  onClick: () => setLinkToDelete(link),
                  destructive: true,
                  disabled: busy,
                },
              ]}
            />
          );
        },
      },
    ],
    [
      busyLinkId,
      handleExportAccessData,
      handleToggleDownload,
      navigate,
      workspaceSlug,
      t,
      tc,
    ],
  );

  const table = useReactTable({
    data,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    // Only Create Time is sortable; keep toggling between asc/desc (default desc).
    enableSorting: false,
    enableSortingRemoval: false,
  });

  // Inbox scopes to the full share surface, not the current search/time slice.
  const linkIds = useMemo(() => allLinks.map((link) => link.id), [allLinks]);

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

  // Always constrain the inbox to links visible in this table (document surface).
  // Combined with API scope=document, this prevents deal-room applicant PII leakage.
  const accessInbox = <ShareAccessRequestsPanel linkIds={linkIds} />;

  if (data.length === 0) {
    if (listFiltersActive && allLinks.length > 0) {
      return (
        <div className="space-y-4">
          {accessInbox}
          <EmptyState
            icon={<MagnifyingGlass size={64} />}
            title={t("empty.noMatchesTitle")}
            description={t("empty.noMatchesDescription")}
            action={
              onClearListFilters
                ? {
                    label: t("empty.clearFilters"),
                    onClick: onClearListFilters,
                  }
                : undefined
            }
          />
        </div>
      );
    }

    const emptyTitle = isFiltered && documentTitle ? t("empty.filteredTitle", { title: documentTitle }) : t("empty.title");
    const emptyDescription = isFiltered && documentTitle ? t("empty.filteredDescription") : t("empty.description");
    // Parent Document Library Share tab already exposes Create Link.
    const emptyAction = embedded
      ? undefined
      : {
          label: t("empty.createLink"),
          onClick: () =>
            navigate(
              documentsCreateLinkPath(workspaceSlug!, {
                documentId: isFiltered ? documentId : undefined,
              }),
            ),
        };

    return (
      <div className="space-y-4">
        {accessInbox}
        <EmptyState
          icon={<LinkIcon size={64} />}
          title={emptyTitle}
          description={emptyDescription}
          action={emptyAction}
        />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {accessInbox}

      <div className="flex items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2">
          {!embedded && (
            <h2 className="text-h2">
              {isFiltered && documentTitle
                ? t("title.filteredLinks", { title: documentTitle })
                : t("title.allLinks")}
            </h2>
          )}
          {embedded && isFiltered && documentTitle ? (
            <p className="truncate text-sm text-muted-foreground">
              {t("title.filteredLinks", { title: documentTitle })}
            </p>
          ) : null}
          {isFiltered && (
            <Button
              variant="ghost"
              size="sm"
              className="gap-1 text-muted-foreground shrink-0"
              onClick={() => navigate(documentsSharePath(workspaceSlug!))}
            >
              <X size={14} />
              {t("table.clearFilter")}
            </Button>
          )}
        </div>
        <span className="text-caption whitespace-nowrap text-muted-foreground">
          {t("table.totalLinks", { count: data.length })}
        </span>
      </div>

      <div className="overflow-x-auto rounded-lg border border-border bg-card">
        <Table className="min-w-[72rem] table-fixed">
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => {
                  const sorted = header.column.getIsSorted();
                  const ariaSort =
                    sorted === "asc"
                      ? "ascending"
                      : sorted === "desc"
                        ? "descending"
                        : header.column.getCanSort()
                          ? "none"
                          : undefined;
                  return (
                    <TableHead
                      key={header.id}
                      className={linksColumnClass(header.column.id)}
                      aria-sort={ariaSort}
                    >
                      {header.isPlaceholder
                        ? null
                        : flexRender(header.column.columnDef.header, header.getContext())}
                    </TableHead>
                  );
                })}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows.map((row) => (
              <TableRow
                key={row.id}
                data-testid="links-table-row"
                data-link-id={row.original.id}
                className="cursor-pointer"
                role="link"
                tabIndex={0}
                onClick={() => openLink(row.original.id)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    openLink(row.original.id);
                  }
                }}
              >
                {row.getVisibleCells().map((cell) => (
                  <TableCell
                    key={cell.id}
                    className={cn(linksColumnClass(cell.column.id))}
                  >
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      <Dialog open={!!linkToDelete} onOpenChange={(open) => !open && setLinkToDelete(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("delete.title")}</DialogTitle>
            <DialogDescription className="break-words">
              {t("delete.description", { documentTitle: linkToDelete?.documentTitle })}
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
                  await api.deleteLink(linkToDelete.id);
                  toast.success(t("delete.success"));
                  setLinkToDelete(null);
                  void refetch();
                } catch (e) {
                  toast.error(apiErrorMessage(e, { fallback: "deleteFailed" }));
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
