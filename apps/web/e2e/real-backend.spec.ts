import { test, expect } from "@playwright/test";
import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const API_BASE_URL = process.env.REAL_API_BASE_URL || "http://localhost:8080";
const FIXTURES_DIR = path.join(__dirname, "fixtures");

interface SeedResult {
  token: string;
  workspaceSlug: string;
}

async function uploadFixtureViaApi(token: string, workspaceSlug: string): Promise<string> {
  const filePath = path.join(FIXTURES_DIR, "sample.pdf");
  const buffer = fs.readFileSync(filePath);
  const file = new File([buffer], "sample.pdf", { type: "application/pdf" });
  const form = new FormData();
  form.append("file", file);

  const res = await fetch(`${API_BASE_URL}/api/workspaces/${workspaceSlug}/documents`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: form,
  });
  if (!res.ok) {
    throw new Error(`api upload failed: ${res.status} ${await res.text()}`);
  }

  await expect.poll(
    async () => {
      const listRes = await apiFetch(`/api/workspaces/${workspaceSlug}/documents`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const body = (await listRes.json()) as { data: { id: string; title: string; status: string }[] };
      return body.data.find((d) => d.title === "sample.pdf" && d.status === "ready")?.id ?? null;
    },
    { timeout: 30000 }
  ).toBeTruthy();

  const listRes = await apiFetch(`/api/workspaces/${workspaceSlug}/documents`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  const body = (await listRes.json()) as { data: { id: string; title: string; status: string }[] };
  const doc = body.data.find((d) => d.title === "sample.pdf" && d.status === "ready");
  if (!doc) throw new Error("shared document not found after polling");
  return doc.id;
}

async function apiFetch(input: string, init?: RequestInit): Promise<Response> {
  return fetch(`${API_BASE_URL}${input}`, {
    headers: { "Content-Type": "application/json", ...init?.headers },
    ...init,
  });
}

async function seedRealBackend(): Promise<SeedResult> {
  const timestamp = Date.now();
  const email = `e2e-${timestamp}@example.com`;
  const password = "Password123!";

  const registerRes = await apiFetch("/api/auth/register", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
  if (!registerRes.ok) {
    throw new Error(`register failed: ${registerRes.status} ${await registerRes.text()}`);
  }
  const { access_token: token } = (await registerRes.json()) as { access_token: string };

  const workspaceSlug = `e2e-${timestamp}`;
  const workspaceRes = await apiFetch("/api/workspaces", {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: JSON.stringify({ name: "E2E Workspace", slug: workspaceSlug, brand_color: "#0055ff" }),
  });
  if (!workspaceRes.ok) {
    throw new Error(`workspace create failed: ${workspaceRes.status} ${await workspaceRes.text()}`);
  }

  return { token, workspaceSlug };
}

let seed: SeedResult;
let sharedDocumentId: string;

test.beforeAll(async () => {
  seed = await seedRealBackend();
  sharedDocumentId = await uploadFixtureViaApi(seed.token, seed.workspaceSlug);
});

async function authenticate(page: import("@playwright/test").Page, token: string) {
  await page.addInitScript((t: string) => {
    localStorage.setItem("access_token", t);
  }, token);
}

function attachDebug(page: import("@playwright/test").Page) {
  page.on("console", (msg) => {
    console.log(`[browser ${msg.type()}] ${msg.text()}`);
  });
  page.on("pageerror", (err) => {
    console.log(`[browser error] ${err.message}`);
  });
  page.on("response", (response) => {
    if (response.status() === 403) {
      console.log(`[browser 403] ${response.url()}`);
    }
  });
}

async function createLinkViaApi(
  token: string,
  workspaceSlug: string,
  documentId: string,
  permissionType: string,
  opts: {
    password?: string;
    allowedEmails?: string[];
    allowedDomains?: string[];
    downloadEnabled?: boolean;
    expiresAt?: string;
    maxAccessCount?: number;
  } = {}
): Promise<{ id: string; shortUrl: string }> {
  const body: Record<string, unknown> = {
    document_id: documentId,
    name: `E2E ${permissionType} link`,
    permission_type: permissionType,
    download_enabled: opts.downloadEnabled ?? true,
  };
  if (opts.password) body.password = opts.password;
  if (opts.allowedEmails) body.allowed_emails = opts.allowedEmails;
  if (opts.allowedDomains) body.allowed_domains = opts.allowedDomains;
  if (opts.expiresAt) body.expires_at = opts.expiresAt;
  if (typeof opts.maxAccessCount === "number") body.max_access_count = opts.maxAccessCount;

  const res = await apiFetch(`/api/workspaces/${workspaceSlug}/links`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}`, Origin: "http://localhost:5173" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    throw new Error(`create link failed: ${res.status} ${await res.text()}`);
  }
  const data = (await res.json()) as { id: string; shortUrl: string };
  return data;
}

async function openGatedPublicLink(
  page: import("@playwright/test").Page,
  shareUrl: string,
  gate: { email?: string; password?: string }
) {
  await page.goto(shareUrl);
  if (gate.email) {
    await expect(page.locator("#email")).toBeVisible({ timeout: 30000 });
    await page.locator("#email").fill(gate.email);
  }
  if (gate.password) {
    await expect(page.locator("#password")).toBeVisible({ timeout: 30000 });
    await page.locator("#password").fill(gate.password);
  }
  await page.getByRole("button", { name: "Continue" }).click();
  await expect(page.locator("img[alt*='Page']")).toBeVisible({ timeout: 30000 });
}

async function revokeLinkViaApi(token: string, workspaceSlug: string, linkId: string) {
  const res = await apiFetch(`/api/workspaces/${workspaceSlug}/links/${linkId}`, {
    method: "PATCH",
    headers: { Authorization: `Bearer ${token}` },
    body: JSON.stringify({ status: "revoked" }),
  });
  if (!res.ok) {
    throw new Error(`revoke link failed: ${res.status} ${await res.text()}`);
  }
}

async function createDealRoomViaApi(
  token: string,
  workspaceSlug: string,
  name: string,
  roomSlug: string
): Promise<{ id: string; slug: string }> {
  const res = await apiFetch(`/api/workspaces/${workspaceSlug}/deal-rooms`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: JSON.stringify({
      name,
      slug: roomSlug,
      description: "E2E deal room with document",
      template_type: "seed",
    }),
  });
  if (!res.ok) {
    throw new Error(`create deal room failed: ${res.status} ${await res.text()}`);
  }
  const data = (await res.json()) as { id: string; slug: string };
  return data;
}

async function addDocumentToDealRoomViaApi(
  token: string,
  workspaceSlug: string,
  roomId: string,
  documentId: string
) {
  const res = await apiFetch(`/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/documents`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: JSON.stringify({ document_id: documentId }),
  });
  if (!res.ok) {
    throw new Error(`add document to deal room failed: ${res.status} ${await res.text()}`);
  }
}

test.describe("real backend P0 flow", () => {
  test("workspace list loads the seeded workspace", async ({ page }) => {
    attachDebug(page);
    await authenticate(page, seed.token);
    await page.goto("/");

    // With a single workspace, the page auto-redirects to its dashboard.
    await expect(page).toHaveURL(new RegExp(`/${seed.workspaceSlug}/dashboard`), { timeout: 30000 });
  });

  test("upload page renders and accepts a file", async ({ page }) => {
    attachDebug(page);
    await authenticate(page, seed.token);
    await page.goto(`/${seed.workspaceSlug}/documents/upload`);
    await expect(page.getByRole("heading", { name: "Upload Document" })).toBeVisible({ timeout: 30000 });

    const fileInput = page.locator('[data-testid="file-upload"]');
    await fileInput.setInputFiles(path.join(FIXTURES_DIR, "sample.pdf"));

    await expect(page.getByText("sample.pdf")).toBeVisible();
    await page.getByRole("button", { name: "Upload now" }).click();
    await expect(page.getByTestId("upload-success")).toBeVisible({ timeout: 30000 });
  });

  test("upload dialog from top nav uploads a file and document appears in the list", async ({ page }) => {
    attachDebug(page);
    await authenticate(page, seed.token);
    await page.goto(`/${seed.workspaceSlug}/dashboard`);

    await page.getByRole("button", { name: "Upload document" }).click();

    const dialog = page.getByRole("dialog", { name: "Upload Document" });
    await expect(dialog).toBeVisible({ timeout: 30000 });

    const fileInput = dialog.locator('[data-testid="file-upload"]');
    await fileInput.setInputFiles(path.join(FIXTURES_DIR, "sample.pdf"));

    await expect(dialog.getByText("sample.pdf")).toBeVisible();
    await dialog.getByRole("button", { name: "Upload now" }).click();
    await expect(dialog.getByTestId("upload-success")).toBeVisible({ timeout: 30000 });

    await page.goto(`/${seed.workspaceSlug}/documents`);
    await expect(page.getByText("sample.pdf").first()).toBeVisible({ timeout: 30000 });
  });

  test("create a link for a document and open the public share URL", async ({ page }) => {
    attachDebug(page);
    await authenticate(page, seed.token);

    const documentId = sharedDocumentId;
    await page.goto(`/${seed.workspaceSlug}/documents/${documentId}`);
    await expect(page.getByRole("button", { name: "Create link" })).toBeVisible({ timeout: 30000 });
    await page.getByRole("button", { name: "Create link" }).click();

    await expect(page).toHaveURL(new RegExp(`/${seed.workspaceSlug}/links/new`), { timeout: 30000 });
    await page.getByTestId("create-link-button").click();

    await expect(page.getByTestId("generated-link")).toBeVisible({ timeout: 30000 });

    const shareUrl = await page.getByTestId("generated-link").textContent();
    expect(shareUrl).toBeTruthy();

    // Open the public share URL in a fresh context (no auth).
    const visitorPage = await page.context().newPage();
    attachDebug(visitorPage);
    await visitorPage.goto(shareUrl!);
    await expect(visitorPage.locator("img[alt*='Page']")).toBeVisible({ timeout: 30000 });
    await visitorPage.close();

    // Back in owner context, refresh the link detail and verify the view was recorded.
    await page.goto(`/${seed.workspaceSlug}/links`);
    await expect(page.getByText("sample.pdf").first()).toBeVisible({ timeout: 30000 });
    await expect(page.getByText(/\d+ visits?/).first()).toBeVisible({ timeout: 30000 });
  });

  test("authenticated viewer reports page views to analytics", async ({ page }) => {
    attachDebug(page);
    await authenticate(page, seed.token);

    const documentId = sharedDocumentId;
    await page.goto(`/${seed.workspaceSlug}/documents/${documentId}`);
    await expect(page.getByRole("button", { name: "Create link" })).toBeVisible({ timeout: 30000 });
    await page.getByRole("button", { name: "Create link" }).click();
    await expect(page).toHaveURL(new RegExp(`/${seed.workspaceSlug}/links/new`), { timeout: 30000 });
    await page.getByTestId("create-link-button").click();
    await expect(page.getByTestId("generated-link")).toBeVisible({ timeout: 30000 });

    // Open the document detail inside the workspace shell so the current workspace
    // is selected, then click Preview to open the authenticated viewer.
    await page.goto(`/${seed.workspaceSlug}/documents/${documentId}`);
    await page.getByRole("button", { name: "Preview" }).first().click();
    await expect(page).toHaveURL(/\/viewer\//, { timeout: 30000 });
    await expect(page.locator("img[alt*='Page']")).toBeVisible({ timeout: 30000 });
    // The authenticated viewer reports the page view after a short dwell time.
    await page.waitForTimeout(3000);

    // Poll the analytics API until the view is recorded.
    await expect.poll(
      async () => {
        const res = await apiFetch(`/api/workspaces/${seed.workspaceSlug}/insights/pages/${documentId}`, {
          headers: { Authorization: `Bearer ${seed.token}` },
        });
        const body = (await res.json()) as { data: { pageNumber: number; viewCount: number }[] };
        return body.data?.[0]?.viewCount ?? 0;
      },
      { timeout: 30000 }
    ).toBeGreaterThan(0);
  });

  test("create a deal room from a template and see it in the list", async ({ page }) => {
    attachDebug(page);
    await authenticate(page, seed.token);

    await page.goto(`/${seed.workspaceSlug}/deal-rooms/new`);
    await expect(page.getByRole("heading", { name: "New Deal Room" })).toBeVisible({ timeout: 30000 });

    // Templates should load.
    await expect(page.getByText("Seed Round Due Diligence")).toBeVisible({ timeout: 30000 });

    const roomName = `E2E Room ${Date.now()}`;
    await page.getByLabel("Name").fill(roomName);
    await page.getByLabel("Description").fill("End-to-end test deal room");

    await page.getByRole("button", { name: "Create deal room" }).click();

    // Should redirect to the new room detail.
    await expect(page).toHaveURL(new RegExp(`/${seed.workspaceSlug}/deal-rooms/`), { timeout: 30000 });
    await expect(page.getByRole("heading", { name: roomName })).toBeVisible({ timeout: 30000 });

    // Should appear in the deal room list.
    await page.goto(`/${seed.workspaceSlug}/deal-rooms`);
    await expect(page.getByText(roomName)).toBeVisible({ timeout: 30000 });
  });

  test("email-required link gate collects email and creates a contact", async ({ page }) => {
    attachDebug(page);
    await authenticate(page, seed.token);

    const visitorEmail = `gate-test-${Date.now()}@example.com`;
    const link = await createLinkViaApi(seed.token, seed.workspaceSlug, sharedDocumentId, "email_required");

    const visitorPage = await page.context().newPage();
    attachDebug(visitorPage);
    await openGatedPublicLink(visitorPage, link.shortUrl, { email: visitorEmail });
    // Let CanvasViewer report the page view.
    await visitorPage.waitForTimeout(3500);
    await visitorPage.close();

    // Contact should appear in the workspace list.
    await page.goto(`/${seed.workspaceSlug}/contacts`);
    await expect(page.getByText(visitorEmail)).toBeVisible({ timeout: 30000 });

    // Navigate to contact detail and verify the timeline shows an open event.
    await page.getByText(visitorEmail).click();
    await expect(page).toHaveURL(new RegExp(`/${seed.workspaceSlug}/contacts/`), { timeout: 30000 });
    await expect(page.getByRole("tab", { name: "Timeline" })).toBeVisible();
    await page.getByRole("tab", { name: "Timeline" }).click();
    await expect(page.getByText("opened the document").first()).toBeVisible({ timeout: 30000 });
  });

  test("password-protected link gate allows access with the correct password", async ({ page }) => {
    attachDebug(page);
    await authenticate(page, seed.token);

    const password = `Secret-${Date.now()}`;
    const link = await createLinkViaApi(seed.token, seed.workspaceSlug, sharedDocumentId, "password", { password });

    const visitorPage = await page.context().newPage();
    attachDebug(visitorPage);
    await openGatedPublicLink(visitorPage, link.shortUrl, { password });
    await expect(visitorPage.locator("img[alt*='Page']")).toBeVisible({ timeout: 30000 });
    await visitorPage.close();
  });

  test("whitelist link gate allows access for an allowed email", async ({ page }) => {
    attachDebug(page);
    await authenticate(page, seed.token);

    const visitorEmail = `whitelist-test-${Date.now()}@example.com`;
    const link = await createLinkViaApi(seed.token, seed.workspaceSlug, sharedDocumentId, "whitelist", {
      allowedEmails: [visitorEmail],
    });

    const visitorPage = await page.context().newPage();
    attachDebug(visitorPage);
    await openGatedPublicLink(visitorPage, link.shortUrl, { email: visitorEmail });
    await expect(visitorPage.locator("img[alt*='Page']")).toBeVisible({ timeout: 30000 });
    await visitorPage.close();
  });

  test("whitelist link gate denies a non-whitelisted email", async ({ page }) => {
    attachDebug(page);
    await authenticate(page, seed.token);

    const allowedEmail = `whitelist-allowed-${Date.now()}@example.com`;
    const link = await createLinkViaApi(seed.token, seed.workspaceSlug, sharedDocumentId, "whitelist", {
      allowedEmails: [allowedEmail],
    });

    const visitorPage = await page.context().newPage();
    attachDebug(visitorPage);
    await visitorPage.goto(link.shortUrl);
    await expect(visitorPage.locator("#email")).toBeVisible({ timeout: 30000 });
    await visitorPage.locator("#email").fill(`blocked-${Date.now()}@example.com`);
    await visitorPage.getByRole("button", { name: "Continue" }).click();
    await expect(visitorPage.getByText(/email not in whitelist/i)).toBeVisible({ timeout: 30000 });
    await visitorPage.close();
  });

  test("whitelist link works again after revoke and re-enable", async ({ page }) => {
    attachDebug(page);
    await authenticate(page, seed.token);

    const visitorEmail = `whitelist-revive-${Date.now()}@example.com`;
    const link = await createLinkViaApi(seed.token, seed.workspaceSlug, sharedDocumentId, "whitelist", {
      allowedEmails: [visitorEmail],
    });

    const visitorPage = await page.context().newPage();
    attachDebug(visitorPage);

    // First access works.
    await openGatedPublicLink(visitorPage, link.shortUrl, { email: visitorEmail });
    await expect(visitorPage.locator("img[alt*='Page']")).toBeVisible({ timeout: 30000 });

    // Revoke and verify blocked.
    await revokeLinkViaApi(seed.token, seed.workspaceSlug, link.id);
    await visitorPage.reload();
    await expect(visitorPage.getByText(/link revoked/i)).toBeVisible({ timeout: 30000 });

    // Re-enable and verify access again.
    await apiFetch(`/api/workspaces/${seed.workspaceSlug}/links/${link.id}`, {
      method: "PATCH",
      headers: { Authorization: `Bearer ${seed.token}` },
      body: JSON.stringify({ status: "active" }),
    });
    await visitorPage.goto(link.shortUrl);
    await expect(visitorPage.locator("#email")).toBeVisible({ timeout: 30000 });
    await visitorPage.locator("#email").fill(visitorEmail);
    await visitorPage.getByRole("button", { name: "Continue" }).click();
    await expect(visitorPage.locator("img[alt*='Page']")).toBeVisible({ timeout: 30000 });
    await visitorPage.close();
  });

  test("whitelist link with expiry/max access stays valid after UI toggle disable/enable", async ({ page }) => {
    attachDebug(page);
    await authenticate(page, seed.token);

    const visitorEmail = `whitelist-toggle-${Date.now()}@example.com`;
    const expiresAt = new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString();
    const link = await createLinkViaApi(seed.token, seed.workspaceSlug, sharedDocumentId, "whitelist", {
      allowedEmails: [visitorEmail],
      expiresAt,
      maxAccessCount: 5,
      downloadEnabled: true,
    });

    // First access works.
    const visitorPage = await page.context().newPage();
    attachDebug(visitorPage);
    await openGatedPublicLink(visitorPage, link.shortUrl, { email: visitorEmail });
    await expect(visitorPage.locator("img[alt*='Page']")).toBeVisible({ timeout: 30000 });
    await visitorPage.close();

    // Disable and re-enable via the link detail page toggle.
    await page.goto(`/${seed.workspaceSlug}/links/${link.id}`);
    const toggleButton = page.getByRole("button", { name: /^(Enabled|Disabled)$/ });
    await expect(toggleButton).toBeVisible({ timeout: 30000 });
    await toggleButton.click();
    // After disabling, the button label flips to "Enabled".
    await expect(page.getByRole("button", { name: "Enabled" })).toBeVisible({ timeout: 30000 });
    await page.getByRole("button", { name: "Enabled" }).click();
    await expect(page.getByRole("button", { name: "Disabled" })).toBeVisible({ timeout: 30000 });

    // Access again with the allowed email.
    const visitorPage2 = await page.context().newPage();
    attachDebug(visitorPage2);
    await openGatedPublicLink(visitorPage2, link.shortUrl, { email: visitorEmail });
    await expect(visitorPage2.locator("img[alt*='Page']")).toBeVisible({ timeout: 30000 });
    await visitorPage2.close();
  });

  test("whitelist link stays valid when viewer tab is open during toggle disable/enable", async ({ page }) => {
    attachDebug(page);
    await authenticate(page, seed.token);

    const visitorEmail = `whitelist-open-tab-${Date.now()}@example.com`;
    const link = await createLinkViaApi(seed.token, seed.workspaceSlug, sharedDocumentId, "whitelist", {
      allowedEmails: [visitorEmail],
    });

    // Open viewer in one tab and authenticate in another.
    const viewerTab = await page.context().newPage();
    attachDebug(viewerTab);
    await openGatedPublicLink(viewerTab, link.shortUrl, { email: visitorEmail });
    await expect(viewerTab.locator("img[alt*='Page']")).toBeVisible({ timeout: 30000 });

    // In the authenticated tab, disable and re-enable the link via detail page.
    await page.goto(`/${seed.workspaceSlug}/links/${link.id}`);
    await page.getByRole("button", { name: "Disabled" }).click();
    await expect(page.getByRole("button", { name: "Enabled" })).toBeVisible({ timeout: 30000 });
    await page.getByRole("button", { name: "Enabled" }).click();
    await expect(page.getByRole("button", { name: "Disabled" })).toBeVisible({ timeout: 30000 });

    // Back in the viewer tab, retry access with the allowed email.
    await viewerTab.goto(link.shortUrl);
    await expect(viewerTab.locator("#email")).toBeVisible({ timeout: 30000 });
    await viewerTab.locator("#email").fill(visitorEmail);
    await viewerTab.getByRole("button", { name: "Continue" }).click();
    await expect(viewerTab.locator("img[alt*='Page']")).toBeVisible({ timeout: 30000 });
    await viewerTab.close();
  });

  test("revoking a link blocks public access", async ({ page }) => {
    attachDebug(page);
    await authenticate(page, seed.token);

    const link = await createLinkViaApi(seed.token, seed.workspaceSlug, sharedDocumentId, "public");
    await revokeLinkViaApi(seed.token, seed.workspaceSlug, link.id);

    const visitorPage = await page.context().newPage();
    attachDebug(visitorPage);
    await visitorPage.goto(link.shortUrl);
    await expect(visitorPage.getByText(/link revoked/i)).toBeVisible({ timeout: 30000 });
    await visitorPage.close();
  });

  test("public viewer download button downloads the document", async ({ page }) => {
    attachDebug(page);
    await authenticate(page, seed.token);

    const link = await createLinkViaApi(seed.token, seed.workspaceSlug, sharedDocumentId, "public", {
      download_enabled: true,
    });

    const visitorPage = await page.context().newPage();
    attachDebug(visitorPage);
    await visitorPage.goto(link.shortUrl);
    await expect(visitorPage.locator("img[alt*='Page']")).toBeVisible({ timeout: 30000 });

    const [download] = await Promise.all([
      visitorPage.waitForEvent("download"),
      visitorPage.getByRole("button", { name: "Download" }).click(),
    ]);
    expect(download.suggestedFilename()).toBe("sample.pdf");
    await visitorPage.close();
  });

  test("insights overview shows top documents and links", async ({ page }) => {
    attachDebug(page);
    await authenticate(page, seed.token);

    await page.goto(`/${seed.workspaceSlug}/insights/overview`);
    await expect(page.getByText("Top documents")).toBeVisible({ timeout: 30000 });
    await expect(page.getByText("Top links")).toBeVisible({ timeout: 30000 });
    await expect(page.getByText("sample.pdf").first()).toBeVisible({ timeout: 30000 });
  });

  test("insights page engagement loads document analytics", async ({ page }) => {
    attachDebug(page);
    await authenticate(page, seed.token);

    await page.goto(`/${seed.workspaceSlug}/insights/pages`);
    await expect(page.getByText("Page engagement")).toBeVisible({ timeout: 30000 });
    await page.getByRole("combobox").click();
    await expect(page.getByRole("option", { name: "sample.pdf" }).first()).toBeVisible({ timeout: 30000 });
  });

  test("settings subpages render", async ({ page }) => {
    attachDebug(page);
    await authenticate(page, seed.token);

    const cases = [
      { path: "general", heading: "Workspace" },
      { path: "brand", heading: "Brand Customization" },
      { path: "security", heading: "Security" },
      { path: "members", heading: "Members" },
      { path: "integrations", heading: "Integrations" },
      { path: "billing", heading: "Subscription & Usage" },
      { path: "language", heading: "Language" },
    ];

    for (const c of cases) {
      await page.goto(`/${seed.workspaceSlug}/settings/${c.path}`);
      await expect(page.getByText(c.heading).first()).toBeVisible({ timeout: 30000 });
    }
  });

  test("deal room detail shows a document added via API", async ({ page }) => {
    attachDebug(page);
    await authenticate(page, seed.token);

    const roomSlug = `e2e-room-${Date.now()}`;
    const room = await createDealRoomViaApi(seed.token, seed.workspaceSlug, "E2E Room With Doc", roomSlug);
    await addDocumentToDealRoomViaApi(seed.token, seed.workspaceSlug, room.id, sharedDocumentId);

    await page.goto(`/${seed.workspaceSlug}/deal-rooms/${room.id}`);
    await expect(page.getByRole("heading", { name: "E2E Room With Doc" })).toBeVisible({ timeout: 30000 });
    const docsCard = page.locator('[data-slot="card"]', { hasText: "Documents" });
    await expect(docsCard).toBeVisible({ timeout: 30000 });
    await expect(docsCard.getByText("1")).toBeVisible({ timeout: 30000 });
  });
});
