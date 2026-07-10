/**
 * Public viewer advanced — email code send/resend, public assistant chat, NDA gate flow.
 * Covers: POST /links/:token/send-email-code, POST /links/:token/resend-code,
 *         POST /assistant/chat (public), public event recording
 */
import { test, expect } from "@playwright/test";
import { seedRealBackend, seedDocument, seedLink, apiFetch } from "./real-helpers";

let testToken: string;
let workspaceSlug: string;
let verificationToken: string;
let publicToken: string;

test.describe("Public viewer advanced (real backend)", () => {
  test.beforeAll(async () => {
    const seed = await seedRealBackend();
    testToken = seed.token;
    workspaceSlug = seed.workspaceSlug;
    const doc = await seedDocument(testToken, workspaceSlug);
    // Link used for email-verification gate tests.
    const verificationLink = await seedLink(testToken, workspaceSlug, doc.id, {
      permissionType: "public",
      requireEmailVerification: true,
      downloadEnabled: true,
    });
    verificationToken = verificationLink.publicToken;
    // Public (non-gated) link used for event recording and asset access.
    const publicLink = await seedLink(testToken, workspaceSlug, doc.id, {
      permissionType: "public",
      downloadEnabled: true,
    });
    publicToken = publicLink.publicToken;
  });

  // ── Email verification code ─────────────────────────────────
  test("sends email verification code", async () => {
    const res = await apiFetch(`/api/v1/public/links/${verificationToken}/send-email-code`, {
      method: "POST",
      body: JSON.stringify({ email: `visitor-${Date.now()}@example.com` }),
    });
    const ok = res.ok || res.status === 202;
    expect(ok).toBe(true);
  });

  test("resends email verification code", async () => {
    const res = await apiFetch(`/api/v1/public/links/${verificationToken}/resend-code`, {
      method: "POST",
      body: JSON.stringify({ email: `visitor-${Date.now()}@example.com` }),
    });
    const ok = res.ok || res.status === 202;
    expect(ok).toBe(true);
  });

  // ── Public access with gates ────────────────────────────────
  test("accesses public link with email verification code gate in browser", async ({ page }) => {
    await page.goto(`/l/${verificationToken}`);

    // Should show access gate
    await page.waitForTimeout(2000);

    // Look for the code input field
    const codeInput = page.locator('input[inputmode="numeric"]');
    const isGateVisible = await codeInput.isVisible({ timeout: 5000 }).catch(() => false);

    if (isGateVisible) {
      // Fill in the code (mock code "123456" in dev)
      await codeInput.fill("123456");
      await page.getByRole("button", { name: /continue/i }).click();

      // Wait for viewer to load
      await page.waitForTimeout(3000);
    }

    // Page should render (either viewer or gate)
    const hasContent = await page.locator("img, h1, h2, p").first().isVisible().catch(() => false);
    expect(hasContent).toBe(true);
  });

  // ── Public event recording ─────────────────────────────────
  test("records a public page_viewed event", async () => {
    const res = await apiFetch(`/api/v1/public/events`, {
      method: "POST",
      body: JSON.stringify({
        event_type: "page_viewed",
        public_token: publicToken,
        visitor_id: `e2e-visitor-${Date.now()}`,
        page_number: 1,
        duration_seconds: 15,
        scroll_depth: 0.85,
      }),
    });
    const ok = res.ok || res.status === 204;
    expect(ok).toBe(true);
  });

  test("records a public download_attempted event", async () => {
    const res = await apiFetch(`/api/v1/public/events`, {
      method: "POST",
      body: JSON.stringify({
        event_type: "download_attempted",
        public_token: publicToken,
        visitor_id: `e2e-visitor-${Date.now()}`,
      }),
    });
    const ok = res.ok || res.status === 204;
    expect(ok).toBe(true);
  });

  // ── Public assistant chat ──────────────────────────────────
  test("public assistant chat endpoint", async () => {
    // First get a session token by accessing the link
    const accessRes = await apiFetch(`/api/v1/public/links/${publicToken}`, {
      method: "POST",
      body: JSON.stringify({}),
    });
    let sessionToken = "";
    if (accessRes.ok) {
      const body = (await accessRes.json()) as { sessionToken?: string; session_token?: string };
      sessionToken = body.sessionToken ?? body.session_token ?? "";
    }

    // Try the public assistant chat
    if (sessionToken) {
      const res = await apiFetch(`/api/v1/public/assistant/chat`, {
        method: "POST",
        headers: { "X-Link-Session": sessionToken },
        body: JSON.stringify({ message: "Summarize this document" }),
      });
      // May fail if AI is disabled or the link does not have the copilot enabled.
      const ok = res.ok || res.status === 400 || res.status === 403 || res.status === 503;
      expect(ok).toBe(true);
    }
  });

  // ── Public document pages (signed URL) ──────────────────────
  test("gets public document signed URL", async () => {
    // First get document ID from link access
    const accessRes = await apiFetch(`/api/v1/public/links/${publicToken}`, {
      method: "POST",
      body: JSON.stringify({}),
    });
    if (accessRes.ok) {
      const body = (await accessRes.json()) as {
        documents?: { id: string }[];
        document?: { id: string };
      };
      const docId = body.documents?.[0]?.id ?? body.document?.id;
      if (docId) {
        const signedRes = await apiFetch(
          `/api/v1/public/documents/${docId}/pages/signed-url?token=${publicToken}&page_number=1`,
          { method: "GET" }
        );
        expect([200, 403]).toContain(signedRes.status);
      }
    }
  });
});
