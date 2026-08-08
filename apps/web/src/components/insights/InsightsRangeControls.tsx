import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { InsightsOverviewParams, InsightsRangeDays } from "@/lib/api";
import {
  INSIGHTS_MAX_CUSTOM_DAYS,
  isInsightsCustomActive,
  isInsightsPresetActive,
  utcTodayISO,
  validateInsightsCustomRange,
  type InsightsCustomRangeError,
  type InsightsRangeSelection,
} from "@/lib/insightsRange";
import { cn } from "@/lib/utils";

export type { InsightsRangeSelection };

const RANGE_OPTIONS: InsightsRangeDays[] = [7, 30, 90];

function defaultDraftFrom(days: InsightsRangeDays): string {
  const end = utcTodayISO();
  const start = new Date(`${end}T00:00:00Z`);
  start.setUTCDate(start.getUTCDate() - (days - 1));
  return start.toISOString().slice(0, 10);
}

function customRangeErrorMessage(
  code: InsightsCustomRangeError,
  t: (key: string, opts?: Record<string, unknown>) => string,
): string {
  switch (code) {
    case "incomplete":
      return t("overview.rangeErrorIncomplete");
    case "invalid":
      return t("overview.rangeErrorInvalid");
    case "order":
      return t("overview.rangeErrorOrder");
    case "tooLong":
      return t("overview.rangeErrorTooLong", { max: INSIGHTS_MAX_CUSTOM_DAYS });
  }
}

/** Convert selection into API query params for Insights range endpoints. */
export function insightsRangeToParams(range: InsightsRangeSelection): InsightsOverviewParams {
  if (range.kind === "custom") {
    return { from: range.from, to: range.to };
  }
  return { days: range.days };
}

export function useInsightsRange(initialDays: InsightsRangeDays = 7) {
  const { t } = useTranslation("insights");
  const [range, setRange] = useState<InsightsRangeSelection>({
    kind: "preset",
    days: initialDays,
  });
  const [customOpen, setCustomOpen] = useState(false);
  const [draftFrom, setDraftFrom] = useState(() => defaultDraftFrom(initialDays));
  const [draftTo, setDraftTo] = useState(() => utcTodayISO());
  const [rangeError, setRangeError] = useState<string | null>(null);

  const selectPreset = (days: InsightsRangeDays) => {
    setRangeError(null);
    setCustomOpen(false);
    setRange({ kind: "preset", days });
  };

  const openCustom = () => {
    setRangeError(null);
    // Seed drafts from the active preset window so the panel matches what the
    // user was looking at before switching to custom.
    if (range.kind === "preset") {
      setDraftFrom(defaultDraftFrom(range.days));
      setDraftTo(utcTodayISO());
    }
    setCustomOpen(true);
  };

  const applyCustom = (): boolean => {
    const code = validateInsightsCustomRange(draftFrom, draftTo);
    if (code) {
      setRangeError(customRangeErrorMessage(code, t));
      return false;
    }
    setRangeError(null);
    setCustomOpen(true);
    setRange({ kind: "custom", from: draftFrom, to: draftTo });
    return true;
  };

  const apiParams = useMemo(() => insightsRangeToParams(range), [range]);

  return {
    range,
    customOpen,
    draftFrom,
    draftTo,
    rangeError,
    setDraftFrom,
    setDraftTo,
    selectPreset,
    openCustom,
    applyCustom,
    apiParams,
  };
}

type InsightsRangeControlsProps = {
  range: InsightsRangeSelection;
  customOpen: boolean;
  draftFrom: string;
  draftTo: string;
  rangeError: string | null;
  onSelectPreset: (days: InsightsRangeDays) => void;
  onOpenCustom: () => void;
  onDraftFromChange: (value: string) => void;
  onDraftToChange: (value: string) => void;
  onApplyCustom: () => void;
  /** Optional visual variant — overview uses Button pills; reports use denser chips. */
  variant?: "buttons" | "chips";
  className?: string;
};

export function InsightsRangeControls({
  range,
  customOpen,
  draftFrom,
  draftTo,
  rangeError,
  onSelectPreset,
  onOpenCustom,
  onDraftFromChange,
  onDraftToChange,
  onApplyCustom,
  variant = "buttons",
  className,
}: InsightsRangeControlsProps) {
  const { t } = useTranslation("insights");
  const showCustom = isInsightsCustomActive(range, customOpen);
  const customActive = showCustom;
  // Active chip: light pink (not black foreground) for clearer selected state.
  const chipActiveClass = "bg-rose-100 text-rose-800";
  const chipIdleClass = "text-muted-foreground hover:bg-rose-50 hover:text-rose-700";
  const applyClass =
    "h-9 shrink-0 border-rose-200 bg-rose-100 text-rose-800 hover:bg-rose-200 hover:text-rose-900";

  return (
    <div className={cn("flex flex-col gap-3", className)}>
      <div
        className="inline-flex w-fit max-w-full flex-wrap rounded-md border border-border p-0.5"
        role="group"
        aria-label={t("overview.rangeLabel")}
      >
        {RANGE_OPTIONS.map((days) => {
          const active = isInsightsPresetActive(range, customOpen, days);
          if (variant === "chips") {
            return (
              <button
                key={days}
                type="button"
                onClick={() => onSelectPreset(days)}
                aria-pressed={active}
                className={cn(
                  "rounded px-3 py-1.5 text-sm font-medium transition-colors",
                  active ? chipActiveClass : chipIdleClass,
                )}
              >
                {t("overview.rangeDays", { days })}
              </button>
            );
          }
          return (
            <Button
              key={days}
              type="button"
              size="sm"
              variant={active ? "secondary" : "ghost"}
              className={cn(
                "min-w-14",
                active && "pointer-events-none bg-rose-100 text-rose-800 hover:bg-rose-100 hover:text-rose-800",
              )}
              aria-pressed={active}
              onClick={() => onSelectPreset(days)}
            >
              {t("overview.rangeDays", { days })}
            </Button>
          );
        })}
        {variant === "chips" ? (
          <button
            type="button"
            onClick={onOpenCustom}
            aria-pressed={customActive}
            className={cn(
              "rounded px-3 py-1.5 text-sm font-medium transition-colors",
              customActive ? chipActiveClass : chipIdleClass,
            )}
          >
            {t("overview.rangeCustom")}
          </button>
        ) : (
          <Button
            type="button"
            size="sm"
            variant={customActive ? "secondary" : "ghost"}
            className={cn(
              customActive && "bg-rose-100 text-rose-800 hover:bg-rose-100 hover:text-rose-800",
            )}
            aria-pressed={customActive}
            onClick={onOpenCustom}
          >
            {t("overview.rangeCustom")}
          </Button>
        )}
      </div>
      {showCustom ? (
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-[minmax(0,11rem)_minmax(0,11rem)_auto] sm:items-end sm:gap-3">
          <div className="flex min-w-0 flex-col gap-1">
            <Label htmlFor="insights-range-from">{t("overview.rangeFrom")}</Label>
            <Input
              id="insights-range-from"
              type="date"
              value={draftFrom}
              max={draftTo || utcTodayISO()}
              onChange={(e) => onDraftFromChange(e.target.value)}
              className="h-9 w-full"
            />
          </div>
          <div className="flex min-w-0 flex-col gap-1">
            <Label htmlFor="insights-range-to">{t("overview.rangeTo")}</Label>
            <Input
              id="insights-range-to"
              type="date"
              value={draftTo}
              max={utcTodayISO()}
              min={draftFrom || undefined}
              onChange={(e) => onDraftToChange(e.target.value)}
              className="h-9 w-full"
            />
          </div>
          <div className="flex flex-col gap-1">
            {/* Spacer matches Label height so Apply aligns with the date inputs. */}
            <span className="text-sm leading-none opacity-0 select-none" aria-hidden>
              {t("overview.rangeApply")}
            </span>
            <Button
              type="button"
              variant="outline"
              className={applyClass}
              onClick={onApplyCustom}
            >
              {t("overview.rangeApply")}
            </Button>
          </div>
        </div>
      ) : null}
      {rangeError ? (
        <p className="text-caption text-destructive" role="alert">
          {rangeError}
        </p>
      ) : null}
    </div>
  );
}
