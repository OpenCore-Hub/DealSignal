import { test, expect } from "@playwright/test";
import {
  seedRealBackend,
  seedDocument,
  authenticatePage,
  attachDebug,
} from "./real-helpers";

let token: string;
let workspaceSlug: string;

test.describe("Dashboard (real backend)", () => {
  test.beforeAll(async () => {
    const seed = await seedRealBackend();
    token = seed.token;
    workspaceSlug = seed.workspaceSlug;
    // Upload a document so dashboard has data
    await seedDocument(token, workspaceSlug);
  });

  test("renders dashboard heading and summary cards", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page, token);
    await page.goto(`/${workspaceSlug}/dashboard`);
    await expect(page.getByRole("heading", { name: "Deal Radar" })).toBeVisible({ timeout: 10000 });
    await expect(page.getByText("Hot signals", { exact: true })).toBeVisible({ timeout: 5000 });
    await expect(page.getByText("Pending actions", { exact: true })).toBeVisible({ timeout: 5000 });
  });

  test("renders signal and recent document sections", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page, token);
    await page.goto(`/${workspaceSlug}/dashboard`);

    await expect(page.getByText("Signals", { exact: true }).first()).toBeVisible({ timeout: 10000 });
    await expect(page.getByText("Recent documents", { exact: true })).toBeVisible({ timeout: 5000 });
    await expect(page.getByText("Actions", { exact: true }).first()).toBeVisible({ timeout: 5000 });
    await expect(page.getByText("Heat map", { exact: true })).toBeVisible({ timeout: 5000 });
  });

  test("navigates from recent documents to document detail", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page, token);
    await page.goto(`/${workspaceSlug}/dashboard`);

    // Find the recently uploaded document in the list
    const docLink = page.getByText("sample.pdf").first();
    await expect(docLink).toBeVisible({ timeout: 10000 });
    await docLink.click();

    // Should navigate to document detail
    await expect(page).toHaveURL(/\/documents\//, { timeout: 10000 });
    await expect(page.getByText("sample.pdf").first()).toBeVisible({ timeout: 5000 });
  });

  test("dashboard loads actions section", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page, token);
    await page.goto(`/${workspaceSlug}/dashboard`);

    // The actions panel should be visible
    await expect(page.getByText("Actions", { exact: true }).first()).toBeVisible({ timeout: 10000 });
  });
});
