import { test, expect } from "@playwright/test";
import path from "path";
import { fileURLToPath } from "url";
import { setupAuthenticatedPage, WORKSPACE_SLUG, attachDebug } from "./helpers";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

test.describe("P0 user flow", () => {
  test("select workspace and upload a document", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/documents/upload`);
    await expect(page.getByRole("heading", { name: "Upload Document" })).toBeVisible();

    const fileInput = page.locator('[data-testid="file-upload"]');
    await fileInput.setInputFiles(path.join(__dirname, "fixtures", "sample.pdf"));

    await page.getByRole("button", { name: "Upload now" }).click();
    await expect(page.getByTestId("upload-success")).toBeVisible({ timeout: 10000 });
  });

  test("create a smart link and view analytics", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/links/new`);

    // Bundle pipeline Step 1: select a document.
    await expect(page.locator('[data-testid="bundle-doc-checkbox-doc_1"]')).toBeVisible({ timeout: 5000 });
    await page.locator('[data-testid="bundle-doc-checkbox-doc_1"]').click();

    // Step 2: Security (leave defaults).
    await page.locator('[data-testid="pipeline-nav-forward"]').click();
    await expect(page.getByText("Security Options")).toBeVisible();

    // Step 3: Review & create.
    await page.locator('[data-testid="pipeline-nav-forward"]').click();
    await expect(page.locator('[data-testid="review-submit-button"]')).toBeVisible();

    await page.locator('[data-testid="review-submit-button"]').click();
    await expect(page.locator('[data-testid="generated-link"]')).toBeVisible({ timeout: 10000 });

    const generatedLink = await page.locator('[data-testid="generated-link"]').textContent();
    expect(generatedLink).toContain("http");

    await page.goto(`/${WORKSPACE_SLUG}/links`);
    await expect(page.getByRole("heading", { name: "Share Links" })).toBeVisible();

    await page.locator('[data-testid="links-table-row"]').first().click();
    await expect(page).toHaveURL(/\/links\//);

    await expect(page.getByText("Total Visits")).toBeVisible();
    await expect(page.getByText("Unique Visitors")).toBeVisible();
    await expect(page.getByText("Access Log")).toBeVisible();
  });
});
