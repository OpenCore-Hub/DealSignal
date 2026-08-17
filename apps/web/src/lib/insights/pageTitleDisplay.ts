/**
 * Keep in lockstep with apps/api/internal/analytics/page_title.go.
 * Display-only: heat matching still uses the stored page title.
 */
const TRUNCATED_JSON_KEY = /^\w{1,40}":/;

export function displayablePageTitle(title: string | undefined | null): string {
  const t = title?.trim() ?? "";
  if (!t || looksLikeStructuredPageDump(t)) {
    return "";
  }
  return t;
}

function looksLikeStructuredPageDump(title: string): boolean {
  const t = title.trim();
  if (!t) {
    return false;
  }
  if (t.startsWith("{") || t.startsWith("[")) {
    return true;
  }
  if (TRUNCATED_JSON_KEY.test(t)) {
    return true;
  }
  const keys = t.match(/"[^"]{1,80}"\s*:/g) ?? [];
  if (keys.length >= 2) {
    return true;
  }
  return keys.length >= 1 && /[{}\[\]]/.test(t);
}
