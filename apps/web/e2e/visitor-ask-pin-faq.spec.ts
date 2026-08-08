/**
 * Phase B/C — Pin FAQ owner workflow (MSW).
 */
import { test, expect } from "@playwright/test";
import {
  setupAuthenticatedPage,
  attachDebug,
  gotoAuthenticatedWaitForApi,
  ASK_INBOX_TITLE,
  WORKSPACE_SLUG,
} from "./helpers";

test.describe("Deal room Pin FAQ (MSW)", () => {
  test("pins AI answered turn and lists it under Pinned FAQ", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await gotoAuthenticatedWaitForApi(
      page,
      `/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=qa`,
      "/deal-rooms/room_1/ask",
    );
    await expect(page.getByText(ASK_INBOX_TITLE)).toBeVisible({ timeout: 15000 });

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

    const card = page.locator("li").filter({
      hasText: "What was revenue growth last year?",
    });
    const pinPromise = page.waitForResponse(
      (res) =>
        res.request().method() === "POST" &&
        res.url().includes("/ask/owner_ai_1/pin-faq") &&
        res.ok(),
    );
    await card.getByRole("button", { name: /Pin as FAQ/i }).click();
    await pinPromise;

    const faqListPromise = page.waitForResponse(
      (res) =>
        res.request().method() === "GET" &&
        res.url().includes("/deal-rooms/room_1/ask/faq") &&
        res.ok(),
    );
    await page.getByRole("tab", { name: /Pinned FAQ/i }).click();
    await faqListPromise;

    const pinnedPanel = page.getByRole("tabpanel", { name: /Pinned FAQ/i });
    await expect(pinnedPanel).toContainText("What was revenue growth last year?");
    await expect(pinnedPanel).toContainText(/Revenue grew 12% year over year/i);
    await expect(pinnedPanel.getByText(/Pinned FAQ/i).first()).toBeVisible();
  });

  test("unpins FAQ and returns empty pinned state", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await gotoAuthenticatedWaitForApi(
      page,
      `/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=qa`,
      "/deal-rooms/room_1/ask",
    );
    await page.getByRole("tab", { name: /AI handled/i }).click();
    await expect(page.getByText("What was revenue growth last year?")).toBeVisible({
      timeout: 10000,
    });

    const card = page.locator("li").filter({
      hasText: "What was revenue growth last year?",
    });
    await card.getByRole("button", { name: /Pin as FAQ/i }).click();
    await page.waitForResponse(
      (res) =>
        res.request().method() === "POST" &&
        res.url().includes("/pin-faq") &&
        res.ok(),
    );

    const faqListPromise = page.waitForResponse(
      (res) =>
        res.request().method() === "GET" &&
        res.url().includes("/deal-rooms/room_1/ask/faq") &&
        res.ok(),
    );
    await page.getByRole("tab", { name: /Pinned FAQ/i }).click();
    await faqListPromise;

    const pinnedCard = page.locator("li").filter({
      hasText: "What was revenue growth last year?",
    });
    const unpinPromise = page.waitForResponse(
      (res) =>
        res.request().method() === "POST" &&
        res.url().includes("/ask/owner_ai_1/unpin-faq") &&
        res.ok(),
    );
    await pinnedCard.getByRole("button", { name: /Unpin FAQ/i }).click();
    await unpinPromise;

    await expect(page.getByRole("tabpanel", { name: /Pinned FAQ/i })).toContainText(
      /No pinned FAQs yet|暂无 Pin FAQ/,
    );
  });
});
