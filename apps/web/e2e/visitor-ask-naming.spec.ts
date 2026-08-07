/**
 * B5/B7 naming (MSW) — Access + Bundle creator must show Visitor Ask / Ask Host,
 * never legacy "AI Copilot" / "AI Agents" / "Q&A conversations".
 */
import { test, expect } from "@playwright/test";
import { setupAuthenticatedPage, attachDebug, WORKSPACE_SLUG } from "./helpers";

test.describe("Visitor Ask naming (MSW) — B5/B7", () => {
  test("deal-room Access advanced shows AI assistant toggle", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=links`);
    const createLink = page.getByTestId("deal-room-create-new-link");
    await expect(createLink).toBeVisible({ timeout: 15000 });
    await createLink.click();

    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible({ timeout: 10000 });
    await expect(dialog.getByRole("button", { name: /^Advanced$/i })).toBeVisible({
      timeout: 10000,
    });
    await dialog.getByRole("button", { name: /^Advanced$/i }).click();
    await expect(dialog.getByTestId("deal-room-ai-assistant-toggle")).toBeVisible({
      timeout: 5000,
    });
    await expect(dialog.getByText(/AI assistant/i)).toBeVisible({ timeout: 5000 });
    await expect(dialog.getByRole("switch", { name: /AI assistant/i })).toBeVisible({
      timeout: 5000,
    });

    await expect(dialog.getByText(/AI Agents/i)).toHaveCount(0);
    await expect(dialog.getByText(/AI Copilot/i)).toHaveCount(0);
    await expect(dialog.getByText(/Q&A conversations/i)).toHaveCount(0);
  });

  test("disabling AI assistant persists ask_ai_enabled on save and reload", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=links`);
    await page.getByTestId("deal-room-create-new-link").click();

    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible({ timeout: 10000 });
    await dialog.getByLabel(/Link name/i).fill(`No AI ${Date.now()}`);

    await dialog.getByRole("button", { name: /^Advanced$/i }).click();
    const aiToggle = dialog.getByTestId("deal-room-ai-assistant-toggle").getByRole("switch");
    await expect(aiToggle).toBeVisible({ timeout: 5000 });
    if (await aiToggle.isChecked()) {
      await aiToggle.click();
    }
    await expect(aiToggle).not.toBeChecked();

    await dialog.getByRole("button", { name: /Create link/i }).click();
    await expect(dialog).toBeHidden({ timeout: 15000 });

    const createdRow = page.locator('[data-testid^="deal-room-link-row-"]').filter({
      hasText: /No AI /,
    });
    await expect(createdRow).toBeVisible({ timeout: 10000 });
    await createdRow.click();

    const editDialog = page.getByRole("dialog");
    await expect(editDialog).toBeVisible({ timeout: 10000 });
    await editDialog.getByRole("button", { name: /^Advanced$/i }).click();
    const reloadedToggle = editDialog
      .getByTestId("deal-room-ai-assistant-toggle")
      .getByRole("switch");
    await expect(reloadedToggle).not.toBeChecked({ timeout: 5000 });
  });

  test("bundle review step shows security summary without visitor Ask toggle", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/links/new`);
    const firstCheckbox = page.locator('[data-testid^="bundle-doc-checkbox-"]').first();
    await expect(firstCheckbox).toBeVisible({ timeout: 15000 });
    await firstCheckbox.click();

    const forward = page.locator('[data-testid="pipeline-nav-forward"]');
    await expect(forward).toBeEnabled({ timeout: 5000 });
    await forward.click();
    await expect(page.getByRole("heading", { name: /Access Control/i })).toBeVisible({
      timeout: 10000,
    });
    await forward.click();

    await expect(page.locator('[data-testid="review-submit-button"]')).toBeVisible({
      timeout: 10000,
    });
    await expect(page.getByText(/Security Configuration/i)).toBeVisible({ timeout: 5000 });
    await expect(page.getByText(/AI Copilot/i)).toHaveCount(0);
    await expect(page.getByText(/AI Agents/i)).toHaveCount(0);
    await expect(page.getByRole("switch", { name: /Visitor Ask/i })).toHaveCount(0);
  });

  test("link Engage tab shows Ask inbox and security events", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=links`);
    const row = page.getByTestId("deal-room-link-row-link_room_1");
    await expect(row).toBeVisible({ timeout: 15000 });
    await row.click();

    await expect(page.getByRole("dialog")).toBeVisible({ timeout: 10000 });
    await page.getByRole("tab", { name: /Engage/i }).click();

    await expect(page.getByText(/Ask activity/i)).toBeVisible({ timeout: 10000 });
    await expect(page.getByTestId("visitor-ask-experience")).toHaveCount(0);
    await expect(page.getByText("Ask inbox", { exact: true })).toBeVisible();
    await expect(page.getByTestId("ask-docs-audit-panel")).toHaveCount(0);
    await expect(page.getByText(/Visitor questions/i)).toHaveCount(0);
    await expect(page.getByText(/^Q&A records$/i)).toHaveCount(0);
  });
});
