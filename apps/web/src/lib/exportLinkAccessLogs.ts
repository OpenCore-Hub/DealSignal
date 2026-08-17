import { api } from "@/lib/api";
import type { AccessLog } from "@/types";

const PAGE_SIZE = 100;
const MAX_ROWS = 10_000;

export type AccessLogCsvHeaders = readonly [
  string,
  string,
  string,
  string,
  string,
  string,
  string,
  string,
];

function csvEscape(value: string): string {
  if (/[",\n\r]/.test(value)) {
    return `"${value.replace(/"/g, '""')}"`;
  }
  return value;
}

function logsToCsv(logs: AccessLog[], headers: AccessLogCsvHeaders): string {
  const lines = [headers.join(",")];
  for (const log of logs) {
    lines.push(
      [
        csvEscape(log.timestamp ?? ""),
        csvEscape(log.visitorEmail ?? ""),
        csvEscape(log.visitorName ?? ""),
        csvEscape(log.documentId ?? ""),
        csvEscape(log.pageNumber != null ? String(log.pageNumber) : ""),
        csvEscape(String(log.durationSeconds ?? 0)),
        csvEscape(log.device ?? ""),
        csvEscape(log.location ?? ""),
      ].join(","),
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

/** Fetch all access logs for a link and download as CSV with localized headers. */
export async function exportLinkAccessLogsCsv(
  linkId: string,
  filenameBase: string,
  headers: AccessLogCsvHeaders,
): Promise<number> {
  const rows: AccessLog[] = [];
  let offset = 0;

  for (;;) {
    const res = await api.getAccessLogs(linkId, { limit: PAGE_SIZE, offset });
    const batch = res.data ?? [];
    rows.push(...batch);
    if (rows.length >= MAX_ROWS) {
      rows.length = MAX_ROWS;
      break;
    }
    if (!res.has_more || batch.length === 0) break;
    offset += batch.length;
  }

  const safeBase = filenameBase.replace(/[^\w.-]+/g, "_").slice(0, 80) || "link";
  downloadTextFile(
    `${safeBase}-access-logs.csv`,
    logsToCsv(rows, headers),
    "text/csv;charset=utf-8",
  );
  return rows.length;
}
