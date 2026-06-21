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

  return (
    <div className={cn("space-y-1.5", className)}>
      <div className="flex items-center justify-between text-caption">
        <span className="text-muted-foreground">{label}</span>
        <span className="font-medium">
          {current} / {max}
        </span>
      </div>
      <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
        <div className={cn("h-full rounded-full transition-all", barColor)} style={{ width: `${percentage}%` }} />
      </div>
    </div>
  );
}
