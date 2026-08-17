/**
 * Session cookies are first-party only when /api is same-origin as the SPA.
 * Production nginx already proxies /api; Vite must do the same in `pnpm dev`
 * whenever VITE_API_BASE_URL points at a separate origin. Cross-origin fetch
 * to the API (localhost vs 127.0.0.1, or different ports treated as
 * cross-site with 127.0.0.1) drops SameSite=Lax cookies and 401s mutations.
 */

export function normalizeApiBaseUrl(raw: string | undefined): string {
  return raw?.replace(/\/+$/, "") ?? "";
}

export function isAbsoluteHttpUrl(value: string): boolean {
  return /^https?:\/\//i.test(value);
}

/** Browser fetch base. Empty string = same-origin `/api` (proxy or nginx). */
export function resolveBrowserApiBaseUrl(
  raw: string | undefined,
  env: { dev: boolean; vitest: boolean },
): string {
  const configured = normalizeApiBaseUrl(raw);
  if (!configured) return "";
  if (env.dev && !env.vitest && isAbsoluteHttpUrl(configured)) {
    return "";
  }
  return configured;
}

/** Vite `server.proxy['/api'].target` origin, or undefined when MSW/relative. */
export function resolveDevApiProxyTarget(raw: string | undefined): string | undefined {
  const configured = normalizeApiBaseUrl(raw);
  if (!isAbsoluteHttpUrl(configured)) return undefined;
  try {
    return new URL(configured).origin;
  } catch {
    return undefined;
  }
}
