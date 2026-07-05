import { test, expect } from "@playwright/test";

test.describe("public link viewer", () => {
  test("accesses a public document with email verification code", async ({ page }) => {
    await page.goto("http://localhost:5173/l/a226e1cb94e31c6da159b92cde57fe15");

    // Should land on the access gate.
    await expect(page.locator("text=This document is shared securely")).toBeVisible({ timeout: 5000 });
    await expect(page.locator("text=Access code")).toBeVisible();

    // Fill the known verification code for yqx-401@126.com.
    await page.locator('input[inputMode="numeric"]').fill("741131");
    await page.locator('button:has-text("Continue")').click();

    // Viewer should load the document title and page metadata.
    await expect(page.locator("text=02_架构设计文档.docx")).toBeVisible({ timeout: 10000 });
    await expect(page.locator("text=6 pages")).toBeVisible();
    await expect(page.locator("text=1 / 6")).toBeVisible();
  });
});
