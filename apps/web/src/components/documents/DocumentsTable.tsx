import { useState } from "react";
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
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
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

  const columns = useDocumentColumns({ workspaceSlug, navigate });

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
        <Button onClick={refetch}>重试</Button>
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
        <Button onClick={() => navigate(`/${workspaceSlug}/documents/upload`)} className="gap-1.5">
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
                  没有找到匹配的文档
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
