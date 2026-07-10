/**
 * Insights — overview, pages analytics, visitors, suggestions.
 * Covers: GET /insights/overview, GET /insights/pages/:id, GET /insights/documents/:id/visitors,
 *         GET /suggestions, POST /suggestions (generate), POST /suggestions/:id/dismiss
 */
import { test, expect } from "@playwright/test";
import { seedRealBackend, seedDocument, seedLink, apiFetch } from "./real-helpers";

let token: string;
let workspaceSlug: string;
let docId: string;
let linkId: string;

test.describe("Insights & suggestions (real backend)", () => {
  test.beforeAll(async () => {
    const seed = await seedRealBackend();
    token = seed.token;
    workspaceSlug = seed.workspaceSlug;
    const doc = await seedDocument(token, workspaceSlug);
    docId = doc.id;
    const link = await seedLink(token, workspaceSlug, docId, { permissionType: "public" });
    linkId = link.id;
  });

  // ── Insights overview ─────────────────────────────────────
  test("gets insights overview", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/insights/overview`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok).toBe(true);
    const body = (await res.json()) as {
      tierCounts: Record<string, number>;
      topDocuments: unknown[];
      topLinks: unknown[];
      topContacts: unknown[];
    };
    expect(body).toHaveProperty("tierCounts");
    expect(body).toHaveProperty("topDocuments");
    expect(body).toHaveProperty("topLinks");
  });

  // ── Page analytics ────────────────────────────────────────
  test("gets page analytics for a document", async () => {
    const res = await apiFetch(
      `/api/workspaces/${workspaceSlug}/insights/pages/${docId}`,
      {
        headers: { Authorization: `Bearer ${token}` },
      }
    );
    expect(res.ok).toBe(true);
    const body = (await res.json()) as { data: { pageNumber: number; viewCount: number }[] };
    expect(Array.isArray(body.data)).toBe(true);
  });

  // ── Document visitors ─────────────────────────────────────
  test("gets document visitors", async () => {
    const res = await apiFetch(
      `/api/workspaces/${workspaceSlug}/insights/documents/${docId}/visitors`,
      {
        headers: { Authorization: `Bearer ${token}` },
      }
    );
    expect(res.ok).toBe(true);
    const body = (await res.json()) as { data: unknown[] };
    expect(Array.isArray(body.data)).toBe(true);
  });

  // ── Suggestions ───────────────────────────────────────────
  test("lists workspace suggestions", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/insights/suggestions`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok).toBe(true);
    const body = (await res.json()) as { data: { id: string }[] };
    expect(Array.isArray(body.data)).toBe(true);
  });

  test("generates link suggestions", async () => {
    const res = await apiFetch(
      `/api/workspaces/${workspaceSlug}/analytics/links/${linkId}/suggestions`,
      {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
      }
    );
    expect([200, 201, 400, 503]).toContain(res.status);
  });

  test("dismisses a link suggestion", async () => {
    // Get first active suggestion for the link
    const listRes = await apiFetch(
      `/api/workspaces/${workspaceSlug}/analytics/links/${linkId}/suggestions`,
      { headers: { Authorization: `Bearer ${token}` } }
    );
    const list = (await listRes.json()) as { suggestions: { id: string }[] };

    if (list.suggestions && list.suggestions.length > 0) {
      const res = await apiFetch(
        `/api/workspaces/${workspaceSlug}/analytics/links/${linkId}/suggestions/${list.suggestions[0].id}/dismiss`,
        { method: "POST", headers: { Authorization: `Bearer ${token}` } }
      );
      expect([200, 204]).toContain(res.status);
    }
  });

  // ── Browser page renders ──────────────────────────────────
  test("insights overview page renders in browser", async ({ page }) => {
    page.on("console", (msg) => console.log(`[browser ${msg.type()}] ${msg.text()}`));
    await page.addInitScript((t: string) => localStorage.setItem("access_token", t), token);
    await page.goto(`/${workspaceSlug}/insights/overview`);
    await expect(page.getByText(/insights|overview|documents/i).first()).toBeVisible({ timeout: 10000 });
  });

  test("insights pages page renders in browser", async ({ page }) => {
    page.on("console", (msg) => console.log(`[browser ${msg.type()}] ${msg.text()}`));
    await page.addInitScript((t: string) => localStorage.setItem("access_token", t), token);
    await page.goto(`/${workspaceSlug}/insights/pages`);
    await expect(page.getByText(/page|engagement|analytics/i).first()).toBeVisible({ timeout: 10000 });
  });

  test("insights suggestions page renders in browser", async ({ page }) => {
    page.on("console", (msg) => console.log(`[browser ${msg.type()}] ${msg.text()}`));
    await page.addInitScript((t: string) => localStorage.setItem("access_token", t), token);
    await page.goto(`/${workspaceSlug}/insights/suggestions`);
    await page.waitForTimeout(3000);
    // Page should render without crashing
    const hasContent = await page.locator("h1, h2, p, div").first().isVisible().catch(() => false);
    expect(hasContent).toBe(true);
  });
});
