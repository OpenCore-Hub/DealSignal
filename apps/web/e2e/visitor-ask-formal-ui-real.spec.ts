/**
 * Formal Q&A owner publish → visitor Published Q&A board (live API + Vite UI).
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
  updateLinkAskPolicy,
  authenticatePage,
  openRealVisitorAskPanel,
  attachDebug,
} from "./real-helpers";
import { ASK_INBOX_TITLE, FORMAL_QUEUE_TAB, PUBLISHED_QA_REGION } from "./helpers";

let workspaceSlug: string;
let roomId: string;
let linkShortUrl: string;

test.describe("Visitor Ask Formal Q&A (real backend UI)", () => {
  test.beforeAll(async () => {
    test.setTimeout(180_000);
    const seed = await seedRealBackend();
    workspaceSlug = seed.workspaceSlug;
    const doc = await seedDocument(workspaceSlug);
    const room = await seedDealRoom(workspaceSlug, {
      name: "Visitor Ask Formal UI Room",
      templateType: "seed",
      documentIds: [doc.id],
    });
    roomId = room.id;
    const link = await seedDealRoomLink(workspaceSlug, room.id, {
      name: `Ask Formal UI Link ${Date.now()}`,
    });
    linkShortUrl = link.shortUrl;
    await updateLinkAskPolicy(workspaceSlug, link.id, { askMode: "formal" });
  });

  test("visitor formal ask → owner publish now → visitor Published Q&A", async ({ page }) => {
    test.setTimeout(180_000);
    attachDebug(page);

    const question = `UI formal disclosure ${Date.now()}?`;
    const answer = "UI formal answer: board-approved mid-teens guidance.";

    const input = await openRealVisitorAskPanel(page, linkShortUrl);
    await input.fill(question);
    await page
      .getByRole("button", { name: "Ask", exact: true })
      .and(page.locator('[type="submit"]'))
      .click();
    await expect(page.getByText(question)).toBeVisible({ timeout: 15000 });
    await expect(
      page.getByText(/Under formal review|Formal 审核中/i),
    ).toBeVisible({ timeout: 15000 });

    await authenticatePage(page);
    await page.goto(`/${workspaceSlug}/deal-rooms/${roomId}?tab=qa&askInbox=formal_queue`);
    await expect(page.getByText(ASK_INBOX_TITLE)).toBeVisible({
      timeout: 20000,
    });
    await expect(page.getByRole("tab", { name: FORMAL_QUEUE_TAB })).toHaveAttribute(
      "aria-selected",
      "true",
      { timeout: 10000 },
    );
    await expect(page.getByText(question)).toBeVisible({ timeout: 15000 });

    const card = page.locator("li").filter({ hasText: question });
    await card.getByPlaceholder(/approved public answer/i).fill(answer);
    const publishPromise = page.waitForResponse(
      (res) =>
        res.request().method() === "PATCH" &&
        res.url().includes("/formal-publish") &&
        res.ok(),
    );
    await card.getByRole("button", { name: /Publish now|立即发布/i }).click();
    await publishPromise;

    await expect(page.getByRole("tabpanel", { name: FORMAL_QUEUE_TAB })).toContainText(
      /No formal questions awaiting review|暂无待审核 Formal 问题/,
      { timeout: 15000 },
    );

    await openRealVisitorAskPanel(page, linkShortUrl);
    const formalSection = page.getByRole("region", { name: PUBLISHED_QA_REGION });
    await expect(formalSection).toBeVisible({ timeout: 20000 });
    await expect(formalSection).toContainText(question);
    await formalSection.getByRole("button", { name: question }).click();
    await expect(formalSection).toContainText(answer);
    await expect(formalSection.getByText(/Asked by|提问者/i)).toHaveCount(0);
  });
});
