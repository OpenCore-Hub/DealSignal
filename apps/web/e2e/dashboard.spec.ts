import { test, expect } from "@playwright/test";
import { setupAuthenticatedPage, WORKSPACE_SLUG } from "./helpers";

test.describe("Dashboard", () => {
  test.beforeEach(async ({ page }) => {
    await setupAuthenticatedPage(page);
  });

  test("renders dashboard summary, signals, actions and risk alerts", async ({ page }) => {
    await expect(page.getByRole("heading", { name: "Deal Radar" })).toBeVisible();
    await expect(page.getByText("Hot signals", { exact: true })).toBeVisible();
    await expect(page.getByText("Pending actions", { exact: true })).toBeVisible();

    await expect(page.getByText("Signals", { exact: true }).first()).toBeVisible();
    await expect(page.getByText("Recent documents", { exact: true })).toBeVisible();
    await expect(page.getByText("Actions", { exact: true }).first()).toBeVisible();
    await expect(page.getByText("Heat map", { exact: true })).toBeVisible();

    await expect(page.getByText("Risk alerts")).toBeVisible();
    await expect(page.getByText("Sarah Chen is heating up")).toBeVisible();
  });

  test("marks a pending action as done", async ({ page }) => {
    const firstAction = page.getByText("Follow up with Sarah Chen");
    await expect(firstAction).toBeVisible();

    await page.getByRole("button", { name: "Complete" }).first().click();
    await expect(page.getByText("Completed (1)")).toBeVisible();
    await expect(page.getByText("Follow up with Sarah Chen")).toHaveCount(1);
  });

  test("navigates from recent documents to document detail", async ({ page }) => {
    await page.getByText("Acme Seed Round Pitch Deck").first().click();
    await expect(page).toHaveURL(/\/documents\/doc_1/);
    await expect(page.getByText("Acme Seed Round Pitch Deck").first()).toBeVisible();
  });
});
