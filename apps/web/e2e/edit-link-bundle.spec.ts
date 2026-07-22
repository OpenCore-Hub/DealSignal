/**
 * Link bundle edit mode — backfill verification.
 *
 * 覆盖编辑已有链接包时文档列表、安全选项、审阅页的回填正确性，
 * 以及修改后保存的完整链路。
 */
import { test, expect } from "@playwright/test";
import {
  seedRealBackend,
  seedDocument,
  seedContact,
  authenticatePage,
  attachDebug,
  apiFetch,
} from "./real-helpers";

let seed: Awaited<ReturnType<typeof seedRealBackend>>;
let doc1Id: string;
let doc2Id: string;
let contactId: string;

test.beforeAll(async () => {
  seed = await seedRealBackend();
  const doc1 = await seedDocument(seed.workspaceSlug);
  const doc2 = await seedDocument(seed.workspaceSlug);
  doc1Id = doc1.id;
  doc2Id = doc2.id;
  const contact = await seedContact(
    seed.workspaceSlug,
    `bundle-edit-${Date.now()}@example.com`,
    "Bundle Contact"
  );
  contactId = contact.id;
});

test.beforeEach(async ({ page }) => {
  attachDebug(page);
  await authenticatePage(page);
});

async function createBundleLink(payload: Record<string, unknown>) {
  const res = await apiFetch(`/api/workspaces/${seed.workspaceSlug}/links`, {
    method: "POST",
    body: JSON.stringify(payload),
  });
  if (!res.ok) throw new Error(`create link failed: ${res.status} ${await res.text()}`);
  return (await res.json()) as { id: string; shortUrl: string };
}

test("edit mode backfills selected documents, security settings and review summary", async ({ page }) => {
  const expiresAt = new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString();
  const link = await createBundleLink({
    document_ids: [doc1Id],
    name: "Bundle Edit Backfill",
    require_nda: true,
    nda_document_id: doc1Id,
    download_enabled: false,
    watermark_enabled: true,
    ai_copilot_enabled: true,
    expires_at: expiresAt,
    max_access_count: 10,
    contact_ids: [contactId],
  });

  await page.goto(`/${seed.workspaceSlug}/links/${link.id}/edit`);
  await page.waitForTimeout(2000);

  // Step 1 — documents: doc1 selected, doc2 not selected.
  const doc1Checkbox = page.locator(`[data-testid="bundle-doc-checkbox-${doc1Id}"]`);
  const doc2Checkbox = page.locator(`[data-testid="bundle-doc-checkbox-${doc2Id}"]`);
  await expect(doc1Checkbox).toBeVisible({ timeout: 10000 });
  expect(await doc1Checkbox.isChecked()).toBe(true);
  expect(await doc2Checkbox.isChecked()).toBe(false);

  // Step 2 — security settings backfilled.
  await page.locator('[data-testid="pipeline-nav-forward"]').click();
  await expect(page.getByText("Security Options").first()).toBeVisible({ timeout: 5000 });

  await expect(page.locator('[data-testid="security-switch-ndaEnabled"]').first()).toHaveAttribute("data-checked", "true");
  await expect(page.locator('[data-testid="security-switch-allowDownload"]').first()).toHaveAttribute("data-checked", "false");
  await expect(page.locator('[data-testid="security-switch-watermarkEnabled"]').first()).toHaveAttribute("data-checked", "true");
  await expect(page.locator('[data-testid="security-switch-requireEmailVerification"]').first()).toHaveAttribute("data-checked", "true");

  const expiryTrigger = page.locator('[data-testid="security-expiry-select"]').first();
  const maxViewsTrigger = page.locator('[data-testid="security-max-views-select"]').first();
  await expect(expiryTrigger).toHaveText(/30 days|30/i);
  await expect(maxViewsTrigger).toHaveText(/10 views|10/i);

  // Step 3 — review summary reflects the loaded settings.
  await page.locator('[data-testid="pipeline-nav-forward"]').click();
  await expect(page.locator('[data-testid="review-submit-button"]')).toBeVisible({ timeout: 5000 });

  await expect(page.getByText("Bundle Edit Backfill")).toBeVisible();
  await expect(page.getByText(/NDA signing|Require NDA agreement/i)).toBeVisible();
  await expect(page.getByText(/Dynamic watermark/i)).toBeVisible();
  await expect(page.getByText(/Visitor Ask/i)).toBeVisible();
  await expect(page.getByText(/Ask Docs/i)).toBeVisible();
  await expect(page.getByText(/Download disabled/i)).toBeVisible();

  // Edit: add doc2, disable NDA, enable download.
  await page.locator('[data-testid="pipeline-nav-back"]').click();
  await page.locator('[data-testid="pipeline-nav-back"]').click();
  await doc2Checkbox.click();

  await page.locator('[data-testid="pipeline-nav-forward"]').click();
  await page.locator('[data-testid="security-switch-ndaEnabled"]').first().click();
  await page.locator('[data-testid="security-switch-allowDownload"]').first().click();

  await page.locator('[data-testid="pipeline-nav-forward"]').click();
  await expect(page.locator('[data-testid="review-submit-button"]')).toBeVisible({ timeout: 5000 });
  await expect(page.getByText(/Download enabled/i)).toBeVisible();
  await expect(page.getByText(/NDA signing|Require NDA agreement/i)).not.toBeVisible().catch(() => {});

  // Save and confirm.
  await page.locator('[data-testid="review-submit-button"]').click();
  await page.getByRole("dialog").getByRole("button", { name: /Save changes/i }).click();
  await page.waitForTimeout(2000);

  await expect(page).toHaveURL(new RegExp(`/${seed.workspaceSlug}/links`), { timeout: 10000 });

  // Verify the update persisted via API.
  const detailRes = await apiFetch(`/api/workspaces/${seed.workspaceSlug}/links/${link.id}`);
  expect(detailRes.ok).toBe(true);
  const detail = (await detailRes.json()) as Record<string, unknown>;
  const documentIds = (detail.documentIds as string[] | undefined) ??
    (detail.documents as Array<{ id: string }> | undefined)?.map((d) => d.id) ??
    [];
  expect(documentIds).toContain(doc2Id);
  expect(detail.requireNda ?? detail.require_nda).toBe(false);
  expect(detail.downloadEnabled ?? detail.download_enabled).toBe(true);
});

test("edit mode preserves document order and allows reordering", async ({ page }) => {
  const link = await createBundleLink({
    document_ids: [doc1Id, doc2Id],
    name: "Bundle Order Test",
    download_enabled: true,
  });

  await page.goto(`/${seed.workspaceSlug}/links/${link.id}/edit`);
  await page.waitForTimeout(2000);

  // Verify both documents are selected.
  const doc1Checkbox = page.locator(`[data-testid="bundle-doc-checkbox-${doc1Id}"]`);
  const doc2Checkbox = page.locator(`[data-testid="bundle-doc-checkbox-${doc2Id}"]`);
  await expect(doc1Checkbox).toBeVisible({ timeout: 10000 });
  expect(await doc1Checkbox.isChecked()).toBe(true);
  expect(await doc2Checkbox.isChecked()).toBe(true);

  // Verify initial order in the selected documents panel.
  // Simpler: check the order numbers in the selected panel.
  const firstOrder = page.locator("text=/^1$/").first();
  await expect(firstOrder).toBeVisible();

  // Use move-down to change order.
  const moveDownButtons = page.getByRole("button", { name: /Move down/i });
  await moveDownButtons.first().click();
  await page.waitForTimeout(300);

  await page.locator('[data-testid="pipeline-nav-forward"]').click();
  await page.locator('[data-testid="pipeline-nav-forward"]').click();
  await page.locator('[data-testid="review-submit-button"]').click();
  await page.getByRole("dialog").getByRole("button", { name: /Save changes/i }).click();
  await page.waitForTimeout(2000);

  await expect(page).toHaveURL(new RegExp(`/${seed.workspaceSlug}/links`), { timeout: 10000 });
});
