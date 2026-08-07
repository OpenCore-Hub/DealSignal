/**
 * Phase C — Formal Q&A inbox + publish (MSW).
 */
import { test, expect } from "@playwright/test";
import {
  setupAuthenticatedPage,
  attachDebug,
  gotoAuthenticatedWaitForApi,
  WORKSPACE_SLUG,
} from "./helpers";

test.describe("Deal room Formal Ask queue (MSW) — Phase C", () => {
  test("formal queue lists pending question and publishes to public board", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await gotoAuthenticatedWaitForApi(
      page,
      `/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=qa`,
      "/deal-rooms/room_1/ask",
    );
    await expect(page.getByText("Ask inbox")).toBeVisible({ timeout: 15000 });

    const formalTabPromise = page.waitForResponse(
      (res) =>
        res.request().method() === "GET" &&
        res.url().includes("/deal-rooms/room_1/ask") &&
        res.url().includes("status=formal_queue") &&
        res.ok(),
    );
    await page.getByRole("tab", { name: /Formal queue/i }).click();
    await formalTabPromise;

    await expect(
      page.getByText("What is the board-approved revenue guidance?"),
    ).toBeVisible({ timeout: 10000 });
    await expect(page.getByText("compliance@example.com")).toBeVisible();

    const card = page.locator("li").filter({
      hasText: "What is the board-approved revenue guidance?",
    });
    await card.getByPlaceholder(/approved public answer/i).fill("Guidance is $42M ARR.");
    const publishPromise = page.waitForResponse(
      (res) =>
        res.request().method() === "PATCH" &&
        res.url().includes("/formal-publish") &&
        res.ok(),
    );
    await card.getByRole("button", { name: /Publish now/i }).click();
    await publishPromise;

    await expect(page.getByRole("tabpanel", { name: /Formal queue/i })).toContainText(
      /No formal questions awaiting review|暂无待审核 Formal 问题/,
    );
  });
});
