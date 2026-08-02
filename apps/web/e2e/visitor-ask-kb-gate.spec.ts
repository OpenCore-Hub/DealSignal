/**
 * B6 / US#11–12 / A4 — MSW e2e for Ask Docs KB pre-gate, coverage warning,
 * and no_searchable_chunks rejection.
 */
import { test, expect, type Locator } from "@playwright/test";
import { setupAuthenticatedPage, attachDebug, WORKSPACE_SLUG } from "./helpers";

async function openAdvancedAccess(dialog: Locator) {
  await dialog.getByRole("tab", { name: /Access/i }).click();
  await expect(dialog.getByText(/Authentication|Require email/i).first()).toBeVisible({
    timeout: 15000,
  });
  await dialog.getByRole("button", { name: /Advanced/i }).click();
  await expect(dialog.getByRole("switch", { name: /Visitor Ask/i })).toBeVisible({
    timeout: 5000,
  });
}

/** Seed an empty ready KB via the page fetch path so MSW intercepts it. */
async function seedEmptyReadyKnowledgeBase(page: import("@playwright/test").Page) {
  const body = await page.evaluate(async (slug) => {
    const res = await fetch(`/api/workspaces/${slug}/deal-rooms/room_1/knowledge-base`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Accept: "application/json" },
      credentials: "include",
      body: JSON.stringify({ folder_paths: [], document_ids: [] }),
    });
    if (!res.ok && res.status !== 201) {
      throw new Error(`KB seed failed: ${res.status}`);
    }
    return res.json();
  }, WORKSPACE_SLUG);
  expect(body.status).toBe("ready");
}

test.describe("Visitor Ask KB gates (MSW) — B6", () => {
  test("Access advanced blocks Ask Docs until room KB is ready", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=participants`);
    await expect(page.getByRole("button", { name: /Create link/i }).first()).toBeVisible({
      timeout: 15000,
    });
    await page.getByRole("button", { name: /Create link/i }).first().click();

    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible({ timeout: 10000 });
    await openAdvancedAccess(dialog);

    const visitorAsk = dialog.getByRole("switch", { name: /Visitor Ask/i });
    if (!(await visitorAsk.isChecked())) {
      await visitorAsk.click({ force: true });
    }

    await expect(
      dialog.getByText(/Create or rebuild the room knowledge base before enabling Ask Docs/i),
    ).toBeVisible({ timeout: 5000 });
    await expect(dialog.getByRole("switch", { name: /Ask Docs/i })).toBeDisabled();
  });

  test("saving Ask Docs with empty KB selection shows coverage warning", async ({ page }) => {
    test.setTimeout(60000);
    attachDebug(page);
    await setupAuthenticatedPage(page);

    // Full navigations remount MSW and wipe in-memory KB state — seed on the
    // same document after landing on participants, then open the create dialog.
    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=participants`);
    await expect(page.getByRole("button", { name: /Create link/i }).first()).toBeVisible({
      timeout: 15000,
    });
    await seedEmptyReadyKnowledgeBase(page);

    await page.getByRole("button", { name: /Create link/i }).first().click();

    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible({ timeout: 10000 });
    await expect(dialog.getByPlaceholder(/Recipient's Organization|Link name/i)).toBeVisible({
      timeout: 15000,
    });
    await dialog
      .getByPlaceholder(/Recipient's Organization|Link name/i)
      .fill(`Ask Docs coverage ${Date.now()}`);

    await dialog.getByRole("tab", { name: /Access/i }).click();
    await expect(dialog.getByText(/Authentication|Require email/i).first()).toBeVisible({
      timeout: 15000,
    });
    // Clear standard-preset email verification so Create is enabled without allowlist.
    const requireVerification = dialog.getByRole("switch", { name: /Require email verification/i });
    if (await requireVerification.isChecked()) {
      await requireVerification.click({ force: true });
    }

    await dialog.getByRole("button", { name: /Advanced/i }).click();
    await expect(dialog.getByRole("switch", { name: /Visitor Ask/i })).toBeVisible({
      timeout: 5000,
    });

    const visitorAsk = dialog.getByRole("switch", { name: /Visitor Ask/i });
    if (!(await visitorAsk.isChecked())) {
      await visitorAsk.click({ force: true });
    }
    const askDocs = dialog.getByRole("switch", { name: /Ask Docs/i });
    await expect(
      dialog.getByText(/Create or rebuild the room knowledge base before enabling Ask Docs/i),
    ).toHaveCount(0, { timeout: 15000 });
    await expect(askDocs).toBeEnabled({ timeout: 5000 });
    if (!(await askDocs.isChecked())) {
      await askDocs.click({ force: true });
    }

    const createBtn = dialog.getByRole("button", { name: /Create link/i });
    await expect(createBtn).toBeEnabled({ timeout: 10000 });
    await createBtn.click();
    await expect(
      page.getByText(/outside the knowledge base|Outside KB/i).first(),
    ).toBeVisible({ timeout: 10000 });
  });

  test("KB create rejects documents with no searchable chunks", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=documents`);
    const panel = page.getByTestId("knowledge-base-panel");
    await expect(panel).toBeVisible({ timeout: 15000 });
    await expect(panel.getByText(/Not created/i)).toBeVisible({ timeout: 10000 });

    await panel.getByRole("button", { name: /Create knowledge base/i }).click();
    const noChunks = panel.getByRole("checkbox", {
      name: /Scanned Exhibit \(no searchable text\)/i,
    });
    await expect(noChunks).toBeVisible({ timeout: 5000 });
    await noChunks.click();
    await panel.getByRole("button", { name: /^Create$/i }).click();

    await expect(
      page.getByText(/no searchable text|Re-ingest them/i).first(),
    ).toBeVisible({ timeout: 10000 });
  });
});
