/**
 * B10 — Deal room Ask Host inbox (MSW): real owner list + answer, no fake seed / comingSoon.
 */
import { test, expect } from "@playwright/test";
import { setupAuthenticatedPage, attachDebug, WORKSPACE_SLUG } from "./helpers";

test.describe("Deal room Ask Host inbox (MSW) — B10", () => {
  test("room qa tab lists pending question and saves an answer", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=qa`);
    await expect(page.getByText("Ask Host inbox")).toBeVisible({ timeout: 15000 });
    await expect(page.getByText(/When is the next board meeting/i)).toHaveCount(0);
    await expect(page.getByText(/coming soon/i)).toHaveCount(0);

    await expect(
      page.getByText("Can you share the updated financial model?"),
    ).toBeVisible({ timeout: 10000 });
    await expect(page.getByText("lp@example.com")).toBeVisible();

    await page.getByPlaceholder(/Type your answer/i).fill("Model is in Finance / Model.xlsx.");
    await page.getByRole("button", { name: /Send answer/i }).click();

    await expect(page.getByText(/Model is in Finance \/ Model\.xlsx/i)).toBeVisible({
      timeout: 10000,
    });
    await expect(page.getByText("Answered", { exact: true })).toBeVisible();
  });
});
