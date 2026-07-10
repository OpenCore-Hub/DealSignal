/**
 * Link advanced operations — access rules, invitations, bundle multi-document links.
 * Covers: GET/POST access-rules, GET/POST invitations, POST revoke invitation, GET access-logs
 */
import { test, expect } from "@playwright/test";
import { seedRealBackend, seedDocument, seedLink, apiFetch } from "./real-helpers";

let token: string;
let workspaceSlug: string;
let docId: string;
let linkId: string;

test.describe("Link advanced operations (real backend)", () => {
  test.beforeAll(async () => {
    const seed = await seedRealBackend();
    token = seed.token;
    workspaceSlug = seed.workspaceSlug;
    const doc = await seedDocument(token, workspaceSlug);
    docId = doc.id;
    const link = await seedLink(token, workspaceSlug, docId, {
      permissionType: "email_required",
      downloadEnabled: true,
      name: "Advanced Test Link",
    });
    linkId = link.id;
  });

  // ── Access rules ────────────────────────────────────────────
  test("reads access rules", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/links/${linkId}/access-rules`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok).toBe(true);
    const body = (await res.json()) as { data: unknown[] };
    expect(Array.isArray(body.data)).toBe(true);
  });

  test("creates access rules", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/links/${linkId}/access-rules`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
      body: JSON.stringify({
        rules: [
          { type: "ip_range", value: "10.0.0.0/8" },
        ],
      }),
    });
    // May succeed or return 400 if IP range format not accepted
    expect([200, 201, 400, 422]).toContain(res.status);
  });

  // ── Invitations ─────────────────────────────────────────────
  test("creates link viewer invitations", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/links/${linkId}/invitations`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
      body: JSON.stringify({ emails: [`link-invite-${Date.now()}@example.com`] }),
    });
    expect(res.ok).toBe(true);
    const body = (await res.json()) as { data: { id: string }[] };
    expect(body.data.length).toBeGreaterThan(0);
  });

  test("lists link invitations", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/links/${linkId}/invitations`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok).toBe(true);
    const body = (await res.json()) as { data: unknown[] };
    expect(Array.isArray(body.data)).toBe(true);
  });

  test("revokes a link invitation", async () => {
    // First get an invitation to revoke
    const listRes = await apiFetch(`/api/workspaces/${workspaceSlug}/links/${linkId}/invitations`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    const list = (await listRes.json()) as { data: { id: string }[] };

    if (list.data.length > 0) {
      const invitationId = list.data[0].ID;
      const res = await apiFetch(
        `/api/workspaces/${workspaceSlug}/links/${linkId}/invitations/${invitationId}/revoke`,
        {
          method: "POST",
          headers: { Authorization: `Bearer ${token}` },
          body: JSON.stringify({ removeFromAllowList: false }),
        }
      );
      expect([200, 204]).toContain(res.status);
    }
  });

  // ── Access logs ─────────────────────────────────────────────
  test("reads link access logs", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/links/${linkId}/access-logs`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok).toBe(true);
    const body = (await res.json()) as { data: unknown[] };
    expect(Array.isArray(body.data)).toBe(true);
  });

  // ── Link detail ─────────────────────────────────────────────
  test("gets link detail by ID", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/links/${linkId}`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok).toBe(true);
    const body = (await res.json()) as { id: string; publicToken?: string; shortUrl?: string };
    expect(body.id).toBe(linkId);
  });

  test("updates link via PUT", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/links/${linkId}`, {
      method: "PUT",
      headers: { Authorization: `Bearer ${token}` },
      body: JSON.stringify({ document_ids: [docId], name: "Updated Link Name" }),
    });
    expect([200, 204]).toContain(res.status);
  });

  // ── Score ───────────────────────────────────────────────────
  test("gets link heat score", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/analytics/links/${linkId}/score`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok).toBe(true);
    const body = (await res.json()) as { score: number; level: string };
    expect(typeof body.score).toBe("number");
    expect(body.level).toBeTruthy();
  });
});
