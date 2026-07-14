/**
 * Real-backend E2E helpers: seed data via API and authenticate the browser.
 *
 * Usage in specs:
 *   import { seedRealBackend, authenticatePage, apiFetch, attachDebug } from "./real-helpers";
 *
 *   let seed: Awaited<ReturnType<typeof seedRealBackend>>;
 *   test.beforeAll(async () => { seed = await seedRealBackend(); });
 *   test("...", async ({ page }) => {
 *     await authenticatePage(page);
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
const API_URL = new URL(API_BASE);
const FIXTURES_DIR = path.join(__dirname, "fixtures");
const PDF_PATH = path.join(FIXTURES_DIR, "sample.pdf");

// ── Types ─────────────────────────────────────────────────────────
interface SeedResult {
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
  contactEmail?: string;
}

interface SeedDealRoom {
  id: string;
  slug: string;
}

interface SeedContact {
  id: string;
  email: string;
  name?: string;
}

interface ParsedCookie {
  name: string;
  value: string;
  domain?: string;
  path?: string;
  httpOnly?: boolean;
  secure?: boolean;
  expires?: number; // Unix seconds
  sameSite?: "Strict" | "Lax" | "None";
}

// ── Cookie jar ────────────────────────────────────────────────────
let cookieJar: ParsedCookie[] = [];

export function getCookieJar(): string[] {
  return cookieJar.map((c) => `${c.name}=${c.value}`);
}

export function clearCookieJar(): void {
  cookieJar = [];
}

function parseSameSite(
  value: string | undefined
): "Strict" | "Lax" | "None" | undefined {
  if (!value) return undefined;
  const normalized = value.trim().toLowerCase();
  if (normalized === "strict") return "Strict";
  if (normalized === "lax") return "Lax";
  if (normalized === "none") return "None";
  return undefined;
}

function parseSetCookie(setCookie: string): ParsedCookie | null {
  const parts = setCookie.split(";").map((p) => p.trim());
  const first = parts[0];
  if (!first) return null;

  const eq = first.indexOf("=");
  if (eq < 0) return null;

  const name = first.slice(0, eq).trim();
  let value = first.slice(eq + 1).trim();
  // Strip surrounding quotes if present
  if (value.length >= 2 && value.startsWith('"') && value.endsWith('"')) {
    value = value.slice(1, -1);
  }

  const cookie: ParsedCookie = { name, value };

  for (let i = 1; i < parts.length; i++) {
    const part = parts[i];
    const [attrName, attrValue = ""] = part.split("=").map((s) => s.trim());
    const key = attrName.toLowerCase();
    if (key === "path") {
      cookie.path = attrValue || "/";
    } else if (key === "domain") {
      cookie.domain = attrValue;
    } else if (key === "httponly") {
      cookie.httpOnly = true;
    } else if (key === "secure") {
      cookie.secure = true;
    } else if (key === "samesite") {
      cookie.sameSite = parseSameSite(attrValue);
    } else if (key === "max-age") {
      const seconds = parseInt(attrValue, 10);
      if (!isNaN(seconds)) {
        cookie.expires = Math.floor(Date.now() / 1000) + seconds;
      }
    } else if (key === "expires") {
      // If Max-Age is also present it takes precedence; we'll overwrite below
      // if needed, but for now set from Expires.
      const d = new Date(attrValue);
      if (!isNaN(d.getTime())) {
        cookie.expires = Math.floor(d.getTime() / 1000);
      }
    }
  }

  return cookie;
}

function updateCookieJar(setCookieHeader: string | null | undefined): void {
  if (!setCookieHeader) return;

  const parsed = parseSetCookie(setCookieHeader);
  if (!parsed) return;

  cookieJar = cookieJar.filter(
    (c) => c.name.toLowerCase() !== parsed.name.toLowerCase()
  );

  // Empty value with an explicit expiration in the past means deletion.
  if (parsed.value || parsed.expires === undefined || parsed.expires > Date.now() / 1000) {
    cookieJar.push(parsed);
  }
}

function updateJarFromResponse(res: Response): void {
  const headers = res.headers as unknown as Headers;
  let setCookies: string[] = [];
  if (typeof headers.getSetCookie === "function") {
    setCookies = headers.getSetCookie();
  } else {
    const combined = headers.get("Set-Cookie");
    if (combined) {
      // Best-effort split; backend cookies should not contain unquoted commas.
      setCookies = combined.split(",").map((s) => s.trim());
    }
  }
  for (const c of setCookies) {
    updateCookieJar(c);
  }
}

// ── API helpers ───────────────────────────────────────────────────
export async function apiFetch(input: string, init?: RequestInit): Promise<Response> {
  const headers = new Headers(init?.headers);

  const body = init?.body;
  const hasContentType = headers.has("Content-Type");
  const isFormData = body instanceof FormData;
  if (!hasContentType && !isFormData) {
    headers.set("Content-Type", "application/json");
  }

  const cookieHeader = getCookieJar().join("; ");
  if (cookieHeader) {
    headers.set("Cookie", cookieHeader);
  }

  const res = await fetch(`${API_BASE}${input}`, {
    ...init,
    headers,
  });

  updateJarFromResponse(res);
  return res;
}

export async function apiGetJson<T>(path: string): Promise<T> {
  const res = await apiFetch(path);
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

export async function authenticatePage(page: Page) {
  if (cookieJar.length === 0) {
    console.warn("[authenticatePage] cookie jar is empty; browser will not be authenticated");
    return;
  }

  const domain = API_URL.hostname;
  const playwrightCookies = cookieJar.map((c) => ({
    name: c.name,
    value: c.value,
    domain: c.domain || domain,
    path: c.path || "/",
    httpOnly: c.httpOnly ?? false,
    secure: c.secure ?? false,
    expires: c.expires,
    sameSite: c.sameSite,
  }));

  await page.context().addCookies(playwrightCookies);
}

// ── Comprehensive seed ────────────────────────────────────────────
export async function seedRealBackend(): Promise<SeedResult> {
  clearCookieJar();

  const ts = Date.now();
  const email = `e2e-${ts}@example.com`;
  const password = "Password123!";

  // 1. Register
  const regRes = await apiFetch("/api/auth/register", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
  if (!regRes.ok) throw new Error(`register failed: ${regRes.status} ${await regRes.text()}`);
  const reg = (await regRes.json()) as { user: { id: string } };
  const userId = reg.user.id;

  // 2. Create workspace
  const slug = `e2e-${ts}`;
  const wsRes = await apiFetch("/api/workspaces", {
    method: "POST",
    body: JSON.stringify({ name: "E2E Workspace", slug, brand_color: "#0055ff" }),
  });
  if (!wsRes.ok) throw new Error(`workspace create failed: ${wsRes.status} ${await wsRes.text()}`);
  const ws = (await wsRes.json()) as { id: string; tenant_id?: string };
  const workspaceId = ws.id;

  return { workspaceSlug: slug, workspaceId, tenantId: ws.tenant_id ?? "", userId };
}

// ── Document upload + wait for ingestion ──────────────────────────
export async function seedDocument(workspaceSlug: string): Promise<SeedDocument> {
  const buffer = fs.readFileSync(PDF_PATH);
  const file = new File([buffer], "sample.pdf", { type: "application/pdf" });
  const form = new FormData();
  form.append("file", file);

  const uploadRes = await apiFetch(`/api/workspaces/${workspaceSlug}/documents`, {
    method: "POST",
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
      `/api/workspaces/${workspaceSlug}/documents/${docId}/status`
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
  workspaceSlug: string,
  documentId: string,
  opts: {
    name?: string;
    permissionType?: string;
    requireEmail?: boolean;
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
    contactEmail?: string;
    contactName?: string;
  } = {}
): Promise<SeedLink> {
  const body: Record<string, unknown> = {
    document_id: documentId,
    name: opts.name ?? "E2E Link",
    download_enabled: opts.downloadEnabled ?? true,
  };
  if (opts.permissionType) body.permission_type = opts.permissionType;
  if (opts.requireEmail) body.require_email = true;
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

  let contactEmail: string | undefined;
  if (opts.requireEmailVerification || opts.requireNda) {
    contactEmail = opts.contactEmail ?? `contact-${Date.now()}@example.com`;
    const contact = await seedContact(workspaceSlug, contactEmail, opts.contactName ?? "E2E Contact");
    body.contact_ids = [contact.id];
  }

  const res = await apiFetch(`/api/workspaces/${workspaceSlug}/links`, {
    method: "POST",
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`create link failed: ${res.status} ${await res.text()}`);
  const link = (await res.json()) as { id: string; shortUrl: string; permissionType?: string };
  const publicToken = link.shortUrl.split("/").pop()!;
  return { id: link.id, shortUrl: link.shortUrl, publicToken, permissionType: link.permissionType ?? "public", contactEmail };
}

// ── Deal room creation (with folders + document) ──────────────────
export async function seedDealRoom(
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
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`create deal room failed: ${res.status} ${await res.text()}`);
  const room = (await res.json()) as { id: string; slug: string };

  // Add documents if provided
  if (opts.documentIds && opts.documentIds.length > 0) {
    for (const docId of opts.documentIds) {
      await apiFetch(`/api/workspaces/${workspaceSlug}/deal-rooms/${room.id}/documents`, {
        method: "POST",
        body: JSON.stringify({ document_id: docId }),
      });
    }
  }

  return { id: room.id, slug: room.slug };
}

// ── Contact creation ──────────────────────────────────────────────
export async function seedContact(
  workspaceSlug: string,
  email: string,
  name?: string
): Promise<SeedContact> {
  const res = await apiFetch(`/api/workspaces/${workspaceSlug}/contacts`, {
    method: "POST",
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

  // Fill all visible gate fields, then submit once. The public viewer renders
  // every configured control on the first response, so we must not click
  // Continue between fields.
  const emailInput = page.locator("#email");
  if (gate?.email && (await emailInput.isVisible({ timeout: 5000 }).catch(() => false))) {
    await emailInput.fill(gate.email);
  }

  const codeInput = page.locator('input[inputmode="numeric"]');
  if (gate?.emailCode && (await codeInput.isVisible({ timeout: 5000 }).catch(() => false))) {
    await codeInput.fill(gate.emailCode);
  }

  const pwdInput = page.locator("#password");
  if (gate?.password && (await pwdInput.isVisible({ timeout: 5000 }).catch(() => false))) {
    await pwdInput.fill(gate.password);
  }

  const ndaCheckbox = page.getByRole("checkbox", { name: /agree/i });
  if (gate?.nda && (await ndaCheckbox.isVisible({ timeout: 5000 }).catch(() => false))) {
    await ndaCheckbox.check();
  }

  const continueButton = page.getByRole("button", { name: /continue/i });
  if (await continueButton.isVisible({ timeout: 5000 }).catch(() => false)) {
    await continueButton.click();
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
