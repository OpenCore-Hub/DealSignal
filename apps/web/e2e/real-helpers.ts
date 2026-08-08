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
const API_BASE = process.env.REAL_API_BASE_URL || process.env.VITE_API_BASE_URL || "http://localhost:8080";
const API_URL = new URL(API_BASE);
const APP_ORIGIN = process.env.PLAYWRIGHT_BASE_URL || "http://localhost:5173";
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
/** publicToken → X-Link-Session token (mirrors browser PublicViewer session). */
const linkSessionByToken = new Map<string, string>();

export function getCookieJar(): string[] {
  return cookieJar.map((c) => `${c.name}=${c.value}`);
}

export function clearCookieJar(): void {
  cookieJar = [];
  linkSessionByToken.clear();
}

function rememberLinkSession(publicToken: string, sessionToken: string | undefined | null): void {
  const token = publicToken.trim();
  const session = (sessionToken ?? "").trim();
  if (!token || !session) return;
  linkSessionByToken.set(token, session);
}

function linkSessionForPath(input: string): string | undefined {
  const match = input.match(/\/api\/v1\/public\/links\/([^/?#]+)/);
  if (!match?.[1]) return undefined;
  return linkSessionByToken.get(match[1]);
}

function captureLinkSessionRefresh(input: string, res: Response): void {
  const refreshed = res.headers.get("X-Link-Session-Refresh");
  if (!refreshed) return;
  const match = input.match(/\/api\/v1\/public\/links\/([^/?#]+)/);
  if (!match?.[1]) return;
  rememberLinkSession(match[1], refreshed);
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

  if (!headers.has("X-Link-Session")) {
    const linkSession = linkSessionForPath(input);
    if (linkSession) {
      headers.set("X-Link-Session", linkSession);
    }
  }

  const res = await fetch(`${API_BASE}${input}`, {
    ...init,
    headers,
  });

  updateJarFromResponse(res);
  captureLinkSessionRefresh(input, res);
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

  // Router gate reads non-HttpOnly auth_session on the Vite origin; API host/port
  // often differs (e.g. 127.0.0.1:8090 vs localhost:5173).
  await page.context().addCookies([
    {
      name: "auth_session",
      value: "1",
      url: APP_ORIGIN,
      sameSite: "Lax",
    },
  ]);
}

// ── Comprehensive seed ────────────────────────────────────────────
export async function seedRealBackend(): Promise<SeedResult> {
  clearCookieJar();

  const ts = Date.now();
  const email = `e2e-${ts}@example.com`;
  const password = "Password123!";

  // 1. Register (with retry/backoff for auth rate limiting in parallel E2E runs)
  const regBody = JSON.stringify({ email, password });
  let regRes: Response | undefined;
  let lastRegError: string | undefined;
  for (let attempt = 0; attempt < 5; attempt++) {
    regRes = await apiFetch("/api/auth/register", {
      method: "POST",
      body: regBody,
    });
    if (regRes.ok) break;
    lastRegError = await regRes.text();
    if (regRes.status === 429 && attempt < 4) {
      const delay = Math.min(1000 * 2 ** attempt + Math.random() * 200, 8000);
      await sleep(delay);
      continue;
    }
    break;
  }
  if (!regRes || !regRes.ok) {
    throw new Error(`register failed: ${regRes?.status ?? "no response"} ${lastRegError ?? ""}`);
  }
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
  const uniqueName = `sample-${Date.now()}-${Math.random().toString(36).slice(2, 8)}.pdf`;
  const file = new File([buffer], uniqueName, { type: "application/pdf" });
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
    ndaDocumentId?: string;
    password?: string;
    allowedEmails?: string[];
    downloadEnabled?: boolean;
    watermarkEnabled?: boolean;
    expiresAt?: string;
    maxAccessCount?: number;
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
  if (opts.requireNda && opts.ndaDocumentId) body.nda_document_id = opts.ndaDocumentId;
  if (opts.password) body.password = opts.password;
  if (opts.allowedEmails) body.allowed_emails = opts.allowedEmails;
  if (opts.expiresAt) body.expires_at = opts.expiresAt;
  if (typeof opts.maxAccessCount === "number") body.max_access_count = opts.maxAccessCount;
  if (typeof opts.watermarkEnabled === "boolean") body.watermark_enabled = opts.watermarkEnabled;

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

/** Open visitor Ask panel on a public link (real backend UI). */
export async function openRealVisitorAskPanel(page: Page, shortUrl: string) {
  await visitPublicLink(page, shortUrl);

  const openSidebar = page.getByRole("button", { name: /Open sidebar/i });
  if (await openSidebar.isVisible().catch(() => false)) {
    await openSidebar.click();
  }

  const input = page.getByPlaceholder(/materials you can access|Ask the host/i);
  if (!(await input.isVisible().catch(() => false))) {
    const askTab = page.locator("button.rounded-full").filter({ hasText: /^Ask$/ });
    if (await askTab.isVisible().catch(() => false)) {
      await askTab.click();
    }
  }

  await input.waitFor({ state: "visible", timeout: 15000 });
  return input;
}

/** Enable grounded AI on a deal-room link via ask-policy API (used by real-backend e2e). */
export async function enableGroundedAiForLink(
  workspaceSlug: string,
  linkId: string,
  askMode: "supervised" | "self_serve" = "supervised",
) {
  await updateLinkAskPolicy(workspaceSlug, linkId, {
    askAiEnabled: true,
    askMode,
  });
}

/** Returns false when docling-rag is not configured (optional AI gates skip). */
export async function probeKnowledgeEnabled(
  workspaceSlug: string,
  roomId: string,
): Promise<boolean> {
  const corpus = await apiGetJson<{ enabled: boolean }>(
    `/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/knowledge`,
  );
  return Boolean(corpus.enabled);
}

/** Sync deal-room knowledge corpus until ask-ready (or throw). */
export async function waitForKnowledgeCorpusReady(
  workspaceSlug: string,
  roomId: string,
  timeoutS = 180,
): Promise<void> {
  const syncRes = await apiFetch(
    `/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/knowledge/sync`,
    { method: "POST" },
  );
  if (syncRes.status !== 202 && syncRes.status !== 200) {
    throw new Error(`knowledge sync failed: ${syncRes.status} ${await syncRes.text()}`);
  }

  for (let i = 0; i < timeoutS; i++) {
    await sleep(1000);
    const corpus = await apiGetJson<{
      enabled: boolean;
      status: string;
      documents?: { status: string }[];
      progress?: { failed?: number };
    }>(`/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/knowledge`);
    if (!corpus.enabled) {
      throw new Error("knowledge disabled (set DOCLING_RAG_BASE_URL + PLATFORM_ADMIN_KEY)");
    }
    const synced = (corpus.documents ?? []).filter((d) => d.status === "synced").length;
    const bad = (corpus.documents ?? []).filter((d) =>
      d.status === "failed" || d.status === "pending" || d.status === "syncing",
    ).length;
    if (corpus.status === "ready" && synced >= 1 && bad === 0) return;
    if (corpus.status === "failed" || (corpus.progress?.failed ?? 0) > 0) {
      throw new Error(`knowledge corpus failed: ${JSON.stringify(corpus)}`);
    }
  }
  throw new Error(`knowledge corpus not ready within ${timeoutS}s`);
}

// ── Document category tri-state helpers ───────────────────────────

export async function fetchDocument(
  workspaceSlug: string,
  documentId: string,
): Promise<{ id: string; category?: string; title?: string; status?: string }> {
  const res = await apiFetch(`/api/workspaces/${workspaceSlug}/documents/${documentId}`);
  if (!res.ok) {
    throw new Error(`fetch document failed: ${res.status} ${await res.text()}`);
  }
  return (await res.json()) as { id: string; category?: string; title?: string; status?: string };
}

export async function listDocumentsByCategory(
  workspaceSlug: string,
  category: "general" | "agreement" | "deal_room",
): Promise<{ id: string; category?: string }[]> {
  const res = await apiFetch(
    `/api/workspaces/${workspaceSlug}/documents?category=${encodeURIComponent(category)}`,
  );
  if (!res.ok) {
    throw new Error(`list documents failed: ${res.status} ${await res.text()}`);
  }
  const body = (await res.json()) as { data: { id: string; category?: string }[] };
  return body.data;
}

/** POST /documents without waiting for ingestion — for contract/error tests. */
export async function uploadDocumentRaw(
  workspaceSlug: string,
  opts?: { category?: string; filename?: string },
): Promise<Response> {
  const buffer = fs.readFileSync(PDF_PATH);
  const uniqueName = opts?.filename ?? `sample-${Date.now()}-${Math.random().toString(36).slice(2, 8)}.pdf`;
  const form = new FormData();
  form.append("file", new File([buffer], uniqueName, { type: "application/pdf" }));
  if (opts?.category) form.append("category", opts.category);
  return apiFetch(`/api/workspaces/${workspaceSlug}/documents`, { method: "POST", body: form });
}

export async function seedAgreementDocument(workspaceSlug: string): Promise<SeedDocument> {
  const buffer = fs.readFileSync(PDF_PATH);
  const uniqueName = `nda-${Date.now()}-${Math.random().toString(36).slice(2, 8)}.pdf`;
  const file = new File([buffer], uniqueName, { type: "application/pdf" });
  const form = new FormData();
  form.append("file", file);
  form.append("category", "agreement");

  const uploadRes = await apiFetch(`/api/workspaces/${workspaceSlug}/documents`, {
    method: "POST",
    body: form,
  });
  if (!uploadRes.ok) {
    throw new Error(`agreement upload failed: ${uploadRes.status} ${await uploadRes.text()}`);
  }
  const doc = (await uploadRes.json()) as { id: string; title: string; category?: string };
  for (let i = 0; i < 30; i++) {
    await sleep(1000);
    const fetched = await fetchDocument(workspaceSlug, doc.id);
    if (fetched.status === "ready") {
      return { id: doc.id, title: doc.title, pageCount: 10 };
    }
    if (fetched.status === "failed") {
      throw new Error(`agreement ingestion failed for doc ${doc.id}`);
    }
  }
  throw new Error(`agreement document ${doc.id} did not become ready in 30s`);
}

export async function attachDocumentToRoom(
  workspaceSlug: string,
  roomId: string,
  documentId: string,
  folderPath = "/general",
): Promise<Response> {
  return apiFetch(`/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/documents`, {
    method: "POST",
    body: JSON.stringify({ document_id: documentId, folder_path: folderPath }),
  });
}

export async function detachDocumentFromRoom(
  workspaceSlug: string,
  roomId: string,
  documentId: string,
): Promise<Response> {
  return apiFetch(`/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/documents/${documentId}`, {
    method: "DELETE",
  });
}

// ── Visitor Ask (real backend) ────────────────────────────────────

export interface PublicAskTurn {
  id: string;
  question: string;
  lane: string;
  status: string;
  route_reason?: string;
  host_answer?: string;
}

export interface OwnerAskTurn {
  id: string;
  link_id: string;
  question: string;
  lane: string;
  status: string;
  visitor_email?: string;
  pinned_faq_at?: string;
  formal_status?: string;
}

export interface DashboardActionItem {
  id: string;
  sourceType?: string;
  sourceId?: string;
  targetId?: string;
  title: string;
  status: string;
}

export function snapshotCookieJar(): ParsedCookie[] {
  return [...cookieJar];
}

export function restoreCookieJar(snapshot: ParsedCookie[]): void {
  cookieJar = [...snapshot];
}

export function publicTokenFromShortUrl(shortUrl: string): string {
  const token = shortUrl.split("/").filter(Boolean).pop();
  if (!token) throw new Error(`invalid shortUrl: ${shortUrl}`);
  return token;
}

export async function seedDealRoomLink(
  workspaceSlug: string,
  roomId: string,
  opts: { name?: string; requireEmail?: boolean } = {},
): Promise<SeedLink> {
  const res = await apiFetch(`/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/links`, {
    method: "POST",
    body: JSON.stringify({
      name: opts.name ?? `E2E Room Link ${Date.now()}`,
      require_email: opts.requireEmail ?? false,
      download_enabled: true,
    }),
  });
  if (!res.ok) throw new Error(`create deal-room link failed: ${res.status} ${await res.text()}`);
  const link = (await res.json()) as {
    id: string;
    shortUrl?: string;
    short_url?: string;
    public_token?: string;
  };
  const shortUrl = link.shortUrl ?? link.short_url ?? "";
  const publicToken =
    link.public_token ?? (shortUrl ? publicTokenFromShortUrl(shortUrl) : "");
  if (!publicToken) throw new Error("deal-room link missing public token");
  return { id: link.id, shortUrl, publicToken, permissionType: "public" };
}

export async function accessPublicLinkApi(
  publicToken: string,
  email = `visitor-${Date.now()}@example.com`,
): Promise<{ visitorId: string; email: string; sessionToken: string }> {
  const res = await apiFetch(`/api/v1/public/links/${publicToken}`, {
    method: "POST",
    body: JSON.stringify({ email }),
  });
  if (!res.ok) throw new Error(`public access failed: ${res.status} ${await res.text()}`);
  const body = (await res.json()) as {
    visitorId?: string;
    visitor_id?: string;
    sessionToken?: string;
    session_token?: string;
  };
  const visitorId = body.visitorId ?? body.visitor_id ?? "";
  if (!visitorId) throw new Error("public access missing visitorId");
  const sessionToken = body.sessionToken ?? body.session_token ?? "";
  if (!sessionToken) throw new Error("public access missing sessionToken");
  rememberLinkSession(publicToken, sessionToken);
  return { visitorId, email, sessionToken };
}

export async function submitPublicAsk(
  publicToken: string,
  question: string,
): Promise<PublicAskTurn> {
  const res = await apiFetch(`/api/v1/public/links/${publicToken}/ask`, {
    method: "POST",
    body: JSON.stringify({ question }),
  });
  if (!res.ok) throw new Error(`public ask failed: ${res.status} ${await res.text()}`);
  const body = (await res.json()) as { data: PublicAskTurn };
  return body.data;
}

/** GET AI stream endpoint — expect 403 ai_not_enabled when ask_ai_enabled is false. */
export async function streamPublicAskTurn(
  publicToken: string,
  turnId: string,
): Promise<Response> {
  return apiFetch(`/api/v1/public/links/${publicToken}/ask/${turnId}/stream`, {
    method: "GET",
    headers: { Accept: "text/event-stream" },
  });
}

export async function updateLinkAskPolicy(
  workspaceSlug: string,
  linkId: string,
  policy: { askAiEnabled?: boolean; askMode?: string; askAiMonthlyQuota?: number; clearAiQuota?: boolean },
): Promise<void> {
  const body: Record<string, unknown> = {};
  if (policy.askAiEnabled !== undefined) body.ask_ai_enabled = policy.askAiEnabled;
  if (policy.askMode !== undefined) body.ask_mode = policy.askMode;
  if (policy.askAiMonthlyQuota !== undefined) body.ask_ai_monthly_quota = policy.askAiMonthlyQuota;
  if (policy.clearAiQuota) body.clear_ai_quota = true;
  const res = await apiFetch(`/api/workspaces/${workspaceSlug}/links/${linkId}/ask-policy`, {
    method: "PATCH",
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    throw new Error(`update link ask policy failed: ${res.status} ${await res.text()}`);
  }
}

export interface OwnerLinkDetail {
  id: string;
  askAiEnabled?: boolean;
  askMode?: string;
  dealRoomId?: string;
}

export async function fetchLinkById(
  workspaceSlug: string,
  linkId: string,
): Promise<OwnerLinkDetail> {
  const res = await apiFetch(`/api/workspaces/${workspaceSlug}/links/${linkId}`);
  if (!res.ok) {
    throw new Error(`fetch link failed: ${res.status} ${await res.text()}`);
  }
  return (await res.json()) as OwnerLinkDetail;
}

export interface LinkAnalyticsResponse {
  ask_summary?: {
    total_turns: number;
    ai_answered: number;
    ai_refused: number;
    host_pending: number;
    host_answered: number;
    user_escalated?: number;
    auto_escalated?: number;
    deflection_rate?: number;
    refuse_rate?: number;
    escalation_rate?: number;
  };
}

export async function fetchLinkAnalytics(
  workspaceSlug: string,
  linkId: string,
): Promise<LinkAnalyticsResponse> {
  const res = await apiFetch(`/api/workspaces/${workspaceSlug}/links/${linkId}/analytics`);
  if (!res.ok) {
    throw new Error(`fetch link analytics failed: ${res.status} ${await res.text()}`);
  }
  return (await res.json()) as LinkAnalyticsResponse;
}

/** Parse visitor Ask SSE body for a done payload answer snippet. */
export function parseVisitorAskSSE(raw: string): { answer: string; refused: boolean } {
  let answer = "";
  let refused = false;
  for (const block of raw.split("\n\n")) {
    const lines = block.split("\n");
    let event = "";
    let data = "";
    for (const line of lines) {
      if (line.startsWith("event:")) event = line.slice(6).trim();
      if (line.startsWith("data:")) data += line.slice(5).trim();
    }
    if (!data) continue;
    try {
      const parsed = JSON.parse(data) as {
        answer?: string;
        refused?: boolean;
        turn?: { answer?: string; refused?: boolean };
      };
      if (event === "done" || parsed.answer || parsed.turn?.answer) {
        answer = parsed.answer ?? parsed.turn?.answer ?? answer;
        refused = parsed.refused ?? parsed.turn?.refused ?? refused;
      }
    } catch {
      // ignore partial chunks
    }
  }
  return { answer, refused };
}

export async function listMyPublicAskTurns(publicToken: string): Promise<PublicAskTurn[]> {
  const res = await apiFetch(`/api/v1/public/links/${publicToken}/ask/me`);
  if (!res.ok) throw new Error(`public ask/me failed: ${res.status} ${await res.text()}`);
  const body = (await res.json()) as { data: PublicAskTurn[] };
  return body.data ?? [];
}

export async function listOwnerLinkAsk(
  workspaceSlug: string,
  linkId: string,
  query?: { lane?: string; status?: string },
): Promise<OwnerAskTurn[]> {
  const params = new URLSearchParams();
  if (query?.lane) params.set("lane", query.lane);
  if (query?.status) params.set("status", query.status);
  const qs = params.toString();
  const res = await apiFetch(
    `/api/workspaces/${workspaceSlug}/links/${linkId}/ask${qs ? `?${qs}` : ""}`,
  );
  if (!res.ok) throw new Error(`owner link ask failed: ${res.status} ${await res.text()}`);
  const body = (await res.json()) as { data: OwnerAskTurn[] };
  return body.data ?? [];
}

export async function listOwnerRoomAsk(
  workspaceSlug: string,
  roomId: string,
  query?: { linkId?: string; lane?: string; status?: string },
): Promise<OwnerAskTurn[]> {
  const params = new URLSearchParams();
  if (query?.linkId) params.set("link_id", query.linkId);
  if (query?.lane) params.set("lane", query.lane);
  if (query?.status) params.set("status", query.status);
  const qs = params.toString();
  const res = await apiFetch(
    `/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/ask${qs ? `?${qs}` : ""}`,
  );
  if (!res.ok) throw new Error(`owner room ask failed: ${res.status} ${await res.text()}`);
  const body = (await res.json()) as { data: OwnerAskTurn[] };
  return body.data ?? [];
}

export async function answerOwnerAskTurn(
  workspaceSlug: string,
  linkId: string,
  turnId: string,
  answer: string,
): Promise<OwnerAskTurn> {
  const res = await apiFetch(
    `/api/workspaces/${workspaceSlug}/links/${linkId}/ask/${turnId}/host-answer`,
    {
      method: "PATCH",
      body: JSON.stringify({ answer }),
    },
  );
  if (!res.ok) throw new Error(`host answer failed: ${res.status} ${await res.text()}`);
  const body = (await res.json()) as { data: OwnerAskTurn };
  return body.data;
}

interface PublicAskFAQ {
  id: string;
  question: string;
  answer: string;
  pinned_at?: string;
}

export async function pinOwnerAskTurnFAQ(
  workspaceSlug: string,
  linkId: string,
  turnId: string,
): Promise<OwnerAskTurn> {
  const res = await apiFetch(
    `/api/workspaces/${workspaceSlug}/links/${linkId}/ask/${turnId}/pin-faq`,
    { method: "POST" },
  );
  if (!res.ok) throw new Error(`pin FAQ failed: ${res.status} ${await res.text()}`);
  const body = (await res.json()) as { data: OwnerAskTurn };
  return body.data;
}

export async function unpinOwnerAskTurnFAQ(
  workspaceSlug: string,
  linkId: string,
  turnId: string,
): Promise<OwnerAskTurn> {
  const res = await apiFetch(
    `/api/workspaces/${workspaceSlug}/links/${linkId}/ask/${turnId}/unpin-faq`,
    { method: "POST" },
  );
  if (!res.ok) throw new Error(`unpin FAQ failed: ${res.status} ${await res.text()}`);
  const body = (await res.json()) as { data: OwnerAskTurn };
  return body.data;
}

export async function listPublicAskFAQs(publicToken: string): Promise<PublicAskFAQ[]> {
  const res = await apiFetch(`/api/v1/public/links/${publicToken}/ask/faq`);
  if (!res.ok) throw new Error(`public ask FAQ failed: ${res.status} ${await res.text()}`);
  const body = (await res.json()) as { data: PublicAskFAQ[] };
  return body.data ?? [];
}

interface PublicFormalAsk {
  id: string;
  question: string;
  answer: string;
  published_at?: string;
  visitor_email?: string;
}

export async function listPublicFormalAsk(publicToken: string): Promise<PublicFormalAsk[]> {
  const res = await apiFetch(`/api/v1/public/links/${publicToken}/ask/formal`);
  if (!res.ok) throw new Error(`public formal ask failed: ${res.status} ${await res.text()}`);
  const body = (await res.json()) as { data: PublicFormalAsk[] };
  return body.data ?? [];
}

export async function publishFormalAskTurn(
  workspaceSlug: string,
  linkId: string,
  turnId: string,
  answer: string,
  opts?: { publishAt?: string; anonymize?: boolean },
): Promise<OwnerAskTurn> {
  const res = await apiFetch(
    `/api/workspaces/${workspaceSlug}/links/${linkId}/ask/${turnId}/formal-publish`,
    {
      method: "PATCH",
      body: JSON.stringify({
        answer,
        ...(opts?.publishAt ? { publish_at: opts.publishAt } : {}),
        ...(typeof opts?.anonymize === "boolean" ? { anonymize: opts.anonymize } : {}),
      }),
    },
  );
  if (!res.ok) throw new Error(`formal publish failed: ${res.status} ${await res.text()}`);
  const body = (await res.json()) as { data: OwnerAskTurn & { formal_status?: string } };
  return body.data;
}

export async function fetchDashboardActionItems(
  workspaceSlug: string,
): Promise<DashboardActionItem[]> {
  const res = await apiFetch(`/api/workspaces/${workspaceSlug}/dashboard/stats`);
  if (!res.ok) throw new Error(`dashboard stats failed: ${res.status} ${await res.text()}`);
  const body = (await res.json()) as { actionItems?: DashboardActionItem[] };
  return body.actionItems ?? [];
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
