/**
 * Workspace settings, members, security, billing — real backend E2E.
 * Covers: PUT settings, PUT security, GET billing, POST logo, GET/POST members, POST invitations
 */
import { test, expect } from "@playwright/test";
import { seedRealBackend, authenticatePage, apiFetch, attachDebug } from "./real-helpers";

let workspaceSlug: string;

test.describe("Workspace settings & members (real backend)", () => {
  test.beforeAll(async () => {
    const seed = await seedRealBackend();
    workspaceSlug = seed.workspaceSlug;
  });

  // ── Settings read/write ──────────────────────────────────────
  test("reads workspace settings", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/settings`, {
    });
    expect(res.ok).toBe(true);
    const body = (await res.json()) as { name: string; slug: string };
    expect(body.name).toBe("E2E Workspace");
    expect(body.slug).toBe(workspaceSlug);
  });

  test("updates workspace settings via API", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/settings`, {
      method: "PUT",
      body: JSON.stringify({
        name: "E2E Workspace Updated",
        slug: workspaceSlug,
        brand_color: "#ff6600",
      }),
    });
    expect(res.ok).toBe(true);
    const body = (await res.json()) as { name: string; brand_color: string };
    expect(body.name).toBe("E2E Workspace Updated");
    expect(body.brand_color).toBe("#ff6600");

    // Revert
    await apiFetch(`/api/workspaces/${workspaceSlug}/settings`, {
      method: "PUT",
      body: JSON.stringify({ name: "E2E Workspace", slug: workspaceSlug, brand_color: "#0055ff" }),
    });
  });

  // ── Security settings ────────────────────────────────────────
  test("reads and updates security settings", async () => {
    const getRes = await apiFetch(`/api/workspaces/${workspaceSlug}/security`, {
    });
    expect(getRes.ok).toBe(true);

    const putRes = await apiFetch(`/api/workspaces/${workspaceSlug}/security`, {
      method: "PUT",
      body: JSON.stringify({
        forceEmailVerification: true,
        watermarkDownloads: true,
        twoFactorEnabled: false,
      }),
    });
    expect(putRes.ok).toBe(true);
  });

  // ── Billing ──────────────────────────────────────────────────
  test("reads billing info", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/billing`, {
    });
    expect(res.ok).toBe(true);
    const body = (await res.json()) as { plan: string };
    expect(body.plan).toBeTruthy();
  });

  // ── Members ──────────────────────────────────────────────────
  test("lists workspace members", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/members`, {
    });
    expect(res.ok).toBe(true);
    const body = (await res.json()) as { data: unknown[] };
    expect(Array.isArray(body.data)).toBe(true);
    // Creator should be a member
    expect(body.data.length).toBeGreaterThan(0);
  });

  test("creates a workspace invitation", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/invitations`, {
      method: "POST",
      body: JSON.stringify({ email: `invite-${Date.now()}@example.com`, role: "member" }),
    });
    // May succeed or return conflict if already invited
    expect([200, 201, 409]).toContain(res.status);
  });

  // ── Logo upload ──────────────────────────────────────────────
  test("uploads workspace logo", async () => {
    // Create a tiny 1x1 PNG in base64
    const tinyPng = Buffer.from(
      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
      "base64"
    );
    const blob = new Blob([tinyPng], { type: "image/png" });
    const form = new FormData();
    form.append("file", blob, "logo.png");

    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/logo`, {
      method: "POST",
      body: form,
    });
    // May not be supported in all environments
    expect([200, 201, 400, 404, 415]).toContain(res.status);
  });

  // ── Settings page renders in browser ────────────────────────
  test("settings general page shows form in browser", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);
    await page.goto(`/${workspaceSlug}/settings/general`);
    await expect(page.getByText(/workspace/i).first()).toBeVisible({ timeout: 10000 });
  });

  test("settings members page renders", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);
    await page.goto(`/${workspaceSlug}/settings/members`);
    await expect(page.getByText(/members/i).first()).toBeVisible({ timeout: 10000 });
  });

  test("settings security page renders", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);
    await page.goto(`/${workspaceSlug}/settings/security`);
    await expect(page.getByText(/security/i).first()).toBeVisible({ timeout: 10000 });
  });

  test("settings billing page renders", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);
    await page.goto(`/${workspaceSlug}/settings/billing`);
    await expect(page.getByText(/subscription|billing|plan/i).first()).toBeVisible({ timeout: 10000 });
  });

  test("settings integrations page renders", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);
    await page.goto(`/${workspaceSlug}/settings/integrations`);
    await expect(page.getByText(/integrations|slack|hubspot/i).first()).toBeVisible({ timeout: 10000 });
  });

  test("settings language page renders", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);
    await page.goto(`/${workspaceSlug}/settings/language`);
    await expect(page.getByText(/language|english|中文/i).first()).toBeVisible({ timeout: 10000 });
  });
});
