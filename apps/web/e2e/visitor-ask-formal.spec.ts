/**
 * Phase C — Formal Q&A inbox + publish (MSW).
 */
import { test, expect } from "@playwright/test";
import {
  setupAuthenticatedPage,
  attachDebug,
  gotoAuthenticatedWaitForApi,
  ASK_INBOX_TITLE,
  FORMAL_QUEUE_TAB,
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
    await expect(page.getByText(ASK_INBOX_TITLE)).toBeVisible({ timeout: 15000 });

    const formalTabPromise = page.waitForResponse(
      (res) =>
        res.request().method() === "GET" &&
        res.url().includes("/deal-rooms/room_1/ask") &&
        res.url().includes("status=formal_queue") &&
        res.ok(),
    );
    await page.getByRole("tab", { name: FORMAL_QUEUE_TAB }).click();
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

    await expect(page.getByRole("tabpanel", { name: FORMAL_QUEUE_TAB })).toContainText(
      /No formal questions awaiting review|暂无待审核 Formal 问题/,
    );
  });

  test("schedules formal publish and keeps turn in queue until publish time", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await gotoAuthenticatedWaitForApi(
      page,
      `/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=qa`,
      "/deal-rooms/room_1/ask",
    );
    await expect(page.getByText(ASK_INBOX_TITLE)).toBeVisible({ timeout: 15000 });

    const formalTabPromise = page.waitForResponse(
      (res) =>
        res.request().method() === "GET" &&
        res.url().includes("/deal-rooms/room_1/ask") &&
        res.url().includes("status=formal_queue") &&
        res.ok(),
    );
    await page.getByRole("tab", { name: FORMAL_QUEUE_TAB }).click();
    await formalTabPromise;

    const card = page.locator("li").filter({
      hasText: "What is the board-approved revenue guidance?",
    });
    await card.getByPlaceholder(/approved public answer/i).fill("Guidance is $42M ARR.");

    const future = new Date(Date.now() + 24 * 60 * 60 * 1000);
    const pad = (n: number) => String(n).padStart(2, "0");
    const scheduleValue = `${future.getFullYear()}-${pad(future.getMonth() + 1)}-${pad(future.getDate())}T${pad(future.getHours())}:${pad(future.getMinutes())}`;
    await card.locator('input[type="datetime-local"]').fill(scheduleValue);

    const schedulePromise = page.waitForResponse(
      (res) =>
        res.request().method() === "PATCH" &&
        res.url().includes("/formal-publish") &&
        res.ok(),
    );
    await card.getByRole("button", { name: /Schedule publish/i }).click();
    const scheduleRes = await schedulePromise;
    const scheduleBody = (await scheduleRes.json()) as {
      data: { formal_status?: string };
    };
    expect(scheduleBody.data.formal_status).toBe("scheduled");

    const formalPanel = page.getByRole("tabpanel", { name: FORMAL_QUEUE_TAB });
    await expect(formalPanel).toContainText("What is the board-approved revenue guidance?");
    await expect(formalPanel).not.toContainText(
      /No formal questions awaiting review|暂无待审核 Formal 问题/,
    );
  });
});
