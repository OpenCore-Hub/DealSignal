import i18next from "i18next";

export function formatFileSize(bytes: number, locale?: string): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const lng = locale || i18next.language || "zh-CN";
  const sizes = i18next.getResourceBundle(lng, "formatters")?.fileSize?.units || ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / k ** i).toFixed(1))} ${sizes[i]}`;
}

export function formatDate(date: string, locale?: string): string {
  return new Intl.DateTimeFormat(locale || i18next.language || "zh-CN", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(date));
}

export function formatDuration(seconds: number, locale?: string): string {
  if (!seconds || seconds <= 0) return "-";
  const lng = locale || i18next.language || "zh-CN";
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  if (m === 0) return i18next.t("formatters:duration.seconds", { count: s, lng });
  return i18next.t("formatters:duration.minutesAndSeconds", { m, s, lng });
}

export function formatRelativeTime(date?: string, locale?: string): string {
  if (!date) return "-";
  const lng = locale || i18next.language || "zh-CN";
  const now = new Date();
  const then = new Date(date);
  const diff = Math.floor((now.getTime() - then.getTime()) / 1000);
  if (diff < 60) return i18next.t("formatters:relativeTime.justNow", { lng });
  if (diff < 3600) return i18next.t("formatters:relativeTime.minutesAgo", { count: Math.floor(diff / 60), lng });
  if (diff < 86400) return i18next.t("formatters:relativeTime.hoursAgo", { count: Math.floor(diff / 3600), lng });
  if (diff < 604800) return i18next.t("formatters:relativeTime.daysAgo", { count: Math.floor(diff / 86400), lng });
  return formatDate(date, lng);
}

export function formatCompactNumber(value: number, locale?: string): string {
  return new Intl.NumberFormat(locale || i18next.language || "zh-CN", {
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(value);
}

/**
 * Format a count using compact notation while preserving the locale's plural suffix.
 * The count used for plural selection is the raw integer; the displayed number is
 * compact-formatted (e.g. 1.5K, 1.2万).
 */
export function formatCountWithPlural(
  t: (key: string, options?: { count?: number }) => string,
  key: string,
  count: number,
  locale?: string
): string {
  const formatted = formatCompactNumber(count, locale);
  return t(key, { count }).replace(String(count), formatted);
}

export function getInitials(name: string | null | undefined): string {
  if (!name) return "?";
  return name
    .split(" ")
    .map((n) => n[0])
    .join("")
    .toUpperCase()
    .slice(0, 2);
}
