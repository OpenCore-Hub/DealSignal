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

    const fileInput = page.locator("input#file-upload");
    await fileInput.setInputFiles(path.join(__dirname, "fixtures", "sample.pdf"));

    await expect(page.getByTestId("upload-success")).toBeVisible({ timeout: 10000 });
  });

  test("create a smart link and view analytics", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/links/new`);
    await expect(page.getByRole("heading", { name: "Smart Link Creator" })).toBeVisible();

    await expect(page.getByTestId("selected-document")).not.toHaveText("Not selected");

    await page.getByTestId("create-link-button").click();
    await expect(page.getByTestId("generated-link")).toBeVisible();

    const generatedLink = await page.getByTestId("generated-link").textContent();
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
