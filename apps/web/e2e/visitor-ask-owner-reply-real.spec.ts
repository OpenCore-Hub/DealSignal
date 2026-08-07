/**
 * Visitor Ask owner reply loop against live API + Vite UI.
 *
 * Run:
 *   REAL_API_BASE_URL=http://localhost:8090 ./e2e-visitor-ask-real.sh --ui
 */
import { test, expect } from "@playwright/test";
import {
  seedRealBackend,
  seedDocument,
  seedDealRoom,
  seedDealRoomLink,
  authenticatePage,
  visitPublicLink,
  attachDebug,
} from "./real-helpers";

let workspaceSlug: string;
let roomId: string;
let linkShortUrl: string;

test.describe("Visitor Ask owner reply (real backend UI)", () => {
  test.beforeAll(async () => {
    test.setTimeout(180_000);
    const seed = await seedRealBackend();
    workspaceSlug = seed.workspaceSlug;
    const doc = await seedDocument(workspaceSlug);
    const room = await seedDealRoom(workspaceSlug, {
      name: "Visitor Ask UI Room",
      templateType: "seed",
      documentIds: [doc.id],
    });
    roomId = room.id;
    const link = await seedDealRoomLink(workspaceSlug, room.id, {
      name: `Ask UI Link ${Date.now()}`,
    });
    linkShortUrl = link.shortUrl;
  });

  test("visitor ask → owner QA inbox reply → visitor sees answer", async ({ page }) => {
    test.setTimeout(120_000);
    attachDebug(page);

    const question = `UI real ask ${Date.now()}?`;
    const answer = "UI real answer: next Monday.";

    await visitPublicLink(page, linkShortUrl);

    const openSidebar = page.getByRole("button", { name: /Open sidebar/i });
    if (await openSidebar.isVisible().catch(() => false)) {
      await openSidebar.click();
    }
    const hostInput = page.getByPlaceholder(/materials you can access|Ask the host/i);
    if (!(await hostInput.isVisible().catch(() => false))) {
      const askTab = page.locator("button.rounded-full").filter({ hasText: /^Ask$/ });
      if (await askTab.isVisible().catch(() => false)) {
        await askTab.click();
      }
    }
    await expect(hostInput).toBeVisible({ timeout: 15000 });
    await hostInput.fill(question);
    await page
      .getByRole("button", { name: "Ask", exact: true })
      .and(page.locator('[type="submit"]'))
      .click();
    await expect(page.getByText(question)).toBeVisible({ timeout: 15000 });
    await expect(page.getByText(/Awaiting reply/i)).toBeVisible({ timeout: 15000 });

    await authenticatePage(page);
    await page.goto(`/${workspaceSlug}/deal-rooms/${roomId}?tab=qa`);
    await expect(page.getByText("Ask inbox", { exact: true })).toBeVisible({
      timeout: 20000,
    });
    await expect(page.getByText(question)).toBeVisible({ timeout: 15000 });

    const questionCard = page.locator("li").filter({ hasText: question });
    await questionCard.getByPlaceholder(/Type your answer/i).fill(answer);
    const answerPromise = page.waitForResponse(
      (res) =>
        res.request().method() === "PATCH" &&
        res.url().includes("/host-answer") &&
        res.ok(),
    );
    await questionCard.getByRole("button", { name: /Send answer/i }).click();
    await answerPromise;
    await expect(page.getByText(question)).toHaveCount(0);

    await visitPublicLink(page, linkShortUrl);
    const openSidebarAgain = page.getByRole("button", { name: /Open sidebar/i });
    if (await openSidebarAgain.isVisible().catch(() => false)) {
      await openSidebarAgain.click();
    }
    const inputAgain = page.getByPlaceholder(/materials you can access|Ask the host/i);
    if (!(await inputAgain.isVisible().catch(() => false))) {
      const askTab = page.locator("button.rounded-full").filter({ hasText: /^Ask$/ });
      if (await askTab.isVisible().catch(() => false)) {
        await askTab.click();
      }
    }
    await expect(page.getByText(answer)).toBeVisible({ timeout: 20000 });
  });
});
