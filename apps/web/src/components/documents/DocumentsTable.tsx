import { useEffect, useState } from "react";
import { useLocation, useNavigate, useParams, useSearchParams } from "react-router";
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  getFilteredRowModel,
  flexRender,
  type SortingState,
} from "@tanstack/react-table";
import { Link as LinkIcon, MagnifyingGlass, Plus, Download } from "@phosphor-icons/react";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { DocumentFilter } from "@/types";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { EmptyState } from "@/components/common/EmptyState";
import { cn } from "@/lib/utils";
import { SkeletonList } from "@/components/common/SkeletonLayout";
import { useAsyncData } from "@/hooks/useAsyncData";
import { api } from "@/lib/api";
import { LinksTable } from "@/components/links/LinksTable";
import {
  documentsCreateLinkPath,
  sanitizeDocumentsLibrarySearchParams,
} from "@/lib/documentsSharePath";
import { buildDocumentRows, useDocumentColumns } from "./DocumentsColumns";
import { AddToDealRoomDialog } from "./AddToDealRoomDialog";
import type { DocumentRow } from "./DocumentsColumns";

interface DocumentsTableProps {
  category?: string;
}

function filterFromTabParam(tab: string | null): DocumentFilter {
  if (tab === "shared" || tab === "archived") return tab;
  return "all";
}

export function DocumentsTable({ category }: DocumentsTableProps) {
  "use no memo";
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const { t } = useTranslation(["documents", "common", "links"]);

  const openDocument = (documentId: string) => {
    navigate(`/${workspaceSlug}/documents/${documentId}`, {
      state: {
        returnTo: location.pathname + location.search,
        returnLabel: t("documents:detail.back"),
      },
    });
  };
  const [sorting, setSorting] = useState<SortingState>([{ id: "totalViews", desc: true }]);
  const [globalFilter, setGlobalFilter] = useState("");
  const filter = category ? ("all" as DocumentFilter) : filterFromTabParam(searchParams.get("tab"));
  const shareDocumentId = searchParams.get("documentId") ?? undefined;
  const shareDocumentTitle = searchParams.get("documentTitle") ?? undefined;
  const showShareTab = !category && filter === "shared";

  const setFilter = (next: DocumentFilter) => {
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        if (next === "all") {
          params.delete("tab");
        } else {
          params.set("tab", next);
        }
        if (next !== "shared") {
          params.delete("documentId");
          params.delete("documentTitle");
        }
        return params;
      },
      { replace: true },
    );
    if (next !== "all") setGlobalFilter("");
  };

  // Drop unknown ?tab= values and share-only filters on non-Share tabs.
  useEffect(() => {
    if (category) return;
    const sanitized = sanitizeDocumentsLibrarySearchParams(searchParams);
    if (!sanitized) return;
    setSearchParams(sanitized, { replace: true });
  }, [category, searchParams, setSearchParams]);

  const [docToAddToRoom, setDocToAddToRoom] = useState<DocumentRow | null>(null);
  const [docToDelete, setDocToDelete] = useState<DocumentRow | null>(null);
  const [deleteImpact, setDeleteImpact] = useState<{
    activeLinkCount: number;
    dealRoomCount: number;
  } | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const filters: DocumentFilter[] = ["all", "shared", "archived"];

  useEffect(() => {
    if (!docToDelete) {
      setDeleteImpact(null);
      return;
    }
    let cancelled = false;
    setDeleteImpact(null);
    void api
      .getDocumentDeleteImpact(docToDelete.id)
      .then((impact) => {
        if (!cancelled) {
          setDeleteImpact({
            activeLinkCount: impact.active_link_count,
            dealRoomCount: impact.deal_room_count,
          });
        }
      })
      .catch(() => {
        // Fall back to row-local link count when impact endpoint is unavailable.
        if (!cancelled) {
          setDeleteImpact({
            activeLinkCount: docToDelete.links.length,
            dealRoomCount: 0,
          });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [docToDelete]);

  const {
    data,
    loading,
    error,
    refetch,
  } = useAsyncData(async () => {
    if (showShareTab) return [] as DocumentRow[];
    const [docsRes, linksRes] = await Promise.all([
      api.getDocuments(filter, category, { excludeDealRoom: true }),
      api.getLinks(),
    ]);
    return buildDocumentRows(docsRes.data, linksRes.data);
  }, [filter, category, showShareTab]);

  // Poll for status updates while any document is still being processed.
  useEffect(() => {
    const hasProcessing = data?.some((row) => row.status === "processing" || row.status === "uploading");
    if (!hasProcessing) return;

    const interval = setInterval(() => {
      refetch();
    }, 3000);
    return () => clearInterval(interval);
  }, [data, refetch]);

  // Refresh immediately after an upload finishes (from dialog or upload page).
  useEffect(() => {
    const handleUploaded = () => refetch();
    window.addEventListener("documents:uploaded", handleUploaded);
    return () => window.removeEventListener("documents:uploaded", handleUploaded);
  }, [refetch]);

  const columns = useDocumentColumns({
    workspaceSlug,
    navigate,
    refetch,
    onAddToDealRoom: setDocToAddToRoom,
    onDelete: setDocToDelete,
    returnTo: location.pathname + location.search,
    returnLabel: t("documents:detail.back"),
  });

  // eslint-disable-next-line react-hooks/incompatible-library
  const table = useReactTable({
    data: data ?? [],
    columns,
    state: { sorting, globalFilter },
    onSortingChange: setSorting,
    onGlobalFilterChange: setGlobalFilter,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
  });

  const filterTabs =
    category !== "agreement" ? (
      <div className="mb-10 flex justify-center sm:mb-12">
        <Tabs
          value={filter}
          onValueChange={(value) => setFilter(value as DocumentFilter)}
          className="min-w-0"
        >
          <TabsList variant="line" className="justify-center gap-12 overflow-x-auto">
            {filters.map((f) => (
              <TabsTrigger
                key={f}
                value={f}
                className={cn(
                  "flex-none px-6 text-[15px] tracking-wide",
                  "transition-[color,transform] duration-300 ease-out",
                  "hover:-translate-y-px hover:text-foreground",
                  "data-active:font-semibold",
                  "after:h-0.5 after:origin-center after:scale-x-0 after:rounded-full",
                  "after:transition-[transform,opacity] after:duration-300 after:ease-out",
                  "data-active:after:scale-x-100 data-active:after:opacity-100"
                )}
              >
                {t(`documents:filters.${f}`)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      </div>
    ) : null;

  if (showShareTab) {
    return (
      <div>
        {filterTabs}
        <div className="space-y-4">
          <div className="flex flex-wrap items-center justify-end gap-3">
            <Button
              onClick={() =>
                navigate(
                  documentsCreateLinkPath(workspaceSlug!, {
                    documentId: shareDocumentId,
                  }),
                )
              }
              className="gap-1.5 shrink-0"
            >
              <Plus size={16} weight="bold" />
              {t("links:page.createLink")}
            </Button>
          </div>
          <LinksTable
            embedded
            documentId={shareDocumentId}
            documentTitle={shareDocumentTitle}
          />
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div>
        {filterTabs}
        <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
          <p className="text-body text-muted-foreground">{error}</p>
          <Button onClick={refetch}>{t("common:retry")}</Button>
        </div>
      </div>
    );
  }

  if (loading) {
    return (
      <div>
        {filterTabs}
        <SkeletonList rows={6} />
      </div>
    );
  }

  const hasDocuments = data && data.length > 0;
  /** Search + upload belong to the Documents tab only (not Shared / Archived). */
  const showDocumentsActions =
    hasDocuments && (category === "agreement" || filter === "all");

  return (
    <div>
      {filterTabs}

      <div className="space-y-4">
      {!data || data.length === 0 ? (
        filter !== "all" ? (
          <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
            <p className="text-body text-muted-foreground">{t("documents:table.emptyFilter")}</p>
            <Button variant="outline" onClick={() => setFilter("all")}>
              {t("documents:table.clearFilter")}
            </Button>
          </div>
        ) : (
          <EmptyState
            icon={<LinkIcon size={64} />}
            title={t("documents:table.emptyTitle")}
            description={t("documents:table.emptyDescription")}
            action={{
              label: t("documents:table.upload"),
              onClick: () =>
                navigate(
                  `/${workspaceSlug}/documents/upload` +
                    (category ? `?category=${encodeURIComponent(category)}` : "")
                ),
            }}
          />
        )
      ) : (
        <>
          <div className="flex flex-wrap items-center justify-end gap-3">
            {showDocumentsActions && (
              <div className="relative w-full min-w-[12rem] max-w-xs sm:w-56">
                <MagnifyingGlass
                  size={18}
                  className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"
                />
                <Input
                  placeholder={t("documents:table.searchPlaceholder")}
                  value={globalFilter}
                  onChange={(e) => setGlobalFilter(e.target.value)}
                  className="pl-9"
                />
              </div>
            )}
            {showDocumentsActions && (
              <>
                <Button
                  onClick={() =>
                    navigate(
                      `/${workspaceSlug}/documents/upload` +
                        (category ? `?category=${encodeURIComponent(category)}` : "")
                    )
                  }
                  className="gap-1.5 shrink-0"
                >
                  <Plus size={16} weight="bold" />
                  {t("documents:table.upload")}
                </Button>
                {category === "agreement" && (
                  <DropdownMenu>
                    <DropdownMenuTrigger
                      render={
                        <Button variant="outline" className="gap-1.5">
                          <Download size={16} weight="bold" />
                          {t("documents:table.downloadTemplate")}
                        </Button>
                      }
                    />
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem onClick={() => window.open("/templates/nda-cn.txt", "_blank")}>
                        {t("documents:table.templates.ndaCN")}
                      </DropdownMenuItem>
                      <DropdownMenuItem onClick={() => window.open("/templates/nda-en.txt", "_blank")}>
                        {t("documents:table.templates.ndaEN")}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                )}
              </>
            )}
          </div>

          <div className="rounded-lg border border-border bg-card">
            <Table>
              <TableHeader>
                {table.getHeaderGroups().map((headerGroup) => (
                  <TableRow key={headerGroup.id}>
                    {headerGroup.headers.map((header) => (
                      <TableHead
                        key={header.id}
                        className={header.id === "actions" ? "w-[100px] text-left" : ""}
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
                      {t("documents:table.noMatches")}
                    </TableCell>
                  </TableRow>
                ) : (
                  table.getRowModel().rows.map((row) => (
                    <TableRow
                      key={row.id}
                      className="cursor-pointer"
                      role="link"
                      tabIndex={0}
                      onClick={() => openDocument(row.original.id)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter" || e.key === " ") {
                          e.preventDefault();
                          openDocument(row.original.id);
                        }
                      }}
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
        </>
      )}
      </div>

      {docToAddToRoom && (
        <AddToDealRoomDialog
          documentId={docToAddToRoom.id}
          documentTitle={docToAddToRoom.title}
          open={!!docToAddToRoom}
          onOpenChange={(open) => !open && setDocToAddToRoom(null)}
          onAdded={() => setDocToAddToRoom(null)}
        />
      )}

      <Dialog open={!!docToDelete} onOpenChange={(open) => !open && !isDeleting && setDocToDelete(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("documents:delete.title")}</DialogTitle>
            <DialogDescription className="space-y-2 break-words">
              <span className="block">
                {t("documents:delete.description", { name: docToDelete?.title ?? "" })}
              </span>
              {(deleteImpact?.activeLinkCount ?? 0) > 0 ? (
                <span className="block text-destructive">
                  {t("documents:delete.withLinks", {
                    count: deleteImpact?.activeLinkCount ?? 0,
                  })}
                </span>
              ) : null}
              {(deleteImpact?.dealRoomCount ?? 0) > 0 ? (
                <span className="block text-destructive">
                  {t("documents:delete.withDealRooms", {
                    count: deleteImpact?.dealRoomCount ?? 0,
                  })}
                </span>
              ) : null}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setDocToDelete(null)}
              disabled={isDeleting}
            >
              {t("common:cancel")}
            </Button>
            <Button
              variant="destructive"
              disabled={isDeleting}
              onClick={async () => {
                if (!docToDelete) return;
                setIsDeleting(true);
                try {
                  await api.deleteDocument(docToDelete.id);
                  toast.success(t("documents:delete.success"));
                  setDocToDelete(null);
                  void refetch();
                } catch (e) {
                  toast.error(
                    e instanceof Error ? e.message : t("documents:delete.failed"),
                  );
                } finally {
                  setIsDeleting(false);
                }
              }}
            >
              {isDeleting ? t("documents:delete.confirmLoading") : t("common:delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
