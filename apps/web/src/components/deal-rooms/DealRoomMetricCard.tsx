import type { ReactNode } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

export interface DealRoomMetricRow {
  label: string;
  value: number | string;
}

export interface DealRoomMetricCardProps {
  title: string;
  /** Status pill on the right of the title (e.g. active / ready). */
  status?: ReactNode;
  metrics: DealRoomMetricRow[];
  /** Left side of the footer (caption). */
  footerNote?: ReactNode;
  /** Right side of the footer (actions). */
  footerActions?: ReactNode;
  /** Expanded content below the footer, still inside the same card. */
  details?: ReactNode;
  /** Optional override for metric value text (e.g. muted copy rows). */
  metricValueClassName?: string;
  onClick?: () => void;
  className?: string;
  "data-testid"?: string;
}

/** Shared deal-room list card chrome: title · status · metric rows · footer · details. */
export function DealRoomMetricCard({
  title,
  status,
  metrics,
  footerNote,
  footerActions,
  details,
  metricValueClassName,
  onClick,
  className,
  "data-testid": testId,
}: DealRoomMetricCardProps) {
  const interactive = typeof onClick === "function";

  return (
    <Card
      role={interactive ? "button" : undefined}
      tabIndex={interactive ? 0 : undefined}
      className={cn(
        interactive &&
          "cursor-pointer transition-colors hover:bg-muted/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        className,
      )}
      data-testid={testId}
      onClick={onClick}
      onKeyDown={
        interactive
          ? (e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                onClick?.();
              }
            }
          : undefined
      }
    >
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between gap-3">
          <CardTitle className="text-h3 line-clamp-1">{title}</CardTitle>
          {status ? <div className="flex shrink-0 items-center gap-1.5">{status}</div> : null}
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          {metrics.map((row) => (
            <div key={row.label} className="flex items-center justify-between text-body">
              <span className="text-muted-foreground">{row.label}</span>
              <span
                className={cn("font-medium tabular-nums", metricValueClassName)}
              >
                {row.value}
              </span>
            </div>
          ))}
        </div>
        {footerNote || footerActions ? (
          <div className="flex flex-wrap items-center justify-between gap-2 border-t border-border pt-3">
            <div className="min-w-0 text-caption text-muted-foreground">{footerNote}</div>
            {footerActions ? (
              <div
                className="flex shrink-0 flex-wrap items-center justify-end gap-2"
                onClick={(e) => e.stopPropagation()}
                onKeyDown={(e) => e.stopPropagation()}
              >
                {footerActions}
              </div>
            ) : null}
          </div>
        ) : null}
        {details ? (
          <div
            className="border-t border-border pt-3"
            onClick={(e) => e.stopPropagation()}
            onKeyDown={(e) => e.stopPropagation()}
          >
            {details}
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}

export function MetricStatusDot({
  active,
  activeLabel,
  inactiveLabel,
}: {
  active: boolean;
  activeLabel: string;
  inactiveLabel: string;
}) {
  return (
    <>
      <span
        className={`h-2 w-2 rounded-full ${active ? "bg-emerald-500" : "bg-slate-400"}`}
        aria-hidden
      />
      <span
        className={`text-caption font-medium ${
          active ? "text-emerald-600" : "text-muted-foreground"
        }`}
      >
        {active ? activeLabel : inactiveLabel}
      </span>
    </>
  );
}
