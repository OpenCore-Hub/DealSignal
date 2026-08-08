import { useState } from "react";
import { cn } from "@/lib/utils";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "./EmptyState";
import { ChartLineUp } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";

interface TrendChartProps {
  title: string;
  data?: number[];
  /** Optional second series (e.g. unique visitors) drawn beside primary bars. */
  secondaryData?: number[];
  labels?: string[];
  className?: string;
  emptyTitle?: string;
  emptyDescription?: string;
  formatValue?: (value: number) => string;
  formatSecondaryValue?: (value: number) => string;
  primaryLegend?: string;
  secondaryLegend?: string;
}

function LegendSwatch({ variant }: { variant: "primary" | "secondary" }) {
  return (
    <span
      className={cn(
        "inline-block h-2.5 w-2.5 shrink-0 rounded-[3px] shadow-[inset_0_1px_0_rgba(255,255,255,0.35)]",
        variant === "primary"
          ? "bg-gradient-to-b from-rose-200 to-rose-300 dark:from-rose-300/80 dark:to-rose-400/70"
          : "bg-gradient-to-b from-emerald-200 to-emerald-300 dark:from-emerald-300/70 dark:to-emerald-400/60",
      )}
      aria-hidden
    />
  );
}

/** Show every day for short windows; thin out labels for 30/90d without losing ends. */
function shouldShowTickLabel(index: number, count: number): boolean {
  if (count <= 14) return true;
  if (index === 0 || index === count - 1) return true;
  const step = count <= 31 ? Math.ceil(count / 7) : Math.ceil(count / 6);
  return index % step === 0;
}

function barHeightPct(value: number, max: number): string {
  if (value <= 0) return "0%";
  return `${(value / max) * 100}%`;
}

export function TrendChart({
  title,
  data,
  secondaryData,
  labels,
  className,
  emptyTitle,
  emptyDescription,
  formatValue,
  formatSecondaryValue,
  primaryLegend,
  secondaryLegend,
}: TrendChartProps) {
  const { t } = useTranslation("common");
  const [hovered, setHovered] = useState<number | null>(null);

  if (!data || data.length === 0) {
    return (
      <Card className={cn(className)}>
        <CardHeader className="pb-2">
          <CardTitle className="text-h3">{title}</CardTitle>
        </CardHeader>
        <CardContent>
          <EmptyState
            icon={<ChartLineUp size={32} />}
            title={emptyTitle ?? t("trendEmptyTitle")}
            description={emptyDescription ?? t("empty.description")}
            size="large"
          />
        </CardContent>
      </Card>
    );
  }

  const hasSecondary =
    Array.isArray(secondaryData) &&
    secondaryData.length === data.length &&
    secondaryData.some((n) => n > 0);
  const max = Math.max(...data, ...(hasSecondary ? secondaryData! : []), 1);

  return (
    <Card className={cn("overflow-hidden", className)}>
      <CardHeader className="pb-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <CardTitle className="text-h3 tracking-tight">{title}</CardTitle>
          {(primaryLegend || (hasSecondary && secondaryLegend)) && (
            <div className="flex items-center gap-4 text-caption text-muted-foreground">
              {primaryLegend ? (
                <span className="inline-flex items-center gap-1.5">
                  <LegendSwatch variant="primary" />
                  {primaryLegend}
                </span>
              ) : null}
              {hasSecondary && secondaryLegend ? (
                <span className="inline-flex items-center gap-1.5">
                  <LegendSwatch variant="secondary" />
                  {secondaryLegend}
                </span>
              ) : null}
            </div>
          )}
        </div>
      </CardHeader>
      <CardContent className="pt-0">
        <div
          className={cn(
            "rounded-xl border border-border/70",
            "bg-gradient-to-b from-slate-50 via-slate-50/90 to-white",
            "dark:from-slate-900/80 dark:via-slate-900/55 dark:to-slate-950",
            "shadow-[inset_0_1px_0_rgba(255,255,255,0.65)] dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]",
            "px-2 pt-3 sm:px-3",
          )}
        >
          {/* Plot + per-day axis: each column owns its bars AND date so coordinates match. */}
          <div className="flex items-stretch gap-0.5 sm:gap-1">
            {data.map((h, i) => {
              const secondary = hasSecondary ? secondaryData![i] : 0;
              const active = hovered === i;
              const label = labels?.[i];
              const showLabel = shouldShowTickLabel(i, data.length);
              return (
                <div
                  key={i}
                  className="group relative flex min-w-0 flex-1 flex-col"
                  onMouseEnter={() => setHovered(i)}
                  onMouseLeave={() => setHovered(null)}
                >
                  <div className="relative h-44 w-full sm:h-48">
                    {/* Guides share the exact bar band so bars sit on the bottom rule. */}
                    <div
                      className="pointer-events-none absolute inset-0 flex flex-col justify-between"
                      aria-hidden
                    >
                      {[0, 1, 2].map((line) => (
                        <div
                          key={line}
                          className="border-t border-slate-200/70 dark:border-slate-700/55"
                        />
                      ))}
                      <div className="border-t border-slate-400/70 dark:border-slate-500/70" />
                    </div>

                    <div
                      className={cn(
                        "relative flex h-full w-full items-end justify-center",
                        hasSecondary ? "gap-px" : "",
                      )}
                    >
                      <div
                        className={cn(
                          "w-[45%] max-w-[14px] min-w-[3px] rounded-t-[3px] transition-opacity duration-150",
                          h > 0
                            ? cn(
                                "bg-gradient-to-b from-rose-200 to-rose-300",
                                "dark:from-rose-300/85 dark:to-rose-400/75",
                                "shadow-[0_1px_2px_rgba(251,113,133,0.25)]",
                                active ? "opacity-100" : "opacity-95",
                              )
                            : "h-0.5 bg-rose-200/70 dark:bg-rose-400/35",
                        )}
                        style={h > 0 ? { height: barHeightPct(h, max) } : undefined}
                        aria-hidden="true"
                      />
                      {hasSecondary ? (
                        <div
                          className={cn(
                            "w-[45%] max-w-[14px] min-w-[3px] rounded-t-[3px] transition-opacity duration-150",
                            secondary > 0
                              ? cn(
                                  "bg-gradient-to-b from-emerald-200 to-emerald-300",
                                  "dark:from-emerald-300/80 dark:to-emerald-400/65",
                                  "shadow-[0_1px_2px_rgba(52,211,153,0.22)]",
                                  active ? "opacity-100" : "opacity-90",
                                )
                              : "h-0.5 bg-emerald-200/60 dark:bg-emerald-400/30",
                          )}
                          style={
                            secondary > 0
                              ? { height: barHeightPct(secondary, max) }
                              : undefined
                          }
                          aria-hidden="true"
                        />
                      ) : null}
                    </div>

                    {active && (
                      <div
                        className={cn(
                          "absolute bottom-[calc(100%-0.25rem)] left-1/2 z-10 -translate-x-1/2",
                          "whitespace-nowrap rounded-lg border border-border/80",
                          "bg-slate-900 px-2.5 py-1.5 text-xs text-slate-50",
                          "shadow-[0_8px_24px_rgba(15,23,42,0.22)]",
                          "dark:bg-slate-100 dark:text-slate-900",
                        )}
                      >
                        {label ? (
                          <div className="mb-0.5 text-[10px] uppercase tracking-wide text-slate-400 dark:text-slate-500">
                            {label}
                          </div>
                        ) : null}
                        <div className="font-medium tabular-nums">
                          {formatValue ? formatValue(h) : h}
                        </div>
                        {hasSecondary ? (
                          <div className="mt-0.5 tabular-nums text-slate-300 dark:text-slate-600">
                            {formatSecondaryValue
                              ? formatSecondaryValue(secondary)
                              : secondary}
                          </div>
                        ) : null}
                      </div>
                    )}
                  </div>

                  {/* Tick under the same column — date ↔ bar correspondence. */}
                  <div
                    className={cn(
                      "flex h-8 items-start justify-center border-t border-transparent pt-1.5",
                      "text-[10px] leading-tight tracking-wide text-muted-foreground sm:text-caption",
                    )}
                    title={label}
                  >
                    {showLabel && label ? (
                      <span className="max-w-full truncate text-center">{label}</span>
                    ) : (
                      <span className="mt-0.5 block h-1 w-px bg-slate-300 dark:bg-slate-600" aria-hidden />
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
