import { cn } from "@/lib/utils";
import type { HeatLevel } from "@/types";

const styles: Record<HeatLevel, string> = {
  hot: "bg-hot-500/10 text-hot-500 border-hot-500/20",
  warm: "bg-warm-500/10 text-warm-500 border-warm-500/20",
  cold: "bg-cold-500/10 text-cold-500 border-cold-500/20",
};

const labels: Record<HeatLevel, string> = {
  hot: "高热度",
  warm: "中热度",
  cold: "低热度",
};

interface HeatBadgeProps {
  level: HeatLevel;
  className?: string;
}

export function HeatBadge({ level, className }: HeatBadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium",
        styles[level],
        className
      )}
    >
      {labels[level]}
    </span>
  );
}
