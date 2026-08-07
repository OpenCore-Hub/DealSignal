/**
 * Owner enables grounded AI (Engage UI) → visitor receives AI answer in viewer UI.
 * Optional: requires docling-rag on API (same skip as visitor-ask-ai-stream-real).
 *
 * Run:
 *   REAL_API_BASE_URL=http://localhost:8090 ./e2e-visitor-ask-real.sh --ai
 */
import { test, expect } from "@playwright/test";
import {
  seedRealBackend,
  seedDocument,
  seedDealRoom,
  seedDealRoomLink,
  waitForKnowledgeCorpusReady,
  probeKnowledgeEnabled,
  fetchLinkById,
  enableGroundedAiInEngageTab,
  openRealVisitorAskPanel,
  attachDebug,
} from "./real-helpers";

let workspaceSlug: string;
let roomId: string;
let linkId = "";
let linkShortUrl = "";
let knowledgeEnabled = false;

const AI_QUESTION = "What is the valuation cap?";

test.describe("Visitor Ask AI UI loop (real backend)", () => {
  test.beforeAll(async () => {
    test.setTimeout(300_000);
    const seed = await seedRealBackend();
    workspaceSlug = seed.workspaceSlug;
    const doc = await seedDocument(workspaceSlug);
    const room = await seedDealRoom(workspaceSlug, {
      name: "Visitor Ask AI UI Room",
      templateType: "seed",
      documentIds: [doc.id],
    });
    roomId = room.id;

    knowledgeEnabled = await probeKnowledgeEnabled(workspaceSlug, roomId);
    if (!knowledgeEnabled) return;

    await waitForKnowledgeCorpusReady(workspaceSlug, roomId, 180);
    const link = await seedDealRoomLink(workspaceSlug, room.id, {
      name: `Ask AI UI Link ${Date.now()}`,
    });
    linkId = link.id;
    linkShortUrl = link.shortUrl;

    const detail = await fetchLinkById(workspaceSlug, linkId);
    expect(detail.askAiEnabled).not.toBe(true);
  });

  test("owner enables AI in Engage then visitor UI shows grounded answer", async ({ page }) => {
    test.skip(!knowledgeEnabled, "docling-rag not configured on API");
    test.setTimeout(240_000);
    attachDebug(page);

    await enableGroundedAiInEngageTab(page, { workspaceSlug, roomId, linkId });
    const enabled = await fetchLinkById(workspaceSlug, linkId);
    expect(enabled.askAiEnabled).toBe(true);

    await page.context().clearCookies();
    const input = await openRealVisitorAskPanel(page, linkShortUrl);
    await input.fill(AI_QUESTION);
    await page
      .getByRole("button", { name: "Ask", exact: true })
      .and(page.locator('[type="submit"]'))
      .click();

    await expect(page.getByText(AI_QUESTION)).toBeVisible({ timeout: 20000 });
    await expect(page.getByText(/AI Ask is temporarily unavailable/i)).toHaveCount(0);
    await expect(page.getByText(/Awaiting reply/i)).toHaveCount(0, { timeout: 180_000 });
    await expect(page.getByText(/\d+ sources|source/i).first()).toBeVisible({ timeout: 180_000 });
  });
});
