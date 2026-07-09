/**
 * Real-backend E2E helpers: seed data via API and authenticate the browser.
 *
 * Usage in specs:
 *   import { seedRealBackend, authenticatePage, apiFetch, attachDebug } from "./real-helpers";
 *
 *   let seed: Awaited<ReturnType<typeof seedRealBackend>>;
 *   test.beforeAll(async () => { seed = await seedRealBackend(); });
 *   test("...", async ({ page }) => {
 *     await authenticatePage(page, seed.token);
 *     await page.goto(`/${seed.workspaceSlug}/dashboard`);
 *   });
 */
import type { Page } from "@playwright/test";
import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// ── Config ────────────────────────────────────────────────────────
const API_BASE = process.env.REAL_API_BASE_URL || "http://localhost:8080";
const FIXTURES_DIR = path.join(__dirname, "fixtures");
const PDF_PATH = path.join(FIXTURES_DIR, "sample.pdf");

// ── Types ─────────────────────────────────────────────────────────
interface SeedResult {
  token: string;
  workspaceSlug: string;
  workspaceId: string;
  tenantId: string;
  userId: string;
}

interface SeedDocument {
  id: string;
  title: string;
  pageCount: number;
}

interface SeedLink {
  id: string;
  shortUrl: string;
  publicToken: string;
  permissionType: string;
}

interface SeedDealRoom {
  id: string;
  slug: string;
}

interface SeedContact {
  id: string;
  email: string;
}

// ── API helpers ───────────────────────────────────────────────────
export async function apiFetch(input: string, init?: RequestInit): Promise<Response> {
  return fetch(`${API_BASE}${input}`, {
    headers: { "Content-Type": "application/json", ...init?.headers },
    ...init,
  });
}

export async function apiGetJson<T>(path: string, token: string): Promise<T> {
  const res = await apiFetch(path, { headers: { Authorization: `Bearer ${token}` } });
  if (!res.ok) throw new Error(`GET ${path} failed: ${res.status} ${await res.text()}`);
  return res.json() as Promise<T>;
}

// ── Authenticate browser ──────────────────────────────────────────
export function attachDebug(page: Page) {
  page.on("console", (msg) => console.log(`[browser ${msg.type()}] ${msg.text()}`));
  page.on("pageerror", (err) => console.log(`[browser error] ${err.message}`));
  page.on("response", (response) => {
    if (response.status() >= 400) {
      console.log(`[browser ${response.status()}] ${response.request().method()} ${response.url()}`);
    }
  });
}

export async function authenticatePage(page: Page, token: string) {
  await page.addInitScript((t: string) => {
    localStorage.setItem("access_token", t);
  }, token);
}

// ── Comprehensive seed ────────────────────────────────────────────
export async function seedRealBackend(): Promise<SeedResult> {
  const ts = Date.now();
  const email = `e2e-${ts}@example.com`;
  const password = "Password123!";

  // 1. Register
  const regRes = await apiFetch("/api/auth/register", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
  if (!regRes.ok) throw new Error(`register failed: ${regRes.status} ${await regRes.text()}`);
  const reg = (await regRes.json()) as { user: { id: string }; access_token: string };
  const token = reg.access_token;
  const userId = reg.user.id;

  // 2. Create workspace
  const slug = `e2e-${ts}`;
  const wsRes = await apiFetch("/api/workspaces", {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: JSON.stringify({ name: "E2E Workspace", slug, brand_color: "#0055ff" }),
  });
  if (!wsRes.ok) throw new Error(`workspace create failed: ${wsRes.status} ${await wsRes.text()}`);
  const ws = (await wsRes.json()) as { id: string; tenant_id?: string };
  const workspaceId = ws.id;

  return { token, workspaceSlug: slug, workspaceId, tenantId: "", userId };
}

// ── Document upload + wait for ingestion ──────────────────────────
export async function seedDocument(token: string, workspaceSlug: string): Promise<SeedDocument> {
  const buffer = fs.readFileSync(PDF_PATH);
  const file = new File([buffer], "sample.pdf", { type: "application/pdf" });
  const form = new FormData();
  form.append("file", file);

  const uploadRes = await fetch(`${API_BASE}/api/workspaces/${workspaceSlug}/documents`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: form,
  });
  if (!uploadRes.ok) {
    throw new Error(`upload failed: ${uploadRes.status} ${await uploadRes.text()}`);
  }
  const doc = (await uploadRes.json()) as { id: string; title: string; page_count?: number; status: string };
  const docId = doc.id;

  // Poll until ready
  for (let i = 0; i < 30; i++) {
    await sleep(1000);
    const statusRes = await apiFetch(
      `/api/workspaces/${workspaceSlug}/documents/${docId}/status`,
      { headers: { Authorization: `Bearer ${token}` } }
    );
    if (!statusRes.ok) continue;
    const st = (await statusRes.json()) as { status: string; page_count?: number };
    if (st.status === "ready") {
      return { id: docId, title: doc.title, pageCount: st.page_count ?? 10 };
    }
    if (st.status === "failed") {
      throw new Error(`ingestion failed for doc ${docId}`);
    }
  }
  throw new Error(`document ${docId} did not become ready in 30s`);
}

// ── Link creation ─────────────────────────────────────────────────
export async function seedLink(
  token: string,
  workspaceSlug: string,
  documentId: string,
  opts: {
    name?: string;
    permissionType?: string;
    requireEmailVerification?: boolean;
    requirePassword?: boolean;
    requireNda?: boolean;
    password?: string;
    allowedEmails?: string[];
    allowedDomains?: string[];
    downloadEnabled?: boolean;
    watermarkEnabled?: boolean;
    expiresAt?: string;
    maxAccessCount?: number;
    aiCopilotEnabled?: boolean;
  } = {}
): Promise<SeedLink> {
  const body: Record<string, unknown> = {
    document_id: documentId,
    name: opts.name ?? "E2E Link",
    download_enabled: opts.downloadEnabled ?? true,
  };
  if (opts.permissionType) body.permission_type = opts.permissionType;
  if (opts.requireEmailVerification) body.require_email_verification = true;
  if (opts.requirePassword) body.require_password = true;
  if (opts.requireNda) body.require_nda = true;
  if (opts.password) body.password = opts.password;
  if (opts.allowedEmails) body.allowed_emails = opts.allowedEmails;
  if (opts.allowedDomains) body.allowed_domains = opts.allowedDomains;
  if (opts.expiresAt) body.expires_at = opts.expiresAt;
  if (typeof opts.maxAccessCount === "number") body.max_access_count = opts.maxAccessCount;
  if (typeof opts.watermarkEnabled === "boolean") body.watermark_enabled = opts.watermarkEnabled;
  if (typeof opts.aiCopilotEnabled === "boolean") body.ai_copilot_enabled = opts.aiCopilotEnabled;

  const res = await apiFetch(`/api/workspaces/${workspaceSlug}/links`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`create link failed: ${res.status} ${await res.text()}`);
  const link = (await res.json()) as { id: string; shortUrl: string; permissionType?: string };
  const publicToken = link.shortUrl.split("/").pop()!;
  return { id: link.id, shortUrl: link.shortUrl, publicToken, permissionType: link.permissionType ?? "public" };
}

// ── Deal room creation (with folders + document) ──────────────────
export async function seedDealRoom(
  token: string,
  workspaceSlug: string,
  opts: {
    name?: string;
    description?: string;
    templateType?: string;
    ndaEnabled?: boolean;
    requiresApproval?: boolean;
    documentIds?: string[];
  } = {}
): Promise<SeedDealRoom> {
  const ts = Date.now();
  const body: Record<string, unknown> = {
    name: opts.name ?? `E2E Room ${ts}`,
    slug: `e2e-room-${ts}`,
    description: opts.description ?? "E2E test deal room",
    template_type: opts.templateType ?? "seed",
  };
  if (opts.ndaEnabled) body.nda_enabled = true;
  if (opts.requiresApproval) body.requires_approval = true;

  const res = await apiFetch(`/api/workspaces/${workspaceSlug}/deal-rooms`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`create deal room failed: ${res.status} ${await res.text()}`);
  const room = (await res.json()) as { id: string; slug: string };

  // Add documents if provided
  if (opts.documentIds && opts.documentIds.length > 0) {
    for (const docId of opts.documentIds) {
      await apiFetch(`/api/workspaces/${workspaceSlug}/deal-rooms/${room.id}/documents`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
        body: JSON.stringify({ document_id: docId }),
      });
    }
  }

  return { id: room.id, slug: room.slug };
}

// ── Contact creation ──────────────────────────────────────────────
export async function seedContact(
  token: string,
  workspaceSlug: string,
  email: string,
  name?: string
): Promise<SeedContact> {
  const res = await apiFetch(`/api/workspaces/${workspaceSlug}/contacts`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: JSON.stringify({ email, name }),
  });
  if (!res.ok) throw new Error(`create contact failed: ${res.status} ${await res.text()}`);
  return (await res.json()) as SeedContact;
}

// ── Visit a public link and record page view ──────────────────────
export async function visitPublicLink(
  page: Page,
  shortUrl: string,
  gate?: { email?: string; emailCode?: string; password?: string; nda?: boolean }
) {
  await page.goto(shortUrl);

  // Handle email gate
  if (gate?.email) {
    const emailInput = page.locator("#email");
    if (await emailInput.isVisible({ timeout: 5000 }).catch(() => false)) {
      await emailInput.fill(gate.email);
      await page.getByRole("button", { name: /continue/i }).click();
    }
  }

  // Handle email code gate
  if (gate?.emailCode) {
    const codeInput = page.locator('input[inputmode="numeric"]');
    if (await codeInput.isVisible({ timeout: 5000 }).catch(() => false)) {
      await codeInput.fill(gate.emailCode);
      await page.getByRole("button", { name: /continue/i }).click();
    }
  }

  // Handle password gate
  if (gate?.password) {
    const pwdInput = page.locator("#password");
    if (await pwdInput.isVisible({ timeout: 5000 }).catch(() => false)) {
      await pwdInput.fill(gate.password);
      await page.getByRole("button", { name: /continue/i }).click();
    }
  }

  // Handle NDA gate
  if (gate?.nda) {
    const ndaCheckbox = page.getByRole("checkbox", { name: /agree/i });
    if (await ndaCheckbox.isVisible({ timeout: 5000 }).catch(() => false)) {
      await ndaCheckbox.check();
      await page.getByRole("button", { name: /continue/i }).click();
    }
  }

  // Wait for viewer to render
  await page.locator("img[alt*='Page']").first().waitFor({ state: "visible", timeout: 15000 }).catch(() => {
    // Might not have images, check for page text
    console.log("[visitPublicLink] no page image visible, continuing");
  });
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
