/**
 * Integrations & Marketing — Slack/HubSpot connect/disconnect, marketing batch send.
 * Covers: GET/PUT integrations/settings, POST slack/hubspot connect/disconnect,
 *         GET sync-logs, POST marketing/send
 */
import { test, expect } from "@playwright/test";
import { seedRealBackend, apiFetch } from "./real-helpers";

let token: string;
let workspaceSlug: string;

test.describe("Integrations & marketing (real backend)", () => {
  test.beforeAll(async () => {
    const seed = await seedRealBackend();
    token = seed.token;
    workspaceSlug = seed.workspaceSlug;
  });

  // ── Integrations ────────────────────────────────────────────
  test("reads integration settings", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/integrations/settings`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok).toBe(true);
    const body = (await res.json()) as Record<string, unknown>;
    expect(body).toHaveProperty("slack");
    expect(body).toHaveProperty("hubspot");
  });

  test("updates integration settings", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/integrations/settings`, {
      method: "PUT",
      headers: { Authorization: `Bearer ${token}` },
      body: JSON.stringify({ slack: false, hubspot: false, zapier: false }),
    });
    expect(res.ok).toBe(true);
  });

  test("connects and disconnects Slack (API)", async () => {
    // Connect — returns OAuth URL (mock or real)
    const connRes = await apiFetch(`/api/workspaces/${workspaceSlug}/integrations/slack/connect`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
    });
    const ok = connRes.ok || connRes.status === 400;
    expect(ok).toBe(true);
    if (connRes.ok) {
      const body = (await connRes.json()) as { url?: string };
      if (body.url) expect(body.url).toContain("slack");
    }

    // Disconnect
    const discRes = await apiFetch(`/api/workspaces/${workspaceSlug}/integrations/slack/disconnect`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
    });
    expect([200, 400, 404]).toContain(discRes.status);
  });

  test("connects and disconnects HubSpot (API)", async () => {
    const connRes = await apiFetch(`/api/workspaces/${workspaceSlug}/integrations/hubspot/connect`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
    });
    const ok = connRes.ok || connRes.status === 400;
    expect(ok).toBe(true);
    if (connRes.ok) {
      const body = (await connRes.json()) as { url?: string };
      if (body.url) expect(body.url).toContain("hubspot");
    }

    const discRes = await apiFetch(`/api/workspaces/${workspaceSlug}/integrations/hubspot/disconnect`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
    });
    expect([200, 400, 404]).toContain(discRes.status);
  });

  test("reads sync logs", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/integrations/sync-logs`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok).toBe(true);
    const body = (await res.json()) as { data: unknown[] };
    expect(Array.isArray(body.data)).toBe(true);
  });

  test("triggers HubSpot sync", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/integrations/hubspot/sync`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
    });
    expect([200, 202, 400, 409]).toContain(res.status);
  });

  // ── Marketing ───────────────────────────────────────────────
  test("sends marketing batch email", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/marketing/send`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
      body: JSON.stringify({
        recipients: [`batch-${Date.now()}@example.com`],
        subject: "E2E Marketing Test",
        body: "This is an automated E2E test email.",
        headline: "Test Headline",
        cta_text: "Click Here",
        cta_url: "https://example.com",
        track_opens: true,
        track_clicks: true,
      }),
    });
    expect([200, 201, 202]).toContain(res.status);

    if (res.ok) {
      const body = (await res.json()) as { sent: number; failed: number };
      expect(typeof body.sent).toBe("number");
    }
  });
});
