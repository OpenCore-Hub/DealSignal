import { test, expect } from "@playwright/test";
import {
  seedContact,
  seedRealBackend,
  seedDocument,
  apiFetch,
  authenticatePage,
  attachDebug,
} from "./real-helpers";
import path from "path";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const FIXTURES_DIR = path.join(__dirname, "fixtures");

let seed: Awaited<ReturnType<typeof seedRealBackend>>;
let sharedDocumentId: string;

test.beforeAll(async () => {
  seed = await seedRealBackend();
  sharedDocumentId = (await seedDocument(seed.workspaceSlug)).id;
});

async function createLinkViaApi(
  workspaceSlug: string,
  documentId: string,
  permissionType?: string,
  opts: {
    requireEmail?: boolean;
    requirePassword?: boolean;
    requireNDA?: boolean;
    ndaDocumentId?: string;
    requireEmailVerification?: boolean;
    password?: string;
    allowedEmails?: string[];
    downloadEnabled?: boolean;
    expiresAt?: string;
    maxAccessCount?: number;
    download_enabled?: boolean;
  } = {}
): Promise<{ id: string; shortUrl: string }> {
  const body: Record<string, unknown> = {
    document_id: documentId,
    name: `E2E ${permissionType ?? "combined"} link`,
    download_enabled: opts.downloadEnabled ?? opts.download_enabled ?? true,
  };
  if (permissionType) body.permission_type = permissionType;
  if (opts.requireEmail) body.require_email = true;
  if (opts.requirePassword) body.require_password = true;
  if (opts.requireNDA) body.require_nda = true;
  if (opts.requireNDA && opts.ndaDocumentId) body.nda_document_id = opts.ndaDocumentId;
  if (opts.requireEmailVerification) body.require_email_verification = true;
  if (opts.password) body.password = opts.password;

  // Email verification (and NDA, which implies verification) requires at least one contact.
  if (opts.requireEmailVerification || opts.requireNDA) {
    const contactEmail = `contact-${Date.now()}@example.com`;
    const contact = await seedContact(workspaceSlug, contactEmail, "E2E Contact");
    body.contact_ids = [contact.id];
  }

  // Legacy permission_type shorthand: set the concrete boolean flags that the backend expects.
  if (permissionType === "password") {
    body.require_password = true;
  } else if (permissionType === "whitelist") {
    body.require_email = true;
  } else if (permissionType === "email" || permissionType === "email_required") {
    body.require_email = true;
  } else if (permissionType === "nda") {
    body.require_nda = true;
    body.require_email = true;
    if (opts.ndaDocumentId) body.nda_document_id = opts.ndaDocumentId;
  }
  if (opts.allowedEmails) body.allowed_emails = opts.allowedEmails;
  if (opts.expiresAt) body.expires_at = opts.expiresAt;
  if (typeof opts.maxAccessCount === "number") body.max_access_count = opts.maxAccessCount;

  const res = await apiFetch(`/api/workspaces/${workspaceSlug}/links`, {
    method: "POST",
    headers: { Origin: "http://localhost:5173" },
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
  gate: { email?: string; password?: string; nda?: boolean }
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
  if (gate.nda) {
    const ndaCheckbox = page.getByRole("checkbox", { name: /I agree to the non-disclosure agreement/i });
    await expect(ndaCheckbox).toBeVisible({ timeout: 30000 });
    await ndaCheckbox.check();
  }
  await page.getByRole("button", { name: "Continue" }).click();
  await expect(page.locator("img[alt*='Page']")).toBeVisible({ timeout: 30000 });
}

async function expectGateError(
  page: import("@playwright/test").Page,
  shareUrl: string,
  gate: { email?: string; password?: string; nda?: boolean },
  expectedText: RegExp
) {
  await page.goto(shareUrl);
  if (gate.email) {
    await expect(page.locator("#email")).toBeVisible({ timeout: 30000 });
    if (gate.email !== "") await page.locator("#email").fill(gate.email);
  }
  if (gate.password) {
    await expect(page.locator("#password")).toBeVisible({ timeout: 30000 });
    if (gate.password !== "") await page.locator("#password").fill(gate.password);
  }
  if (gate.nda) {
    const ndaCheckbox = page.getByRole("checkbox", { name: /I agree to the non-disclosure agreement/i });
    await expect(ndaCheckbox).toBeVisible({ timeout: 30000 });
    await ndaCheckbox.check();
  }
  await page.getByRole("button", { name: "Continue" }).click();
  await expect(page.getByText(expectedText)).toBeVisible({ timeout: 30000 });
}

async function revokeLinkViaApi(workspaceSlug: string, linkId: string) {
  const res = await apiFetch(`/api/workspaces/${workspaceSlug}/links/${linkId}`, {
    method: "PATCH",
    body: JSON.stringify({ status: "revoked" }),
  });
  if (!res.ok) {
    throw new Error(`revoke link failed: ${res.status} ${await res.text()}`);
  }
}

async function createDealRoomViaApi(
  workspaceSlug: string,
  name: string,
  roomSlug: string
): Promise<{ id: string; slug: string }> {
  const res = await apiFetch(`/api/workspaces/${workspaceSlug}/deal-rooms`, {
    method: "POST",
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
  workspaceSlug: string,
  roomId: string,
  documentId: string
) {
  const res = await apiFetch(`/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/documents`, {
    method: "POST",
    body: JSON.stringify({ document_id: documentId }),
  });
  if (!res.ok) {
    throw new Error(`add document to deal room failed: ${res.status} ${await res.text()}`);
  }
}

test.describe("real backend P0 flow", () => {
  test("workspace list loads the seeded workspace", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);
    await page.goto("/");

    // With a single workspace, the page auto-redirects to its dashboard.
    await expect(page).toHaveURL(new RegExp(`/${seed.workspaceSlug}/dashboard`), { timeout: 30000 });
  });

  test("upload page renders and accepts a file", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);
    await page.goto(`/${seed.workspaceSlug}/documents/upload`);
    await expect(page.getByRole("heading", { name: "Upload Document" })).toBeVisible({ timeout: 30000 });

    const fileInput = page.locator('[data-testid="file-upload"]');
    await fileInput.setInputFiles(path.join(FIXTURES_DIR, "sample.pdf"));

    await expect(page.getByText("sample.pdf")).toBeVisible();
    await page.getByRole("button", { name: "Upload now" }).click();

    // The uploader redirects to the documents list on completion.
    await expect(page).toHaveURL(new RegExp(`/${seed.workspaceSlug}/documents`), { timeout: 30000 });
    await expect(page.getByText("sample.pdf").first()).toBeVisible({ timeout: 30000 });
  });

  test("upload dialog from top nav uploads a file and document appears in the list", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);
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
    await authenticatePage(page);

    const documentId = sharedDocumentId;
    const link = await createLinkViaApi(seed.workspaceSlug, documentId, "public");

    // Open the public share URL in a fresh context (no auth).
    const visitorPage = await page.context().newPage();
    attachDebug(visitorPage);
    await visitorPage.goto(link.shortUrl);
    await expect(visitorPage.locator("img[alt*='Page']")).toBeVisible({ timeout: 30000 });
    await visitorPage.close();

    // Back in owner context, refresh the link detail and verify the view was recorded.
    await page.goto(`/${seed.workspaceSlug}/links`);
    await expect(page.getByText("sample.pdf").first()).toBeVisible({ timeout: 30000 });
    await expect(page.getByText(/\d+ (visits?|views?)/).first()).toBeVisible({ timeout: 30000 });
  });

  test("authenticated viewer reports page views to analytics", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);

    const documentId = sharedDocumentId;
    await createLinkViaApi(seed.workspaceSlug, documentId, "public");

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
        const res = await apiFetch(`/api/workspaces/${seed.workspaceSlug}/insights/pages/${documentId}`);
        const body = (await res.json()) as { data: { pageNumber: number; viewCount: number }[] };
        return body.data?.[0]?.viewCount ?? 0;
      },
      { timeout: 30000 }
    ).toBeGreaterThan(0);
  });

  test("create a deal room from a template and see it in the list", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);

    await page.goto(`/${seed.workspaceSlug}/deal-rooms/new`);
    await expect(page.getByRole("heading", { name: "New Deal Room" })).toBeVisible({ timeout: 30000 });

    // Templates should load.
    await expect(page.getByRole("button", { name: /Startup Fundraising/i }).first()).toBeVisible({ timeout: 30000 });

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
    await authenticatePage(page);

    const visitorEmail = `gate-test-${Date.now()}@example.com`;
    const link = await createLinkViaApi(seed.workspaceSlug, sharedDocumentId, "email_required");

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
    await authenticatePage(page);

    const password = `Secret-${Date.now()}`;
    const link = await createLinkViaApi(seed.workspaceSlug, sharedDocumentId, "password", { password });

    const visitorPage = await page.context().newPage();
    attachDebug(visitorPage);
    await openGatedPublicLink(visitorPage, link.shortUrl, { password });
    await expect(visitorPage.locator("img[alt*='Page']")).toBeVisible({ timeout: 30000 });
    await visitorPage.close();
  });

  test("whitelist link gate allows access for an allowed email", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);

    const visitorEmail = `whitelist-test-${Date.now()}@example.com`;
    const link = await createLinkViaApi(seed.workspaceSlug, sharedDocumentId, "whitelist", {
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
    await authenticatePage(page);

    const allowedEmail = `whitelist-allowed-${Date.now()}@example.com`;
    const link = await createLinkViaApi(seed.workspaceSlug, sharedDocumentId, "whitelist", {
      allowedEmails: [allowedEmail],
    });

    const visitorPage = await page.context().newPage();
    attachDebug(visitorPage);
    await visitorPage.goto(link.shortUrl);
    await expect(visitorPage.locator("#email")).toBeVisible({ timeout: 30000 });
    await visitorPage.locator("#email").fill(`blocked-${Date.now()}@example.com`);
    await visitorPage.getByRole("button", { name: "Continue" }).click();
    await expect(visitorPage.getByText(/email is not allowed/i)).toBeVisible({ timeout: 30000 });
    await visitorPage.close();
  });

  test("whitelist link works again after revoke and re-enable", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);

    const visitorEmail = `whitelist-revive-${Date.now()}@example.com`;
    const link = await createLinkViaApi(seed.workspaceSlug, sharedDocumentId, "whitelist", {
      allowedEmails: [visitorEmail],
    });

    const visitorPage = await page.context().newPage();
    attachDebug(visitorPage);

    // First access works.
    await openGatedPublicLink(visitorPage, link.shortUrl, { email: visitorEmail });
    await expect(visitorPage.locator("img[alt*='Page']")).toBeVisible({ timeout: 30000 });

    // Revoke and verify blocked in a fresh visitor context (no session reuse).
    await revokeLinkViaApi(seed.workspaceSlug, link.id);
    await visitorPage.close();
    const blockedPage = await page.context().newPage();
    attachDebug(blockedPage);
    await blockedPage.goto(link.shortUrl);
    await expect(blockedPage.getByText(/Link disabled/i)).toBeVisible({ timeout: 30000 });
    await blockedPage.close();

    // Re-enable and verify access again.
    await apiFetch(`/api/workspaces/${seed.workspaceSlug}/links/${link.id}`, {
      method: "PATCH",
      body: JSON.stringify({ status: "active" }),
    });
    const revivedPage = await page.context().newPage();
    attachDebug(revivedPage);
    await openGatedPublicLink(revivedPage, link.shortUrl, { email: visitorEmail });
    await expect(revivedPage.locator("img[alt*='Page']")).toBeVisible({ timeout: 30000 });
    await revivedPage.close();
  });

  test("whitelist link with expiry/max access stays valid after UI toggle disable/enable", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);

    const visitorEmail = `whitelist-toggle-${Date.now()}@example.com`;
    const expiresAt = new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString();
    const link = await createLinkViaApi(seed.workspaceSlug, sharedDocumentId, "whitelist", {
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
    await authenticatePage(page);

    const visitorEmail = `whitelist-open-tab-${Date.now()}@example.com`;
    const link = await createLinkViaApi(seed.workspaceSlug, sharedDocumentId, "whitelist", {
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
    await authenticatePage(page);

    const link = await createLinkViaApi(seed.workspaceSlug, sharedDocumentId, "public");
    await revokeLinkViaApi(seed.workspaceSlug, link.id);

    const visitorPage = await page.context().newPage();
    attachDebug(visitorPage);
    await visitorPage.goto(link.shortUrl);
    await expect(visitorPage.getByText(/Link disabled/i)).toBeVisible({ timeout: 30000 });
    await visitorPage.close();
  });

  test("public viewer download button downloads the document", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);

    const link = await createLinkViaApi(seed.workspaceSlug, sharedDocumentId, "public", {
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

  test("NDA link gate collects email and agreement", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);

    const visitorEmail = `nda-test-${Date.now()}@example.com`;
    const link = await createLinkViaApi(seed.workspaceSlug, sharedDocumentId, undefined, {
      requireEmail: true,
      requireNDA: true,
      ndaDocumentId: sharedDocumentId,
    });

    const visitorPage = await page.context().newPage();
    attachDebug(visitorPage);
    await openGatedPublicLink(visitorPage, link.shortUrl, { email: visitorEmail, nda: true });
    await expect(visitorPage.locator("img[alt*='Page']")).toBeVisible({ timeout: 30000 });
    await visitorPage.close();
  });

  test("combined email+password+NDA gate requires all credentials", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);

    const visitorEmail = `combined-test-${Date.now()}@example.com`;
    const link = await createLinkViaApi(seed.workspaceSlug, sharedDocumentId, undefined, {
      requireEmail: true,
      requirePassword: true,
      requireNDA: true,
      ndaDocumentId: sharedDocumentId,
      password: "Secret123!",
    });

    const visitorPage = await page.context().newPage();
    attachDebug(visitorPage);
    await openGatedPublicLink(visitorPage, link.shortUrl, {
      email: visitorEmail,
      password: "Secret123!",
      nda: true,
    });
    await expect(visitorPage.locator("img[alt*='Page']")).toBeVisible({ timeout: 30000 });
    await visitorPage.close();
  });

  test("password gate shows inline error for wrong password and allows retry", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);

    const link = await createLinkViaApi(seed.workspaceSlug, sharedDocumentId, "password", {
      password: "Secret123!",
    });

    const visitorPage = await page.context().newPage();
    attachDebug(visitorPage);
    await expectGateError(visitorPage, link.shortUrl, { password: "wrong" }, /invalid password/i);

    await visitorPage.locator("#password").fill("Secret123!");
    await visitorPage.getByRole("button", { name: "Continue" }).click();
    await expect(visitorPage.locator("img[alt*='Page']")).toBeVisible({ timeout: 30000 });
    await visitorPage.close();
  });

  test("whitelist gate shows inline error for denied email and allows retry", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);

    const allowedEmail = `allowed-${Date.now()}@example.com`;
    const link = await createLinkViaApi(seed.workspaceSlug, sharedDocumentId, "whitelist", {
      allowedEmails: [allowedEmail],
    });

    const visitorPage = await page.context().newPage();
    attachDebug(visitorPage);
    await expectGateError(
      visitorPage,
      link.shortUrl,
      { email: `blocked-${Date.now()}@example.com` },
      /email is not allowed/i
    );

    await visitorPage.locator("#email").fill(allowedEmail);
    await visitorPage.getByRole("button", { name: "Continue" }).click();
    await expect(visitorPage.locator("img[alt*='Page']")).toBeVisible({ timeout: 30000 });
    await visitorPage.close();
  });

  test("max access count blocks further public access", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);

    const link = await createLinkViaApi(seed.workspaceSlug, sharedDocumentId, "public", {
      maxAccessCount: 1,
    });

    const visitorPage = await page.context().newPage();
    attachDebug(visitorPage);
    await visitorPage.goto(link.shortUrl);
    await expect(visitorPage.locator("img[alt*='Page']")).toBeVisible({ timeout: 30000 });
    await visitorPage.close();

    // A fresh visitor context should hit the access limit.
    const blockedPage = await page.context().newPage();
    attachDebug(blockedPage);
    await blockedPage.goto(link.shortUrl);
    await expect(blockedPage.getByText(/Access limit reached/i)).toBeVisible({ timeout: 30000 });
    await blockedPage.close();
  });

  test("expired link returns gone error", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);

    const expiresAt = new Date(Date.now() - 60 * 60 * 1000).toISOString();
    const link = await createLinkViaApi(seed.workspaceSlug, sharedDocumentId, "public", {
      expiresAt,
    });

    const visitorPage = await page.context().newPage();
    attachDebug(visitorPage);
    await visitorPage.goto(link.shortUrl);
    await expect(visitorPage.getByText(/link expired/i)).toBeVisible({ timeout: 30000 });
    await visitorPage.close();
  });

  test("insights overview shows top documents and links", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);

    await page.goto(`/${seed.workspaceSlug}/insights/overview`);
    await expect(page.getByText("Top documents")).toBeVisible({ timeout: 30000 });
    await expect(page.getByText("Top links")).toBeVisible({ timeout: 30000 });
    await expect(page.getByText("sample.pdf").first()).toBeVisible({ timeout: 30000 });
  });

  test("insights page engagement loads document analytics", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);

    await page.goto(`/${seed.workspaceSlug}/insights/pages`);
    await expect(page.getByRole("heading", { name: "Page engagement" })).toBeVisible({ timeout: 30000 });
    await page.getByRole("combobox").click();
    await expect(page.getByRole("option", { name: "sample.pdf" }).first()).toBeVisible({ timeout: 30000 });
  });

  test("settings subpages render", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);

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
    await authenticatePage(page);

    const roomSlug = `e2e-room-${Date.now()}`;
    const room = await createDealRoomViaApi(seed.workspaceSlug, "E2E Room With Doc", roomSlug);
    await addDocumentToDealRoomViaApi(seed.workspaceSlug, room.id, sharedDocumentId);

    await page.goto(`/${seed.workspaceSlug}/deal-rooms/${room.id}`);
    await expect(page.getByRole("heading", { name: "E2E Room With Doc" })).toBeVisible({ timeout: 30000 });
    await expect(page.getByText("sample.pdf").first()).toBeVisible({ timeout: 30000 });
  });
});
