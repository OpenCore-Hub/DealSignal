const ACCOUNT_EMAIL_KEY = "ds.auth.accountEmail";

/** Persist login email for owner surfaces (e.g. /viewer watermark) across reloads. */
export function setCachedAccountEmail(email: string | undefined | null): void {
  const value = email?.trim();
  if (typeof sessionStorage === "undefined") return;
  if (!value) {
    sessionStorage.removeItem(ACCOUNT_EMAIL_KEY);
    return;
  }
  sessionStorage.setItem(ACCOUNT_EMAIL_KEY, value);
}

export function getCachedAccountEmail(): string | undefined {
  if (typeof sessionStorage === "undefined") return undefined;
  const value = sessionStorage.getItem(ACCOUNT_EMAIL_KEY)?.trim();
  return value || undefined;
}

export function clearCachedAccountEmail(): void {
  if (typeof sessionStorage === "undefined") return;
  sessionStorage.removeItem(ACCOUNT_EMAIL_KEY);
}
