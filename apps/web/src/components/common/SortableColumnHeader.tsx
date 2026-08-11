import type { Column } from "@tanstack/react-table";
import { CaretDown, CaretUp, CaretUpDown } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface SortableColumnHeaderProps<TData> {
  column: Column<TData, unknown>;
  label: string;
}

/** Shared sortable table header control; put aria-sort on the parent columnheader. */
export function SortableColumnHeader<TData>({
  column,
  label,
}: SortableColumnHeaderProps<TData>) {
  const sorted = column.getIsSorted();

  return (
    <Button
      type="button"
      variant="ghost"
      size="sm"
      className={cn(
        "-ml-2 h-8 gap-1 px-2",
        "text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground/70",
        "hover:bg-transparent hover:text-foreground",
      )}
      onClick={column.getToggleSortingHandler()}
    >
      {label}
      {sorted === "asc" ? (
        <CaretUp size={12} aria-hidden />
      ) : sorted === "desc" ? (
        <CaretDown size={12} aria-hidden />
      ) : (
        <CaretUpDown size={12} className="opacity-50" aria-hidden />
      )}
    </Button>
  );
}
