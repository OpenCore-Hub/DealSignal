import type { InsightsDailyVisit } from "@/lib/api";

export type InsightsDailyCsvHeaders = readonly [string, string, string];

function csvEscape(value: string): string {
  if (/[",\n\r]/.test(value)) {
    return `"${value.replace(/"/g, '""')}"`;
  }
  return value;
}

function dayKey(iso: string): string {
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return iso;
  return new Date(t).toISOString().slice(0, 10);
}

function visitsToCsv(visits: InsightsDailyVisit[], headers: InsightsDailyCsvHeaders): string {
  const lines = [headers.join(",")];
  for (const v of visits) {
    lines.push(
      [csvEscape(dayKey(v.date)), csvEscape(String(v.opens)), csvEscape(String(v.uniqueVisitors))].join(
        ",",
      ),
    );
  }
  return `${lines.join("\n")}\n`;
}

function downloadTextFile(filename: string, content: string, mime: string) {
  const blob = new Blob([content], { type: mime });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.rel = "noopener";
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}

/** Download the Insights daily series already loaded for the selected range. */
export function exportInsightsDailyVisitsCsv(
  visits: InsightsDailyVisit[],
  rangeDays: number,
  headers: InsightsDailyCsvHeaders,
  range?: { from?: string; to?: string },
): void {
  const days = Number.isFinite(rangeDays) && rangeDays > 0 ? Math.trunc(rangeDays) : 7;
  const from = range?.from?.trim();
  const to = range?.to?.trim();
  const filename =
    from && to ? `insights-daily-${from}_${to}.csv` : `insights-daily-${days}d.csv`;
  downloadTextFile(filename, visitsToCsv(visits, headers), "text/csv;charset=utf-8");
}
