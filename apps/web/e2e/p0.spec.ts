import { test, expect } from "@playwright/test";
import path from "path";
import { fileURLToPath } from "url";
import {
  seedRealBackend,
  seedDocument,
  authenticatePage,
  attachDebug,
} from "./real-helpers";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

let token: string;
let workspaceSlug: string;

test.describe("P0 user flow (real backend)", () => {
  test.beforeAll(async () => {
    const seed = await seedRealBackend();
    token = seed.token;
    workspaceSlug = seed.workspaceSlug;
    await seedDocument(token, workspaceSlug);
  });

  test("select workspace and upload a document", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page, token);

    await page.goto(`/${workspaceSlug}/documents/upload`);
    await expect(page.getByRole("heading", { name: "Upload Document" })).toBeVisible({ timeout: 10000 });

    const fileInput = page.locator('[data-testid="file-upload"]');
    await fileInput.setInputFiles(path.join(__dirname, "fixtures", "sample.pdf"));

    await page.getByRole("button", { name: "Upload now" }).click();
    // Uploader navigates to the documents list on completion.
    await expect(page).toHaveURL(new RegExp(`/${workspaceSlug}/documents$`), { timeout: 15000 });
    await expect(page.getByText("sample.pdf").first()).toBeVisible({ timeout: 10000 });
  });

  test("create a smart link via bundle pipeline", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page, token);

    await page.goto(`/${workspaceSlug}/links/new`);

    // Bundle pipeline Step 1: select a document.
    // Wait for the document list to load, then select the first document.
    await page.waitForTimeout(2000);
    const firstCheckbox = page.locator('[data-testid^="bundle-doc-checkbox-"]').first();
    await expect(firstCheckbox).toBeVisible({ timeout: 10000 });
    await firstCheckbox.click();

    // Step 2: Security (leave defaults).
    const forwardBtn = page.locator('[data-testid="pipeline-nav-forward"]');
    await expect(forwardBtn).toBeVisible({ timeout: 5000 });
    await forwardBtn.click();
    await expect(page.getByText("Security Options")).toBeVisible({ timeout: 5000 });

    // Step 3: Review & create.
    await forwardBtn.click();
    await expect(page.locator('[data-testid="review-submit-button"]')).toBeVisible({ timeout: 5000 });

    await page.locator('[data-testid="review-submit-button"]').click();
    await expect(page.locator('[data-testid="generated-link"]')).toBeVisible({ timeout: 15000 });

    const generatedLink = await page.locator('[data-testid="generated-link"]').textContent();
    expect(generatedLink).toContain("http");

    // Navigate to links list and verify link appears
    await page.goto(`/${workspaceSlug}/links`);
    await expect(page.getByRole("heading", { name: /links/i }).first()).toBeVisible({ timeout: 10000 });

    // Click first link row
    const firstRow = page.locator('[data-testid="links-table-row"]').first();
    await expect(firstRow).toBeVisible({ timeout: 5000 });
    await firstRow.click();
    await expect(page).toHaveURL(/\/links\//, { timeout: 10000 });

    await expect(page.getByText("Total Visits")).toBeVisible({ timeout: 5000 });
    await expect(page.getByText("Unique Visitors")).toBeVisible({ timeout: 5000 });
    await expect(page.getByText("Access Log")).toBeVisible({ timeout: 5000 });
  });
});
