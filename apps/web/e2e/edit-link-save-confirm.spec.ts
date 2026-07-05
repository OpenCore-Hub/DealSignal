import { test, expect } from "@playwright/test";
import { setupAuthenticatedPage, WORKSPACE_SLUG } from "./helpers";

test("edit link: free step navigation, save shows confirm dialog", async ({ page }) => {
  page.on("dialog", async (dialog) => {
    // Any legacy window.confirm / window.alert should fail this test.
    throw new Error(`Unexpected legacy dialog: ${dialog.type()} - ${dialog.message()}`);
  });

  await setupAuthenticatedPage(page);

  // mockLinks[2] id is "link_3" in MSW; public permission type avoids the
  // email-verification contact guard so the save-confirm dialog can be tested.
  await page.goto(`/${WORKSPACE_SLUG}/links/link_3/edit`);

  // Wait for the edit pipeline to load and show the selected document.
  await expect(page.locator('[data-testid="bundle-doc-checkbox-doc_1"]')).toBeVisible({ timeout: 5000 });
  await expect(page.getByTestId('bundle-doc-label-doc_1')).toContainText('Acme Seed Round Pitch Deck.pdf');

  // Navigate forward/back between steps in edit mode; no window.confirm should appear.
  await page.locator('[data-testid="pipeline-nav-forward"]').click();
  await expect(page.locator('text=Security Options')).toBeVisible();

  await page.locator('[data-testid="pipeline-nav-forward"]').click();
  await expect(page.locator('text=Link Bundle Contents')).toBeVisible();

  await page.locator('[data-testid="pipeline-nav-back"]').click();
  await expect(page.locator('text=Security Options')).toBeVisible();

  await page.locator('[data-testid="pipeline-nav-forward"]').click();
  await expect(page.locator('text=Link Bundle Contents')).toBeVisible();

  // Click save on step 3 — should open the custom confirmation dialog (not window.confirm).
  await page.locator('[data-testid="review-submit-button"]').click();

  await expect(page.locator('text=Confirm save?')).toBeVisible({ timeout: 3000 });
  await expect(page.locator('text=Once saved, the distributed link will be updated immediately')).toBeVisible();

  // Cancel should close the dialog and keep the user on the review step.
  await page.getByRole("button", { name: "Cancel" }).click();
  await expect(page.locator('text=Confirm save?')).not.toBeVisible();
  await expect(page.locator('[data-testid="review-submit-button"]')).toBeVisible();

  // Confirm save should persist and navigate back to the link list.
  await page.locator('[data-testid="review-submit-button"]').click();
  await expect(page.locator('text=Confirm save?')).toBeVisible();
  await page.getByRole("button", { name: "Save Changes" }).click();

  await page.waitForURL(`/${WORKSPACE_SLUG}/links`);
  await expect(page.getByRole("heading", { name: "Links" }).first()).toBeVisible();
});
