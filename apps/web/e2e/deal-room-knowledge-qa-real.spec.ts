/**
 * Deal-room knowledge Q&A against a live API + docling-rag.
 *
 * Prefers API smoke (`apps/api/e2e-knowledge.sh`) for CI-ish gates; this covers
 * the research-desk UI path (ask → refresh hydrate → feedback).
 *
 * Run:
 *   REAL_API_BASE_URL=http://localhost:8090 ./e2e-knowledge-real.sh
 */
import { test, expect } from "@playwright/test";
import {
  seedRealBackend,
  seedDocument,
  seedDealRoom,
  waitForKnowledgeCorpusReady,
  authenticatePage,
  attachDebug,
  apiFetch,
} from "./real-helpers";

let workspaceSlug: string;
let roomId: string;
let knowledgeEnabled = true;

test.describe("Deal room knowledge Q&A (real backend)", () => {
  test.beforeAll(async () => {
    test.setTimeout(240_000);
    const seed = await seedRealBackend();
    workspaceSlug = seed.workspaceSlug;
    const doc = await seedDocument(workspaceSlug);
    const room = await seedDealRoom(workspaceSlug, {
      name: "Knowledge Real Smoke",
      templateType: "seed",
      documentIds: [doc.id],
    });
    roomId = room.id;

    const probe = await apiFetch(`/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/knowledge`);
    if (!probe.ok) {
      throw new Error(`knowledge probe failed: ${probe.status} ${await probe.text()}`);
    }
    const corpus = (await probe.json()) as { enabled: boolean };
    if (!corpus.enabled) {
      knowledgeEnabled = false;
      return;
    }
    await waitForKnowledgeCorpusReady(workspaceSlug, roomId, 180);
  });

  test("asks on desk, recovers after refresh, and keeps feedback", async ({ page }) => {
    test.skip(!knowledgeEnabled, "knowledge disabled on API");
    test.setTimeout(120_000);
    attachDebug(page);
    await authenticatePage(page);

    await page.goto(`/${workspaceSlug}/deal-rooms/${roomId}?tab=knowledge`);
    await expect(page.getByTestId("deal-room-knowledge-corpus")).toBeVisible({
      timeout: 20000,
    });

    const start = page.getByTestId("deal-room-knowledge-ask-entry-start");
    await expect(start).toBeEnabled({ timeout: 20000 });
    await start.click();
    await expect(page.getByTestId("deal-room-knowledge-desk")).toBeVisible();

    const question = "What is the valuation cap?";
    const composer = page.getByLabel("Question");
    const ask = page.getByTestId("deal-room-knowledge-ask");
    await expect(ask).toBeVisible({ timeout: 15000 });
    await composer.fill(question);
    await expect(ask).toBeEnabled();
    await ask.click();

    const turn = page.getByTestId("grounded-chat-turn").last();
    await expect(turn).toContainText(question, { timeout: 60000 });
    await expect(ask).toBeVisible({ timeout: 60000 });

    await page.getByTestId("knowledge-feedback-helpful").click();
    await expect(page.getByTestId("knowledge-feedback-helpful")).toHaveAttribute(
      "aria-pressed",
      "true",
    );

    await page.reload();
    await expect(page.getByTestId("deal-room-knowledge-desk")).toBeVisible({
      timeout: 20000,
    });
    await expect(page.getByTestId("grounded-chat-turn").first()).toContainText(question, {
      timeout: 20000,
    });
    await expect(page.getByTestId("knowledge-feedback-helpful")).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });
});
