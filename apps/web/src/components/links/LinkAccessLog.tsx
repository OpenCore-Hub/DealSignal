import {
  useReactTable,
  getCoreRowModel,
  flexRender,
  type ColumnDef,
} from "@tanstack/react-table";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { ClockCounterClockwise } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { formatDate } from "@/lib/formatters";
import { EmptyState } from "@/components/common/EmptyState";
import { accessLogDocumentTitle } from "@/lib/shareDocumentLabel";
import type { AccessLog, DocumentSummary } from "@/types";

interface LinkAccessLogProps {
  logs: AccessLog[];
  documents?: Pick<DocumentSummary, "id" | "title">[];
  primaryDocumentId?: string;
}

export function LinkAccessLog({ logs, documents = [], primaryDocumentId }: LinkAccessLogProps) {
  "use no memo";
  const { t } = useTranslation("links");
  const columns: ColumnDef<AccessLog>[] = [
    {
      accessorKey: "timestamp",
      header: t("accessLog.timestamp"),
      cell: ({ row }) => formatDate(row.original.timestamp),
    },
    {
      accessorKey: "visitorEmail",
      header: t("accessLog.visitor"),
      cell: ({ row }) => row.original.visitorEmail || t("accessLog.anonymous"),
    },
    {
      accessorKey: "pageNumber",
      header: t("accessLog.page"),
      cell: ({ row }) => {
        const page = row.original.pageNumber;
        if (!page) return "-";
        const title = accessLogDocumentTitle(row.original, documents, primaryDocumentId);
        if (title && documents.length > 1) {
          return t("detail.pageOnDocument", { title, page });
        }
        return page;
      },
    },
    {
      accessorKey: "durationSeconds",
      header: t("accessLog.duration"),
      cell: ({ row }) => `${row.original.durationSeconds}s`,
    },
    {
      accessorKey: "device",
      header: t("accessLog.device"),
      cell: ({ row }) => row.original.device || "-",
    },
    {
      accessorKey: "location",
      header: t("accessLog.location"),
      cell: ({ row }) => row.original.location || "-",
    },
  ];

  // eslint-disable-next-line react-hooks/incompatible-library
  const table = useReactTable({
    data: logs,
    columns,
    getCoreRowModel: getCoreRowModel(),
  });

  if (logs.length === 0) {
    return (
      <EmptyState
        icon={<ClockCounterClockwise size={48} />}
        title={t("accessLog.empty.title")}
        description={t("accessLog.empty.description")}
        size="large"
      />
    );
  }

  return (
    <div className="rounded-lg border border-border bg-card">
      <Table>
        <TableHeader>
          {table.getHeaderGroups().map((headerGroup) => (
            <TableRow key={headerGroup.id}>
              {headerGroup.headers.map((header) => (
                <TableHead key={header.id}>
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
            <TableRow key={row.id}>
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
  );
}
