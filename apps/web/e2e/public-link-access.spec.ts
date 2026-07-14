import { test, expect } from "@playwright/test";
import {
  seedRealBackend,
  seedDocument,
  seedLink,
} from "./real-helpers";

let shortUrl: string;

test.describe("public link viewer (real backend)", () => {
  test.beforeAll(async () => {
    const seed = await seedRealBackend();
    const doc = await seedDocument(seed.workspaceSlug);
    // Create a public link (no gate) so the viewer loads directly
    const link = await seedLink(seed.workspaceSlug, doc.id, {
      permissionType: "public",
      downloadEnabled: true,
    });
    shortUrl = link.shortUrl;
  });

  test("accesses a public document and renders the viewer", async ({ page }) => {
    await page.goto(shortUrl);

    // Public link should load the viewer directly (no gate)
    // Wait for the document title or page image to appear
    await expect(page.locator("img[alt*='Page']").first()).toBeVisible({ timeout: 15000 });

    // Verify page count is shown
    await expect(page.locator("text=/\\d+ page/i").first()).toBeVisible({ timeout: 5000 });
  });
});
