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
import { formatDate } from "@/lib/formatters";
import { EmptyState } from "@/components/common/EmptyState";
import type { AccessLog } from "@/types";

interface LinkAccessLogProps {
  logs: AccessLog[];
}

export function LinkAccessLog({ logs }: LinkAccessLogProps) {
  const columns: ColumnDef<AccessLog>[] = [
    {
      accessorKey: "timestamp",
      header: "时间",
      cell: ({ row }) => formatDate(row.original.timestamp),
    },
    {
      accessorKey: "visitorEmail",
      header: "访客",
      cell: ({ row }) => row.original.visitorEmail || "匿名",
    },
    {
      accessorKey: "pageNumber",
      header: "页面",
      cell: ({ row }) => row.original.pageNumber || "-",
    },
    {
      accessorKey: "durationSeconds",
      header: "停留",
      cell: ({ row }) => `${row.original.durationSeconds}s`,
    },
    {
      accessorKey: "device",
      header: "设备",
      cell: ({ row }) => row.original.device || "-",
    },
    {
      accessorKey: "location",
      header: "地点",
      cell: ({ row }) => row.original.location || "-",
    },
  ];

  const table = useReactTable({
    data: logs,
    columns,
    getCoreRowModel: getCoreRowModel(),
  });

  if (logs.length === 0) {
    return (
      <EmptyState
        icon={<ClockCounterClockwise size={48} />}
        title="暂无访问记录"
        description="该链接尚未被访问，分享后将在此显示访客行为。"
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
