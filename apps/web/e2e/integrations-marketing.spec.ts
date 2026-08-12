/**
 * Integrations & Marketing — Slack/HubSpot connect/disconnect, marketing batch send.
 * Covers: GET/PUT integrations/settings, POST slack/hubspot connect/disconnect,
 *         GET sync-logs, POST marketing/send
 */
import { test, expect } from "@playwright/test";
import { seedRealBackend, apiFetch } from "./real-helpers";

let workspaceSlug: string;

test.describe("Integrations & marketing (real backend)", () => {
  test.beforeAll(async () => {
    const seed = await seedRealBackend();
    workspaceSlug = seed.workspaceSlug;
  });

  // ── Integrations ────────────────────────────────────────────
  test("reads integration settings", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/integrations/settings`, {
    });
    expect(res.ok).toBe(true);
    const body = (await res.json()) as Record<string, unknown>;
    expect(body).toHaveProperty("slack_connected");
    expect(body).toHaveProperty("hubspot_connected");
  });

  test("updates integration settings", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/integrations/settings`, {
      method: "PUT",
      body: JSON.stringify({ email_enabled: true, daily_digest_enabled: false, key_page_slack_enabled: false }),
    });
    expect(res.ok).toBe(true);
  });

  test("connects and disconnects Slack (API)", async () => {
    // Connect — returns OAuth URL (mock or real)
    const connRes = await apiFetch(`/api/workspaces/${workspaceSlug}/integrations/slack/connect`, {
      method: "POST",
    });
    const ok = connRes.ok || [400, 422].includes(connRes.status);
    expect(ok).toBe(true);
    if (connRes.ok) {
      const body = (await connRes.json()) as { url?: string };
      if (body.url) expect(body.url).toContain("slack");
    }

    // Disconnect
    const discRes = await apiFetch(`/api/workspaces/${workspaceSlug}/integrations/slack/disconnect`, {
      method: "POST",
    });
    expect([200, 400, 404]).toContain(discRes.status);
  });

  test("reads and rejects insecure outbound webhook", async () => {
    const getRes = await apiFetch(`/api/workspaces/${workspaceSlug}/integrations/webhook`);
    expect(getRes.ok).toBe(true);
    const body = (await getRes.json()) as { configured?: boolean };
    expect(body).toHaveProperty("configured");

    const bad = await apiFetch(`/api/workspaces/${workspaceSlug}/integrations/webhook`, {
      method: "PUT",
      body: JSON.stringify({
        url: "http://example.com/hooks/catch/1",
        enabled: true,
        event_types: ["key_page"],
      }),
    });
    expect(bad.status).toBe(400);
  });

  test("connects and disconnects HubSpot (API)", async () => {
    const connRes = await apiFetch(`/api/workspaces/${workspaceSlug}/integrations/hubspot/connect`, {
      method: "POST",
    });
    const ok = connRes.ok || [400, 422].includes(connRes.status);
    expect(ok).toBe(true);
    if (connRes.ok) {
      const body = (await connRes.json()) as { url?: string };
      if (body.url) expect(body.url).toContain("hubspot");
    }

    const discRes = await apiFetch(`/api/workspaces/${workspaceSlug}/integrations/hubspot/disconnect`, {
      method: "POST",
    });
    expect([200, 400, 404]).toContain(discRes.status);
  });

  test("reads sync logs", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/integrations/sync-logs`, {
    });
    expect(res.ok).toBe(true);
    const body = (await res.json()) as { data: unknown[] | null };
    expect(body.data === null || Array.isArray(body.data)).toBe(true);
  });

  test("triggers HubSpot sync", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/integrations/hubspot/sync`, {
      method: "POST",
    });
    expect([200, 202, 400, 409]).toContain(res.status);
  });

  // ── Marketing ───────────────────────────────────────────────
  test("sends marketing batch email", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/marketing/send`, {
      method: "POST",
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
      const body = (await res.json()) as { data: { sent: number; failed: number } };
      expect(typeof body.data.sent).toBe("number");
    }
  });
});
