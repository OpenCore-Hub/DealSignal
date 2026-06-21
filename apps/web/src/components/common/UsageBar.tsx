import { cn } from "@/lib/utils";

interface UsageBarProps {
  label: string;
  current: number;
  max: number;
  className?: string;
}

export function UsageBar({ label, current, max, className }: UsageBarProps) {
  const percentage = Math.min(100, Math.round((current / max) * 100));
  const barColor = percentage >= 100 ? "bg-error-500" : percentage >= 80 ? "bg-warning-500" : "bg-primary";
  const statusText = percentage >= 100 ? "已达上限" : percentage >= 80 ? "接近上限" : "";

  return (
    <div className={cn("space-y-1.5", className)}>
      <div className="flex items-center justify-between text-caption">
        <span className="text-muted-foreground">{label}</span>
        <span className="font-medium tabular-nums">
          {current} / {max} ({percentage}%){statusText && <span className="ml-1 text-warning-500">{statusText}</span>}
        </span>
      </div>
      <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
        <div className={cn("h-full rounded-full transition-colors", barColor)} style={{ width: `${percentage}%` }} />
      </div>
    </div>
  );
}
