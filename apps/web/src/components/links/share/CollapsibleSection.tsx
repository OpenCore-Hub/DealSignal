import { CaretDown } from "@phosphor-icons/react";
import { cn } from "@/lib/utils";

interface CollapsibleSectionProps {
  title: string;
  badge?: React.ReactNode;
  open: boolean;
  onToggle: () => void;
  children: React.ReactNode;
  disabled?: boolean;
}

export function CollapsibleSection({
  title,
  badge,
  open,
  onToggle,
  children,
  disabled,
}: CollapsibleSectionProps) {
  return (
    <div className={cn("shrink-0", disabled && "opacity-50")}>
      <button
        type="button"
        onClick={onToggle}
        disabled={disabled}
        className="flex w-full items-center justify-between py-3 text-sm font-medium disabled:cursor-not-allowed"
      >
        <span className="flex items-center gap-2">
          {title}
          {badge}
        </span>
        <CaretDown size={16} className={cn("transition-transform", open && "rotate-180")} />
      </button>
      {open && <div className="space-y-4 pb-2">{children}</div>}
    </div>
  );
}
