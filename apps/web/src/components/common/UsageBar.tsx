import { cn } from "@/lib/utils";
import { useTranslation } from "react-i18next";

interface UsageBarProps {
  label: string;
  current: number;
  max: number;
  unit?: string;
  className?: string;
}

export function UsageBar({ label, current, max, unit, className }: UsageBarProps) {
  const { t } = useTranslation("common");
  const percentage = Math.min(100, Math.round((current / max) * 100));
  const barColor = percentage >= 100 ? "bg-error-500" : percentage >= 80 ? "bg-warning-500" : "bg-primary";
  const statusText = percentage >= 100 ? t("usageAtLimit") : percentage >= 80 ? t("usageNearLimit") : "";

  return (
    <div className={cn("space-y-1.5", className)}>
      <div className="flex items-center justify-between text-caption">
        <span className="text-muted-foreground">{label}</span>
        <span className="font-medium tabular-nums">
          {current}{unit ? ` ${unit}` : ""} / {max}{unit ? ` ${unit}` : ""} ({percentage}%){statusText && <span className="ml-1 text-warning-500">{statusText}</span>}
        </span>
      </div>
      <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
        <div className={cn("h-full rounded-full transition-colors", barColor)} style={{ width: `${percentage}%` }} />
      </div>
    </div>
  );
}
