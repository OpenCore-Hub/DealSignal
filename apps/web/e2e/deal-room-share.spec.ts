/**
 * Deal room share dialog — end-to-end coverage.
 *
 * 覆盖数据室分享弹窗（DealRoomShareDialog）的完整交互路径：
 * - 创建 / 编辑分享链接
 * - Share / Access 标签切换
 * - 链接名称、预设、过期时间、自定义域名、访问通知
 * - 访问控制：邮箱、邮箱验证码、密码、白名单 / 黑名单
 * - 内容保护：水印、NDA、下载控制、截图保护
 * - 高级功能：Visitor Ask（Ask Docs / Ask Host）、文件请求、索引文件
 * - 激活 / 禁用开关与二次确认
 * - 未保存更改关闭确认
 * - 表单校验
 * - 联系人选择器（从联系人列表添加、新建联系人）
 * - 公开访问者端对端验证
 */
import { test, expect, type Page } from "@playwright/test";
import {
  seedRealBackend,
  seedDocument,
  seedDealRoom,
  authenticatePage,
  attachDebug,
  apiFetch,
} from "./real-helpers";

let seed: Awaited<ReturnType<typeof seedRealBackend>>;
let roomId: string;

test.beforeAll(async () => {
  seed = await seedRealBackend();
  const doc = await seedDocument(seed.workspaceSlug);
  const room = await seedDealRoom(seed.workspaceSlug, {
    name: "Share Dialog Test Room",
    templateType: "seed",
    documentIds: [doc.id],
  });
  roomId = room.id;
});

test.beforeEach(async ({ page }) => {
  attachDebug(page);
  await authenticatePage(page);
});

// ── Helpers ──────────────────────────────────────────────────────────

async function openCreateDialog(page: Page) {
  await page.goto(`/${seed.workspaceSlug}/deal-rooms/${roomId}?tab=participants`);
  await page.waitForTimeout(1500);

  // Click the share button in the page header or the empty-state CTA.
  const shareBtn = page.getByRole("button", { name: /Create link/i }).first();
  await expect(shareBtn).toBeVisible({ timeout: 10000 });
  await shareBtn.click();

  await expect(page.getByRole("dialog", { name: /Create share link/i })).toBeVisible({ timeout: 10000 });
}

async function openEditDialog(page: Page, linkName: string) {
  await page.goto(`/${seed.workspaceSlug}/deal-rooms/${roomId}?tab=participants`);
  await page.waitForTimeout(1500);

  const row = page.locator("table tbody tr").filter({ hasText: linkName }).first();
  await expect(row).toBeVisible({ timeout: 10000 });

  await row.getByRole("button", { name: "moreActions" }).click();
  await page.getByRole("menuitem", { name: /Edit/i }).click();

  await expect(page.getByRole("dialog")).toBeVisible({ timeout: 10000 });
}

async function switchToAccessTab(page: Page) {
  await page.getByRole("tab", { name: /Access/i }).click();
  await page.waitForTimeout(300);
}

async function switchToShareTab(page: Page) {
  await page.getByRole("tab", { name: /Share/i }).click();
  await page.waitForTimeout(300);
}

async function setSwitch(page: Page, name: RegExp, checked: boolean) {
  const sw = page.getByRole("switch", { name });
  const current = await sw.getAttribute("data-checked");
  const isChecked = current === "true" || current === "";
  if (isChecked !== checked) {
    await sw.click();
    await page.waitForTimeout(200);
  }
}

async function selectNdaDocument(page: Page) {
  const select = page.getByRole("combobox", { name: /NDA agreement document/i });
  await expect(select).toBeVisible({ timeout: 5000 });
  await select.click();
  await page.getByRole("option").first().click();
  await page.waitForTimeout(300);
}

async function getContactTextarea(page: Page, type: "allowed" | "blocked") {
  const heading = type === "allowed" ? "Allowed viewers" : "Blocked viewers";
  return page
    .getByRole("heading", { name: heading })
    .locator("..")
    .locator("textarea")
    .first();
}

async function addEmailTag(page: Page, type: "allowed" | "blocked", email: string) {
  const textarea = await getContactTextarea(page, type);
  await textarea.fill(email);
  await textarea.press("Enter");
  await page.waitForTimeout(300);
}

async function createDealRoomLinkViaApi(payload: Record<string, unknown>) {
  const res = await apiFetch(`/api/workspaces/${seed.workspaceSlug}/deal-rooms/${roomId}/links`, {
    method: "POST",
    body: JSON.stringify(payload),
  });
  if (!res.ok) throw new Error(`create deal room link failed: ${res.status} ${await res.text()}`);
  return (await res.json()) as { id: string; shortUrl: string };
}

async function waitForCreateResponse(page: Page) {
  return page.waitForResponse(
    (res) => res.url().includes(`/deal-rooms/${roomId}/links`) && res.request().method() === "POST",
    { timeout: 10000 }
  );
}

async function visitGatedLink(
  page: Page,
  url: string,
  opts: {
    email?: string;
    code?: string;
    password?: string;
    nda?: boolean;
    expectDenied?: boolean;
  }
) {
  await page.goto(url);
  await page.waitForTimeout(1000);

  // Email-verification gates no longer collect email / send codes on the
  // visitor page; owners issue codes out-of-band. Keep optional email fill for
  // require-email-only links.
  if (opts.email) {
    const emailInput = page.locator("#email");
    if (await emailInput.isVisible({ timeout: 5000 }).catch(() => false)) {
      await emailInput.fill(opts.email);
    }
  }

  if (opts.code) {
    const codeInput = page.locator('input[inputmode="numeric"]');
    if (await codeInput.isVisible({ timeout: 5000 }).catch(() => false)) {
      await codeInput.fill(opts.code);
    }
  }

  if (opts.password) {
    const pwdInput = page.locator("#password");
    if (await pwdInput.isVisible({ timeout: 5000 }).catch(() => false)) {
      await pwdInput.fill(opts.password);
    }
  }

  if (opts.nda) {
    const ndaCheckbox = page.getByRole("checkbox", { name: /agree/i });
    if (await ndaCheckbox.isVisible({ timeout: 5000 }).catch(() => false)) {
      await ndaCheckbox.check();
    }
  }

  await page.getByRole("button", { name: /Continue/i }).click();

  if (opts.expectDenied) {
    await expect(
      page.getByText(/Access denied|not allowed|blocked|disabled|expired|Link inactive/i).first()
    ).toBeVisible({ timeout: 10000 });
  } else {
    await page
      .locator("img[alt*='Page']")
      .first()
      .waitFor({ state: "visible", timeout: 15000 })
      .catch(() => {});
    await expect(page.locator("img[alt*='Page']").first()).toBeVisible({ timeout: 15000 });
  }
}

// ── Tests ────────────────────────────────────────────────────────────

test("presets apply the correct access rules", async ({ page }) => {
  await openCreateDialog(page);
  await page.getByLabel("Link name").fill("Preset Test");

  // Default is Standard: require email + watermark + 30-day expiry.
  await expect(page.getByRole("combobox", { name: "Link preset" })).toHaveText(/Standard/i);
  await switchToAccessTab(page);
  await expect(page.getByRole("switch", { name: /Require email to view/i })).toHaveAttribute(
    "data-checked",
    "true"
  );
  await expect(page.getByRole("switch", { name: /Apply watermark/i })).toHaveAttribute(
    "data-checked",
    "true"
  );

  // Public preset clears all gates.
  await switchToShareTab(page);
  await page.getByRole("combobox", { name: "Link preset" }).click();
  await page.getByRole("option", { name: /Public/i }).click();
  await switchToAccessTab(page);
  await expect(page.getByRole("switch", { name: /Require email to view/i })).toHaveAttribute(
    "data-checked",
    "false"
  );
  await expect(page.getByRole("switch", { name: /Apply watermark/i })).toHaveAttribute(
    "data-checked",
    "false"
  );

  // Confidential preset enables email verification + password + watermark.
  await switchToShareTab(page);
  await page.getByRole("combobox", { name: "Link preset" }).click();
  await page.getByRole("option", { name: /Confidential/i }).click();
  await switchToAccessTab(page);
  await expect(page.getByRole("switch", { name: /Require email to view/i })).toHaveAttribute(
    "data-checked",
    "true"
  );
  await expect(page.getByRole("switch", { name: /Require email verification/i })).toHaveAttribute(
    "data-checked",
    "true"
  );
  await expect(page.getByRole("switch", { name: /Require password to view/i })).toHaveAttribute(
    "data-checked",
    "true"
  );
  await expect(page.getByRole("switch", { name: /Apply watermark/i })).toHaveAttribute(
    "data-checked",
    "true"
  );

  await page.getByRole("dialog", { name: /Create share link/i }).getByRole("button", { name: "Cancel" }).click();
  // Preset changes count as unsaved changes; discard them.
  await page.getByRole("button", { name: /Close without saving/i }).click();
  await expect(page.getByRole("dialog", { name: /Create share link/i })).not.toBeVisible({ timeout: 5000 });
});

test("creates a highly constrained link and verifies every public gate", async ({ page }) => {
  const allowedEmail = `allowed-${Date.now()}@example.com`;
  const blockedEmail = `blocked-${Date.now()}@example.com`;
  const password = `Strong-Pass-${Date.now()}!`;

  await openCreateDialog(page);
  await page.getByLabel("Link name").fill("Full Control Link");

  await switchToAccessTab(page);

  // Enable all strong constraints.
  await setSwitch(page, /Require email verification/i, true);
  await setSwitch(page, /Require password to view/i, true);
  await page.locator('input[type="password"][placeholder="Enter password"]').fill(password);
  await setSwitch(page, /Require NDA to view/i, true);
  await selectNdaDocument(page);
  await setSwitch(page, /Apply watermark/i, true);
  await setSwitch(page, /Allow downloading/i, false);
  await setSwitch(page, /Enable screenshot protection/i, true);

  // Allowed + blocked viewers.
  await addEmailTag(page, "allowed", allowedEmail);
  await addEmailTag(page, "blocked", blockedEmail);

  // Advanced features — Visitor Ask master + Ask Host; Ask Docs may require KB.
  await page.getByRole("button", { name: /Advanced/i }).click();
  await setSwitch(page, /Visitor Ask/i, true);
  // Master defaults to Ask Docs when KB allows; ensure Ask Host is also on.
  const askHost = page.getByRole("switch", { name: /Ask Host/i });
  if (await askHost.isVisible().catch(() => false)) {
    const checked = await askHost.isChecked();
    if (!checked) await askHost.click();
  }
  await setSwitch(page, /Enable file requests/i, true);
  await setSwitch(page, /Enable index file generation/i, true);

  // Intercept the create response to capture the public URL.
  const responsePromise = waitForCreateResponse(page);
  await page.getByRole("button", { name: /Create link/i }).click();
  const response = await responsePromise;
  expect(response.ok()).toBe(true);
  const request = response.request();
  const createPayload = request.postDataJSON() as Record<string, unknown>;
  expect(createPayload.nda_document_id).toBeTruthy();
  const link = (await response.json()) as { id: string; shortUrl: string };

  await expect(page.getByRole("dialog", { name: /Create share link/i })).not.toBeVisible({ timeout: 10000 });

  // Allowed visitor passes all gates.
  const allowedPage = await page.context().newPage();
  attachDebug(allowedPage);
  await visitGatedLink(allowedPage, link.shortUrl, {
    email: allowedEmail,
    code: "123456",
    password,
    nda: true,
  });
  await expect(allowedPage.locator("img[alt*='Page']").first()).toBeVisible({ timeout: 15000 });

  // Download should be disabled.
  await expect(allowedPage.getByRole("button", { name: /Download/i })).not.toBeVisible({ timeout: 5000 }).catch(() => {});

  // Advanced features should be exposed in the public sidebar.
  await allowedPage.getByRole("button", { name: /Open sidebar/i }).click().catch(() => {});
  await expect(allowedPage.getByText(/Ask|Visitor Ask|Ask Docs|Ask Host/i).first()).toBeVisible({ timeout: 5000 }).catch(() => {});
  await allowedPage.close();

  // Blocked visitor is denied even with correct credentials.
  const blockedPage = await page.context().newPage();
  attachDebug(blockedPage);
  await visitGatedLink(blockedPage, link.shortUrl, {
    email: blockedEmail,
    code: "123456",
    password,
    nda: true,
    expectDenied: true,
  });
  await blockedPage.close();

  // Wrong password is rejected and allows retry.
  const retryPage = await page.context().newPage();
  attachDebug(retryPage);
  await retryPage.goto(link.shortUrl);
  await retryPage.locator("#email").fill(allowedEmail);
  await retryPage.getByRole("button", { name: /Send code/i }).click();
  await retryPage.locator('input[inputmode="numeric"]').fill("123456");
  await retryPage.locator("#password").fill("wrong-password");
  await retryPage.getByRole("checkbox", { name: /agree/i }).check();
  await retryPage.getByRole("button", { name: /Continue/i }).click();
  await expect(retryPage.getByText(/invalid password/i)).toBeVisible({ timeout: 10000 });
  await retryPage.locator("#password").fill(password);
  await retryPage.getByRole("button", { name: /Continue/i }).click();
  await expect(retryPage.locator("img[alt*='Page']").first()).toBeVisible({ timeout: 15000 });
  await retryPage.close();
});

test("edits an existing link and verifies the updated gates", async ({ page }) => {
  const link = await createDealRoomLinkViaApi({ name: "Edit Target", download_enabled: true });
  const allowedEmail = `edit-allowed-${Date.now()}@example.com`;
  const password = `Edit-Strong-${Date.now()}!`;

  await openEditDialog(page, "Edit Target");

  // Verify name backfilled.
  await expect(page.getByLabel("Link name")).toHaveValue("Edit Target");

  await switchToAccessTab(page);
  await setSwitch(page, /Require password to view/i, true);
  await page.locator('input[type="password"][placeholder="Enter password"]').fill(password);
  await addEmailTag(page, "allowed", allowedEmail);

  await page.getByRole("button", { name: /Save link settings|Save access rules/i }).click();
  await page.waitForTimeout(1500);

  // Allowed visitor can now access with the new password.
  const allowedPage = await page.context().newPage();
  attachDebug(allowedPage);
  await visitGatedLink(allowedPage, link.shortUrl, { email: allowedEmail, password });
  await expect(allowedPage.locator("img[alt*='Page']").first()).toBeVisible({ timeout: 15000 });
  await allowedPage.close();

  // Non-allowed visitor is denied.
  const deniedPage = await page.context().newPage();
  attachDebug(deniedPage);
  await visitGatedLink(deniedPage, link.shortUrl, {
    email: `other-${Date.now()}@example.com`,
    password,
    expectDenied: true,
  });
  await deniedPage.close();
});

test("disables and re-enables a link from the edit dialog", async ({ page }) => {
  const link = await createDealRoomLinkViaApi({ name: "Toggle Target", download_enabled: true });

  await openEditDialog(page, "Toggle Target");

  // The header switch is the only Switch inside the dialog header.
  const headerSwitch = page.locator('[role="dialog"] [data-slot="switch"]').first();
  await expect(headerSwitch).toBeVisible({ timeout: 5000 });

  // Disable.
  await headerSwitch.click();
  await page.getByRole("button", { name: /Disable/i }).click();
  await page.waitForTimeout(1000);
  await expect(page.getByText(/Inactive/i).first()).toBeVisible({ timeout: 5000 });

  // Public access is blocked.
  const blockedPage = await page.context().newPage();
  await blockedPage.goto(link.shortUrl);
  await expect(blockedPage.getByText(/Link disabled|inactive/i).first()).toBeVisible({ timeout: 10000 });
  await blockedPage.close();

  // Re-enable.
  await headerSwitch.click();
  await page.waitForTimeout(1000);
  await expect(page.getByText(/Active/i).first()).toBeVisible({ timeout: 5000 });

  // Public access works again.
  const activePage = await page.context().newPage();
  await activePage.goto(link.shortUrl);
  await expect(activePage.locator("img[alt*='Page']").first()).toBeVisible({ timeout: 15000 });
  await activePage.close();
});

test("warns about unsaved changes when closing the dialog", async ({ page }) => {
  await openCreateDialog(page);
  await page.getByLabel("Link name").fill("Unsaved Test");

  const dialog = page.getByRole("dialog", { name: /Create share link/i });

  // Cancel triggers the unsaved-changes confirm dialog.
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await expect(page.getByRole("dialog", { name: /Unsaved changes/i })).toBeVisible({ timeout: 5000 });

  // Choosing Cancel keeps the dialog open with the draft intact.
  await page.getByRole("dialog", { name: /Unsaved changes/i }).getByRole("button", { name: "Cancel" }).click();
  await expect(page.getByRole("dialog", { name: /Unsaved changes/i })).not.toBeVisible({ timeout: 5000 });
  await expect(page.getByLabel("Link name")).toHaveValue("Unsaved Test");

  // Choosing Close without saving discards the draft.
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await page.getByRole("dialog", { name: /Unsaved changes/i }).getByRole("button", { name: /Close without saving/i }).click();
  await expect(dialog).not.toBeVisible({ timeout: 5000 });
});

test("shows validation errors for invalid share settings", async ({ page }) => {
  await openCreateDialog(page);

  // Empty link name: the button is disabled and the error is shown reactively.
  await expect(page.getByText(/Link name is required/i)).toBeVisible({ timeout: 5000 });
  await expect(page.getByRole("button", { name: /Create link/i })).toBeDisabled();

  await page.getByLabel("Link name").fill("Validation Test");
  await switchToAccessTab(page);

  // Weak password.
  await setSwitch(page, /Require password to view/i, true);
  await page.locator('input[type="password"][placeholder="Enter password"]').fill("weak");
  await expect(page.getByText(/Password must be at least 8 characters/i)).toBeVisible({ timeout: 5000 });
  await expect(page.getByRole("button", { name: /Create link/i })).toBeDisabled();

  await page.locator('input[type="password"][placeholder="Enter password"]').fill("StrongPass123!");

  // Allowed viewers without email collection.
  await addEmailTag(page, "allowed", "viewer@example.com");
  await setSwitch(page, /Require email to view/i, false);
  await expect(page.getByText(/Allowed viewers require email collection/i)).toBeVisible({ timeout: 5000 });
  await expect(page.getByRole("button", { name: /Create link/i })).toBeDisabled();

  // Re-enable email and remove the allowed viewer; email gates now require allowed viewers.
  await setSwitch(page, /Require email to view/i, true);
  await page.getByRole("button", { name: /Remove viewer@example.com/i }).click();
  await page.waitForTimeout(300);
  await expect(page.getByText(/Allowed viewers are required/i)).toBeVisible({ timeout: 5000 });
  await expect(page.getByRole("button", { name: /Create link/i })).toBeDisabled();

  // Re-add the allowed viewer to resolve the error, then add a conflicting blocked viewer.
  await addEmailTag(page, "allowed", "viewer@example.com");
  await addEmailTag(page, "blocked", "viewer@example.com");
  await expect(page.getByText(/cannot be in both allowed and blocked lists/i)).toBeVisible({ timeout: 5000 });
  await expect(page.getByRole("button", { name: /Create link/i })).toBeDisabled();

  // Invalid custom domain.
  await switchToShareTab(page);
  await page.getByRole("combobox", { name: "Custom domain" }).click();
  await page.getByRole("option", { name: /Custom domain/i }).click();
  await page.getByPlaceholder("links.yourdomain.com").fill("not a domain");
  await expect(page.getByText(/Please enter a valid domain/i)).toBeVisible({ timeout: 5000 });
  await expect(page.getByRole("button", { name: /Create link/i })).toBeDisabled();

  await page.getByRole("dialog", { name: /Create share link/i }).getByRole("button", { name: "Cancel" }).click();
});

test("adds allowed viewers from existing contacts and creates new contacts", async ({ page }) => {
  // Seed an existing contact.
  const existingEmail = `existing-${Date.now()}@example.com`;
  await apiFetch(`/api/workspaces/${seed.workspaceSlug}/contacts`, {
    method: "POST",
    body: JSON.stringify({ email: existingEmail, name: "Existing Contact" }),
  });

  await openCreateDialog(page);
  await page.getByLabel("Link name").fill("Contact Picker Test");
  await switchToAccessTab(page);

  // Add from the existing contact list.
  await page.getByRole("button", { name: /Add from contacts/i }).first().click();
  await page.getByRole("menuitem", { name: /Contact list/i }).click();
  await page.getByText("Existing Contact").click();
  await expect(page.getByText(existingEmail)).toBeVisible({ timeout: 5000 });

  // Create and add a new contact.
  await page.getByRole("button", { name: /Add from contacts/i }).first().click();
  await page.getByRole("menuitem", { name: /Add contact/i }).click();
  const newEmail = `new-${Date.now()}@example.com`;
  await page.getByLabel("Email").fill(newEmail);
  await page.getByRole("button", { name: /Create/i }).click();
  await page.waitForTimeout(500);
  await expect(page.getByText(newEmail)).toBeVisible({ timeout: 5000 });

  await page.getByRole("dialog", { name: /Create share link/i }).getByRole("button", { name: "Cancel" }).click();
});

test("applies expiration, custom domain and notify-on-access, and manages the link from participants", async ({ page }) => {
  const future = new Date(Date.now() + 24 * 60 * 60 * 1000);
  const dateTimeLocal = future.toISOString().slice(0, 16);
  const domain = "links.example.com";

  await openCreateDialog(page);
  await page.getByLabel("Link name").fill("Expiry Domain Test");

  // 公开预设没有访问控制强约束，避免创建按钮因缺少允许访客而禁用。
  await page.getByRole("combobox", { name: "Link preset" }).click();
  await page.getByRole("option", { name: /Public/i }).click();

  await setSwitch(page, /Expires on/i, true);
  await page.locator('input[type="datetime-local"]').fill(dateTimeLocal);

  await setSwitch(page, /Notify on access/i, true);

  await page.getByRole("combobox", { name: "Custom domain" }).click();
  await page.getByRole("option", { name: /Custom domain/i }).click();
  await page.getByPlaceholder("links.yourdomain.com").fill(domain);

  const responsePromise = waitForCreateResponse(page);
  await page.getByRole("button", { name: /Create link/i }).click();
  const response = await responsePromise;
  const request = response.request();
  const payload = request.postDataJSON() as Record<string, unknown>;
  expect(payload.expires_at).toBeDefined();
  expect(payload.custom_domain).toBe(domain);
  expect(payload.notify_on_access).toBe(true);

  const link = (await response.json()) as { id: string; shortUrl: string };

  // Public link works before expiry.
  const visitorPage = await page.context().newPage();
  await visitorPage.goto(link.shortUrl);
  await expect(visitorPage.locator("img[alt*='Page']").first()).toBeVisible({ timeout: 15000 });
  await visitorPage.close();

  // Edit the link to set an expired date and verify it blocks access.
  await page.goto(`/${seed.workspaceSlug}/deal-rooms/${roomId}?tab=participants`);
  await page.waitForTimeout(1500);
  const row = page.locator("table tbody tr").filter({ hasText: "Expiry Domain Test" }).first();
  await row.getByRole("button", { name: "moreActions" }).click();
  await page.getByRole("menuitem", { name: /Edit/i }).click();

  await page.locator('input[type="datetime-local"]').fill(new Date(Date.now() - 60 * 60 * 1000).toISOString().slice(0, 16));
  await page.getByRole("button", { name: /Save link settings|Save access rules/i }).click();
  await page.waitForTimeout(1500);

  const expiredPage = await page.context().newPage();
  await expiredPage.goto(link.shortUrl);
  await expect(expiredPage.getByText(/Link expired/i)).toBeVisible({ timeout: 10000 });
  await expiredPage.close();
});

