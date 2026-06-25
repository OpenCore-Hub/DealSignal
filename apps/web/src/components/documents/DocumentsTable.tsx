import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router";
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  getFilteredRowModel,
  flexRender,
  type SortingState,
} from "@tanstack/react-table";
import { Link as LinkIcon, MagnifyingGlass, Plus } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
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
import { EmptyState } from "@/components/common/EmptyState";
import { SkeletonList } from "@/components/common/SkeletonLayout";
import { useAsyncData } from "@/hooks/useAsyncData";
import { api } from "@/lib/api";
import { buildDocumentRows, useDocumentColumns } from "./DocumentsColumns";

export function DocumentsTable() {
  "use no memo";
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const { t } = useTranslation(["documents", "common"]);
  const [sorting, setSorting] = useState<SortingState>([]);
  const [globalFilter, setGlobalFilter] = useState("");

  const {
    data,
    loading,
    error,
    refetch,
  } = useAsyncData(async () => {
    const [docsRes, linksRes] = await Promise.all([api.getDocuments(), api.getLinks()]);
    return buildDocumentRows(docsRes.data, linksRes.data);
  }, []);

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

  const columns = useDocumentColumns({ workspaceSlug, navigate });

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

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
        <p className="text-body text-muted-foreground">{error}</p>
        <Button onClick={refetch}>{t("common:retry")}</Button>
      </div>
    );
  }

  if (loading) {
    return <SkeletonList rows={6} />;
  }

  if (!data || data.length === 0) {
    return (
      <EmptyState
        icon={<LinkIcon size={64} />}
        title={t("documents:table.emptyTitle")}
        description={t("documents:table.emptyDescription")}
        action={{
          label: t("documents:table.upload"),
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
            placeholder={t("documents:table.searchPlaceholder")}
            value={globalFilter}
            onChange={(e) => setGlobalFilter(e.target.value)}
            className="pl-9"
          />
        </div>
        <Button onClick={() => navigate(`/${workspaceSlug}/documents/upload`)} className="gap-1.5">
          <Plus size={16} weight="bold" />
          {t("documents:table.upload")}
        </Button>
      </div>

      <p className="text-caption text-muted-foreground">
        {globalFilter
          ? t("documents:table.documentCountFiltered", {
              count: data.length,
              filtered: table.getRowModel().rows.length,
            })
          : t("documents:table.documentCount", { count: data.length })}
      </p>

      <div className="rounded-lg border border-border bg-card">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <TableHead
                    key={header.id}
                    className={header.id === "actions" ? "w-[100px] text-right" : ""}
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
                  onClick={() => navigate(`/${workspaceSlug}/documents/${row.original.id}`)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      navigate(`/${workspaceSlug}/documents/${row.original.id}`);
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
    </div>
  );
}
