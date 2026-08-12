/** Shared helpers for invite → login/register redirect flows. */

export function safeAuthRedirect(raw: string | null): string | null {
  return raw && /^\/[^/]/.test(raw) ? raw : null;
}

export function inviteEmailFromSearchParams(searchParams: URLSearchParams): string {
  const email = (searchParams.get("email") ?? "").trim();
  return email.includes("@") ? email : "";
}

export function isInviteAuthFlow(searchParams: URLSearchParams): boolean {
  return searchParams.get("invite") === "1" && Boolean(inviteEmailFromSearchParams(searchParams));
}

export function buildInviteAuthPath(
  mode: "login" | "register",
  opts: { redirect?: string | null; email?: string | null },
): string {
  const params = new URLSearchParams();
  const redirect = safeAuthRedirect(opts.redirect ?? null);
  if (redirect) params.set("redirect", redirect);
  const email = (opts.email ?? "").trim();
  if (email.includes("@")) {
    params.set("email", email);
    params.set("invite", "1");
  }
  const qs = params.toString();
  return qs ? `/${mode}?${qs}` : `/${mode}`;
}
