/**
 * Auth edge cases — token refresh, logout, rate limiting, missing token, invalid credentials.
 * Covers: POST /auth/refresh, POST /auth/logout, edge error responses
 */
import { test, expect } from "@playwright/test";

const API_BASE = process.env.REAL_API_BASE_URL || "http://localhost:8080";

async function apiFetch(input: string, init?: RequestInit): Promise<Response> {
  return fetch(`${API_BASE}${input}`, {
    headers: { "Content-Type": "application/json", ...init?.headers },
    ...init,
  });
}

test.describe("Auth edge cases (real backend)", () => {
  // ── Token refresh ──────────────────────────────────────────
  test("refreshes an access token", async () => {
    const ts = Date.now();
    const email = `refresh-${ts}@example.com`;

    // Register
    const regRes = await apiFetch("/api/auth/register", {
      method: "POST",
      body: JSON.stringify({ email, password: "Password123!" }),
    });
    expect(regRes.ok).toBe(true);
    const { refresh_token: initialRefresh } = (await regRes.json()) as {
      access_token: string;
      refresh_token: string;
    };

    // Refresh
    const refreshRes = await apiFetch("/api/auth/refresh", {
      method: "POST",
      body: JSON.stringify({ refresh_token: initialRefresh }),
    });
    expect(refreshRes.ok).toBe(true);
    const refreshed = (await refreshRes.json()) as { access_token: string; refresh_token: string };
    expect(refreshed.access_token).toBeTruthy();
    expect(refreshed.refresh_token).toBeTruthy();

    // New token should be usable
    const wsRes = await apiFetch("/api/workspaces", {
      headers: { Authorization: `Bearer ${refreshed.access_token}` },
    });
    expect(wsRes.ok).toBe(true);
  });

  test("token refresh fails with invalid token", async () => {
    const res = await apiFetch("/api/auth/refresh", {
      method: "POST",
      body: JSON.stringify({ refresh_token: "invalid_token" }),
    });
    expect(res.status).toBe(401);
  });

  test("token refresh fails with missing token", async () => {
    const res = await apiFetch("/api/auth/refresh", {
      method: "POST",
      body: JSON.stringify({}),
    });
    expect(res.status).toBe(401);
  });

  // ── Logout ─────────────────────────────────────────────────
  test("logout invalidates refresh token", async () => {
    const ts = Date.now();
    const email = `logout-${ts}@example.com`;

    const regRes = await apiFetch("/api/auth/register", {
      method: "POST",
      body: JSON.stringify({ email, password: "Password123!" }),
    });
    const { access_token, refresh_token } = (await regRes.json()) as {
      access_token: string;
      refresh_token: string;
    };

    // Logout
    const logoutRes = await apiFetch("/api/auth/logout", {
      method: "POST",
      headers: { Authorization: `Bearer ${access_token}` },
      body: JSON.stringify({ refresh_token }),
    });
    expect(logoutRes.ok).toBe(true);

    // Refresh should now fail
    const refreshRes = await apiFetch("/api/auth/refresh", {
      method: "POST",
      body: JSON.stringify({ refresh_token }),
    });
    expect(refreshRes.status).toBe(401);
  });

  // ── Login validation ──────────────────────────────────────
  test("login fails with invalid credentials", async () => {
    const res = await apiFetch("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ email: "nonexistent@example.com", password: "wrong" }),
    });
    expect(res.status).toBe(401);
  });

  test("login fails with missing fields", async () => {
    const res = await apiFetch("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({}),
    });
    expect(res.status).toBe(401);
  });

  // ── Registration validation ───────────────────────────────
  test("register fails with weak password", async () => {
    const res = await apiFetch("/api/auth/register", {
      method: "POST",
      body: JSON.stringify({ email: `weak-${Date.now()}@example.com`, password: "short" }),
    });
    expect(res.status).toBe(400);
  });

  test("register fails with duplicate email", async () => {
    const ts = Date.now();
    const email = `dup-${ts}@example.com`;

    // First registration
    await apiFetch("/api/auth/register", {
      method: "POST",
      body: JSON.stringify({ email, password: "Password123!" }),
    });

    // Duplicate registration
    const res = await apiFetch("/api/auth/register", {
      method: "POST",
      body: JSON.stringify({ email, password: "Password123!" }),
    });
    expect(res.status).toBe(409);
  });

  // ── Unauthorized access ───────────────────────────────────
  test("protected endpoint returns 401 without token", async () => {
    // Use any authenticated workspace endpoint
    const res = await apiFetch("/api/workspaces", {
      headers: { Authorization: "Bearer invalid_token" },
    });
    expect(res.status).toBe(401);
  });

  test("protected endpoint returns 401 without auth header", async () => {
    const res = await apiFetch("/api/workspaces");
    expect(res.status).toBe(401);
  });

  // ── Verify email endpoints ────────────────────────────────
  test("verify email returns 404 for missing token", async () => {
    // Without token parameter, route won't match
    const res = await fetch(`${API_BASE}/api/auth/verify-email/`);
    expect([400, 404]).toContain(res.status);
  });
});
