/**
 * B5/B7 naming (MSW) — Access + Bundle creator must show Visitor Ask / Ask Host,
 * never legacy "AI Copilot" / "AI Agents" / "Q&A conversations".
 */
import { test, expect } from "@playwright/test";
import { setupAuthenticatedPage, attachDebug, WORKSPACE_SLUG } from "./helpers";

test.describe("Visitor Ask naming (MSW) — B5/B7", () => {
  test("deal-room Access advanced shows Visitor Ask, not AI Agents", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=participants`);
    await expect(page.getByRole("button", { name: /Create link/i }).first()).toBeVisible({
      timeout: 15000,
    });
    await page.getByRole("button", { name: /Create link/i }).first().click();

    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible({ timeout: 10000 });
    await dialog.getByRole("tab", { name: /Access/i }).click();

    await dialog.getByRole("button", { name: /Advanced/i }).click();
    await expect(dialog.getByText(/Visitor Ask/i)).toBeVisible({ timeout: 5000 });

    const visitorAsk = dialog.getByRole("switch", { name: /Visitor Ask/i });
    await expect(visitorAsk).toBeVisible();
    await expect(dialog.getByRole("switch", { name: /Ask Docs/i })).toHaveCount(0);

    await expect(dialog.getByText(/AI Agents/i)).toHaveCount(0);
    await expect(dialog.getByText(/AI Copilot/i)).toHaveCount(0);
    await expect(dialog.getByText(/Q&A conversations/i)).toHaveCount(0);
  });

  test("bundle review step labels Ask Host, not AI Copilot", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/links/new`);
    const firstCheckbox = page.locator('[data-testid^="bundle-doc-checkbox-"]').first();
    await expect(firstCheckbox).toBeVisible({ timeout: 15000 });
    await firstCheckbox.click();

    const forward = page.locator('[data-testid="pipeline-nav-forward"]');
    await forward.click();
    await expect(page.getByText(/Security Options/i).first()).toBeVisible({ timeout: 10000 });
    await forward.click();

    await expect(page.locator('[data-testid="review-submit-button"]')).toBeVisible({
      timeout: 10000,
    });
    await expect(page.getByText(/Visitor Ask/i).first()).toBeVisible({ timeout: 5000 });
    await expect(page.getByText(/Ask Host/i).first()).toBeVisible();
    await expect(page.getByText(/AI Copilot/i)).toHaveCount(0);
    await expect(page.getByText(/AI Agents/i)).toHaveCount(0);
  });

  test("link Engage tab shows Ask Host inbox and security events", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=participants`);
    const row = page.getByTestId("deal-room-link-row-link_1");
    await expect(row).toBeVisible({ timeout: 15000 });
    await row.click();

    await expect(page.getByRole("dialog")).toBeVisible({ timeout: 10000 });
    await page.getByRole("tab", { name: /Engage/i }).click();

    await expect(page.getByText(/Ask Host activity/i)).toBeVisible({ timeout: 10000 });
    await expect(page.getByText("Ask Host inbox", { exact: true })).toBeVisible();
    await expect(page.getByTestId("ask-docs-audit-panel")).toHaveCount(0);
    await expect(page.getByText(/Visitor questions/i)).toHaveCount(0);
    await expect(page.getByText(/^Q&A records$/i)).toHaveCount(0);
  });
});
