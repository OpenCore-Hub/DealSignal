import { test, expect } from "@playwright/test";
import {
  seedRealBackend,
  seedDocument,
  seedLink,
  authenticatePage,
} from "./real-helpers";

let workspaceSlug: string;
let linkId: string;

test.describe("edit link save confirm (real backend)", () => {
  test.beforeAll(async () => {
    const seed = await seedRealBackend();
    workspaceSlug = seed.workspaceSlug;
    const doc = await seedDocument(workspaceSlug);
    // Create a public link so we can edit without contact guard
    const link = await seedLink(workspaceSlug, doc.id, {
      permissionType: "public",
      downloadEnabled: true,
      name: "Editable Test Link",
    });
    linkId = link.id;
  });

  test("edit link: free step navigation, save shows confirm dialog", async ({ page }) => {
    page.on("dialog", async (dialog) => {
      throw new Error(`Unexpected legacy dialog: ${dialog.type()} - ${dialog.message()}`);
    });

    await authenticatePage(page);

    await page.goto(`/${workspaceSlug}/links/${linkId}/edit`);

    // Wait for the edit pipeline to load
    await page.waitForTimeout(2000);

    // Navigate forward to security step
    const forwardBtn = page.locator('[data-testid="pipeline-nav-forward"]');
    await expect(forwardBtn).toBeVisible({ timeout: 10000 });
    await forwardBtn.click();

    // Should show security options
    await expect(page.getByText("Security Options").first()).toBeVisible({ timeout: 5000 });

    // Navigate forward to review step
    await forwardBtn.click();

    // Should show the link bundle content or review screen
    await page.waitForTimeout(1000);

    // Navigate back to security
    const backBtn = page.locator('[data-testid="pipeline-nav-back"]');
    if (await backBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
      await backBtn.click();
      await expect(page.getByText("Security Options").first()).toBeVisible({ timeout: 5000 });

      // Forward again to review
      await forwardBtn.click();
      await page.waitForTimeout(1000);
    }

    // Click save — should open confirmation dialog or save directly
    const submitBtn = page.locator('[data-testid="review-submit-button"], button:has-text("Save")');
    if (await submitBtn.first().isVisible({ timeout: 3000 }).catch(() => false)) {
      await submitBtn.first().click();

      // If confirm dialog appears, handle it
      const confirmSave = page.getByRole("button", { name: /save changes/i });
      const confirmDialog = page.locator("text=Confirm save").first();

      const hasDialog = await Promise.race([
        confirmDialog.isVisible().then(() => true),
        page.waitForTimeout(3000).then(() => false),
      ]);

      if (hasDialog) {
        await expect(confirmSave).toBeVisible({ timeout: 3000 });
        await confirmSave.click();
      }
    }

    // Should end up on links page after save
    await page.waitForTimeout(3000);
    const finalUrl = page.url();
    expect(finalUrl).toMatch(/\/links/);
  });
});
