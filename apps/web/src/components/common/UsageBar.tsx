import { cn } from "@/lib/utils";
import { useTranslation } from "react-i18next";

interface UsageBarProps {
  label: string;
  current: number;
  max: number;
  unit?: string;
  /** Pre-formatted current value (e.g. "12.4 MB"). */
  formatCurrent?: string;
  /** Pre-formatted max value (e.g. "1 GB"). */
  formatMax?: string;
  /** When false, the metric is not on this plan (not the same as unlimited). */
  included?: boolean;
  /** `featured` is a large statement meter; `ledger` is a compact hairline row. */
  variant?: "default" | "featured" | "ledger";
  className?: string;
}

/** Percent of quota used, or null when the ratio cannot be computed. */
export function usagePercent(current: number, max: number): number | null {
  if (!Number.isFinite(current) || !Number.isFinite(max) || max <= 0) {
    return null;
  }
  return Math.min(100, Math.max(0, Math.round((current / max) * 100)));
}

function displayNumber(value: number): string {
  return Number.isFinite(value) ? String(value) : "—";
}

export function UsageBar({
  label,
  current,
  max,
  unit,
  formatCurrent,
  formatMax,
  included = true,
  variant = "default",
  className,
}: UsageBarProps) {
  const { t } = useTranslation("common");
  const percentage = included ? usagePercent(current, max) : null;
  const unlimited = included && Number.isFinite(max) && max <= 0;
  const statement = variant === "featured" || variant === "ledger";
  const barColor =
    percentage == null
      ? "bg-primary"
      : percentage >= 100
        ? "bg-error-500"
        : percentage >= 80
          ? "bg-warning-500"
          : "bg-primary";
  const statusText =
    percentage == null
      ? ""
      : percentage >= 100
        ? t("usageAtLimit")
        : percentage >= 80
          ? t("usageNearLimit")
          : "";
  const currentText =
    formatCurrent ??
    `${displayNumber(current)}${unit && Number.isFinite(current) ? ` ${unit}` : ""}`;
  const maxText = !included
    ? t("notIncluded")
    : unlimited
      ? t("unlimited")
      : (formatMax ??
        `${displayNumber(max)}${unit && Number.isFinite(max) ? ` ${unit}` : ""}`);

  const fillScale = unlimited ? 1 : (percentage ?? 0) / 100;

  return (
    <div
      className={cn(
        statement ? "space-y-2" : "space-y-1.5",
        variant === "featured" && "space-y-3",
        className,
      )}
      data-testid="usage-bar"
    >
      <div
        className={cn(
          "flex items-baseline justify-between gap-6",
          variant === "default" && "items-center text-caption",
        )}
      >
        <span
          className={cn(
            "text-muted-foreground",
            variant === "featured" &&
              "text-[10px] font-medium uppercase tracking-[0.18em] text-muted-foreground/80",
            variant === "ledger" && "text-[13px] text-muted-foreground",
          )}
        >
          {label}
        </span>
        <span
          className={cn(
            "shrink-0 font-medium tabular-nums",
            variant === "featured" &&
              "font-mono text-[1.375rem] font-normal leading-none tracking-tight",
            variant === "ledger" && "font-mono text-[13px] font-normal tracking-tight text-foreground",
          )}
        >
          {unlimited ? (
            <>
              {currentText} / {maxText}
              <span className="ml-1 text-muted-foreground">{t("usageUnlimitedHint")}</span>
            </>
          ) : !included ? (
            <>{maxText}</>
          ) : (
            <>
              {currentText} / {maxText} ({percentage ?? 0}%)
              {statusText ? <span className="ml-1 text-warning-500">{statusText}</span> : null}
            </>
          )}
        </span>
      </div>
      {unlimited ? (
        <div
          className={cn(
            "w-full overflow-hidden",
            statement ? "h-[3px] bg-foreground/[0.07]" : "h-2 rounded-full bg-primary/15",
          )}
          data-testid="usage-bar-unlimited"
          aria-hidden
        >
          <div
            className={cn(
              "h-full w-full origin-left",
              statement ? "bg-foreground/35" : "rounded-full bg-primary/35",
            )}
          />
        </div>
      ) : (
        <div
          className={cn(
            "w-full overflow-hidden",
            statement ? "h-[3px] bg-foreground/[0.07]" : "h-2 rounded-full bg-muted",
          )}
        >
          <div
            className={cn(
              "h-full w-full origin-left will-change-transform",
              statement
                ? cn(barColor, "transition-transform duration-700 ease-[cubic-bezier(0.32,0.72,0,1)]")
                : cn("rounded-full transition-colors", barColor),
            )}
            style={
              statement
                ? { transform: `scaleX(${fillScale})` }
                : { width: `${percentage ?? 0}%` }
            }
            data-testid="usage-bar-fill"
          />
        </div>
      )}
    </div>
  );
}
