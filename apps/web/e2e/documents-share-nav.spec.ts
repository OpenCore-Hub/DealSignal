/**
 * Regression (MSW): Document Library Share tab owns link management.
 * Legacy /links redirects here; sidebar keeps Documents active (no Links item).
 */
import { test, expect } from "@playwright/test";
import { setupAuthenticatedPage, attachDebug, WORKSPACE_SLUG } from "./helpers";

test.describe("Documents share navigation (MSW)", () => {
  test("legacy /links redirects to documents?tab=shared", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/links`);
    await expect(page).toHaveURL(new RegExp(`/${WORKSPACE_SLUG}/documents\\?tab=shared`), {
      timeout: 15000,
    });
    await expect(page.getByRole("tab", { name: /Shared|分享/i })).toBeVisible({
      timeout: 10000,
    });
  });

  test("Share tab shows links table and Documents nav stays active", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/documents?tab=shared`);
    await expect(page.getByRole("tab", { name: /Shared|分享/i })).toBeVisible({
      timeout: 15000,
    });
    await expect(page.locator('[data-testid="links-table-row"]').first()).toBeVisible({
      timeout: 15000,
    });

    // Sidebar no longer exposes a top-level Links destination.
    await expect(page.locator(`a[href="/${WORKSPACE_SLUG}/links"]`)).toHaveCount(0);

    const documentsNav = page.locator(`a[href="/${WORKSPACE_SLUG}/documents"]`).first();
    await expect(documentsNav).toBeVisible({ timeout: 5000 });
    await expect(documentsNav).toHaveAttribute("aria-current", "page");
  });

  test("/links?documentId= filters share tab query", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/links?documentId=doc_1&documentTitle=Deck`);
    await expect(page).toHaveURL(/tab=shared/, { timeout: 15000 });
    await expect(page).toHaveURL(/documentId=doc_1/, { timeout: 5000 });
  });
});
