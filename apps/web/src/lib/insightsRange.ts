import type { InsightsRangeDays } from "@/lib/api";

/** Mirrors apps/api/internal/analytics/insights_range.go limits. */
export const INSIGHTS_MAX_CUSTOM_DAYS = 90;

export type InsightsCustomRangeError = "incomplete" | "invalid" | "order" | "tooLong";

export type InsightsRangeSelection =
  | { kind: "preset"; days: InsightsRangeDays }
  | { kind: "custom"; from: string; to: string };

/** Preset chip is active only when custom draft/panel is closed. */
export function isInsightsPresetActive(
  range: InsightsRangeSelection,
  customOpen: boolean,
  days: InsightsRangeDays,
): boolean {
  return !customOpen && range.kind === "preset" && range.days === days;
}

/** Custom chip is active while drafting or after a custom range is applied. */
export function isInsightsCustomActive(
  range: InsightsRangeSelection,
  customOpen: boolean,
): boolean {
  return customOpen || range.kind === "custom";
}

function parseUtcDay(isoDate: string): Date | null {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(isoDate)) return null;
  const [y, m, d] = isoDate.split("-").map(Number);
  const dt = new Date(Date.UTC(y, m - 1, d));
  if (
    dt.getUTCFullYear() !== y ||
    dt.getUTCMonth() !== m - 1 ||
    dt.getUTCDate() !== d
  ) {
    return null;
  }
  return dt;
}

export function utcTodayISO(now = new Date()): string {
  return new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()))
    .toISOString()
    .slice(0, 10);
}

/** Inclusive UTC calendar-day count between from/to, or null if invalid. */
export function inclusiveUtcDayCount(from: string, to: string): number | null {
  const a = parseUtcDay(from);
  const b = parseUtcDay(to);
  if (!a || !b) return null;
  const ms = b.getTime() - a.getTime();
  if (ms < 0) return null;
  return Math.floor(ms / 86_400_000) + 1;
}

export function validateInsightsCustomRange(
  from: string,
  to: string,
): InsightsCustomRangeError | null {
  const fromTrim = from.trim();
  const toTrim = to.trim();
  if (!fromTrim || !toTrim) return "incomplete";
  const days = inclusiveUtcDayCount(fromTrim, toTrim);
  if (days == null) {
    if (parseUtcDay(fromTrim) && parseUtcDay(toTrim)) return "order";
    return "invalid";
  }
  if (days > INSIGHTS_MAX_CUSTOM_DAYS) return "tooLong";
  return null;
}
