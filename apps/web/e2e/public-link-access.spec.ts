import { test, expect } from "@playwright/test";

test.describe("public link viewer", () => {
  test("accesses a public document with email verification code", async ({ page }) => {
    // Use the MSW mock public link token for link_1 (legacy email gate).
    await page.goto("/l/A1b2C3");

    // Should land on the access gate.
    await expect(page.locator("text=This document is shared securely")).toBeVisible({ timeout: 5000 });
    await expect(page.locator("text=Access code")).toBeVisible();

    // Legacy email gate requires both email and code.
    await page.locator('input#email').fill("visitor@example.com");
    await page.locator('input[inputMode="numeric"]').fill("123456");
    await page.locator('button:has-text("Continue")').click();

    // Viewer should load the document title and page metadata.
    await expect(page.locator("text=Acme Seed Round Pitch Deck")).toBeVisible({ timeout: 10000 });
    await expect(page.locator("text=18 pages")).toBeVisible();
    await expect(page.locator("text=1 / 18")).toBeVisible();
  });
});
