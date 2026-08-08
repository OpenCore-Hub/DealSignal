import type { WatermarkInfo } from "./WatermarkOverlay";

const IP_PREFIX = "IP:";
/** World-unified watermark clock: always UTC, never local timezone. */
const UTC_STAMP_RE = /^(\d{4}-\d{2}-\d{2}) (\d{2}:\d{2}:\d{2}) UTC$/;
const RFC3339_UTC_RE = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z$/;

/**
 * Format a Date as world-unified UTC: `YYYY-MM-DD HH:mm:ss UTC`.
 * Matches backend `watermarkTextFor` (never uses the browser local zone).
 */
export function formatWatermarkTimestamp(date: Date = new Date()): string {
  const y = date.getUTCFullYear();
  const mo = String(date.getUTCMonth() + 1).padStart(2, "0");
  const d = String(date.getUTCDate()).padStart(2, "0");
  const h = String(date.getUTCHours()).padStart(2, "0");
  const mi = String(date.getUTCMinutes()).padStart(2, "0");
  const s = String(date.getUTCSeconds()).padStart(2, "0");
  return `${y}-${mo}-${d} ${h}:${mi}:${s} UTC`;
}

/** Normalize legacy RFC3339 `…Z` or already-UTC stamps into world-unified form. */
export function normalizeWatermarkTimestamp(raw: string | undefined | null): string | null {
  const text = raw?.trim();
  if (!text) return null;
  if (UTC_STAMP_RE.test(text)) return text;
  if (RFC3339_UTC_RE.test(text)) {
    const ms = Date.parse(text);
    if (!Number.isNaN(ms)) return formatWatermarkTimestamp(new Date(ms));
  }
  return null;
}

/**
 * Parse server `email | <UTC stamp> | IP:hash` watermark payloads.
 * Returns null when the string is not in the expected dynamic format.
 */
export function parseWatermarkText(
  text: string | undefined | null,
): Pick<WatermarkInfo, "email" | "ip" | "viewedAt"> | null {
  const raw = text?.trim();
  if (!raw) return null;
  const parts = raw.split(" | ");
  if (parts.length !== 3) return null;
  const [email, stampRaw, ipPart] = parts;
  if (!email || !ipPart.startsWith(IP_PREFIX)) return null;
  const ip = ipPart.slice(IP_PREFIX.length).trim();
  if (!ip) return null;
  const viewedAt = normalizeWatermarkTimestamp(stampRaw);
  if (!viewedAt) return null;
  return { email, ip, viewedAt };
}

export type BuildDynamicWatermarkOptions = {
  /** Used when no identity is available (static fallback; never invents an email). */
  fallback: string;
  /** Localized IP label when identity is known but IP is absent (owner /viewer). */
  previewIp: string;
};

/**
 * Compose the same dynamic watermark visitors see on share links:
 * `email | YYYY-MM-DD HH:mm:ss UTC | IP:<hash|preview>`.
 *
 * Prefers the server-issued UTC stamp when present (world clock); otherwise
 * formats the session `now` in UTC. Never uses the browser local timezone.
 * Never forges a fake identity (e.g. "Owner preview") when email is missing.
 */
export function buildDynamicWatermarkText(
  info: WatermarkInfo | undefined,
  now: Date,
  opts: BuildDynamicWatermarkOptions,
): string {
  const parsed = parseWatermarkText(info?.watermarkText);
  const email = (info?.email ?? parsed?.email)?.trim();
  const ip = (info?.ip ?? parsed?.ip)?.trim();
  const timestamp =
    normalizeWatermarkTimestamp(info?.viewedAt) ??
    parsed?.viewedAt ??
    formatWatermarkTimestamp(now);

  if (email) {
    const ipLabel = ip || opts.previewIp;
    return `${email} | ${timestamp} | ${IP_PREFIX}${ipLabel}`;
  }

  if (info?.watermarkText?.trim()) {
    // Non-standard payload: keep as-is rather than inventing structure.
    return info.watermarkText.trim();
  }

  return opts.fallback;
}
