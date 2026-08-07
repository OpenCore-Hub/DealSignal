/**
 * B10 — Deal room Ask inbox (MSW): real owner list + answer, no fake seed / comingSoon.
 */
import { test, expect } from "@playwright/test";
import {
  setupAuthenticatedPage,
  attachDebug,
  gotoAuthenticatedWaitForApi,
  WORKSPACE_SLUG,
} from "./helpers";

test.describe("Deal room Ask inbox (MSW) — B10", () => {
  test("room qa tab lists pending question and saves an answer", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await gotoAuthenticatedWaitForApi(
      page,
      `/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=qa`,
      "/deal-rooms/room_1/ask",
    );
    await expect(page.getByText("Ask inbox")).toBeVisible({ timeout: 15000 });
    await expect(page.getByRole("tab", { name: /Needs host/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    await expect(page.getByText(/When is the next board meeting/i)).toHaveCount(0);
    await expect(page.getByText(/coming soon/i)).toHaveCount(0);

    await expect(
      page.getByText("Can you share the updated financial model?"),
    ).toBeVisible({ timeout: 10000 });
    await expect(page.getByText("lp@example.com")).toBeVisible();

    const questionCard = page.locator("li").filter({
      hasText: "Can you share the updated financial model?",
    });
    await questionCard.getByPlaceholder(/Type your answer/i).fill("Model is in Finance / Model.xlsx.");
    const answerPromise = page.waitForResponse(
      (res) =>
        res.request().method() === "PATCH" &&
        res.url().includes("/host-answer") &&
        res.ok(),
    );
    await questionCard.getByRole("button", { name: /Send answer/i }).click();
    await answerPromise;

    await expect(page.getByRole("tabpanel", { name: /Needs host/i })).toContainText(
      /No Ask questions yet|暂无 Ask 问题/,
    );

    await page.getByRole("tab", { name: /^All$/i }).click();
    await expect(page.getByText(/Model is in Finance \/ Model\.xlsx/i)).toBeVisible({
      timeout: 10000,
    });
    await expect(page.getByText("Answered", { exact: true })).toBeVisible();
  });

  test("owner opens cited page from AI handled inbox", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await gotoAuthenticatedWaitForApi(
      page,
      `/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=qa`,
      "/deal-rooms/room_1/ask",
    );
    const aiInboxPromise = page.waitForResponse(
      (res) =>
        res.request().method() === "GET" &&
        res.url().includes("/deal-rooms/room_1/ask") &&
        res.url().includes("lane=ai") &&
        res.ok(),
    );
    await page.getByRole("tab", { name: /AI handled/i }).click();
    await aiInboxPromise;

    await expect(page.getByText("What was revenue growth last year?")).toBeVisible({
      timeout: 10000,
    });
    await page.getByRole("button", { name: /Open page 3/i }).click();
    await expect(page).toHaveURL(/\/viewer\/doc_1\?.*page=3/, { timeout: 10000 });
  });
});
