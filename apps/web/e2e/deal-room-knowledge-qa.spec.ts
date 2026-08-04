/**
 * Deal-room knowledge Q&A audit (MSW) — Phase A / B / C acceptance.
 * Covers: ask → session turn → feedback → hard refresh still hydrated;
 * citation open → browser Back; multi-turn; refuse without evidence.
 */
import { readFile } from "node:fs/promises";
import { test, expect, type Page } from "@playwright/test";
import {
  setupAuthenticatedPage,
  attachDebug,
  setMockKnowledgeAskGate,
  setMockKnowledgeCorpus,
  WORKSPACE_SLUG,
} from "./helpers";

async function openKnowledgeDesk(page: Page) {
  await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=knowledge`);
  await expect(page.getByTestId("deal-room-knowledge-corpus")).toBeVisible({
    timeout: 15000,
  });
  const start = page.getByTestId("deal-room-knowledge-ask-entry-start");
  await expect(start).toBeEnabled({ timeout: 10000 });
  await start.click();
  await expect(page.getByTestId("deal-room-knowledge-desk")).toBeVisible();
}

async function askOnDesk(page: Page, question: string) {
  const composer = page.getByLabel("Question");
  const ask = page.getByTestId("deal-room-knowledge-ask");
  // Idle = Ask button present (Stop swaps the same slot while streaming).
  // Do NOT wait for enabled before fill — empty composer keeps Ask disabled.
  await expect(ask).toBeVisible({ timeout: 15000 });
  await composer.fill(question);
  await expect(composer).toHaveValue(question);
  await expect(ask).toBeEnabled();
  await ask.click();
  await expect(page.getByTestId("grounded-chat-turn").last()).toContainText(question, {
    timeout: 15000,
  });
  // Stream finished: Ask control is back (may be disabled until next fill).
  await expect(ask).toBeVisible({ timeout: 15000 });
}

test.describe("Deal room knowledge Q&A (MSW)", () => {
  test("asks, leaves feedback, and recovers after refresh", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=knowledge`);
    await expect(page.getByTestId("deal-room-knowledge-corpus")).toBeVisible({
      timeout: 15000,
    });
    const start = page.getByTestId("deal-room-knowledge-ask-entry-start");
    await expect(start).toBeEnabled({ timeout: 10000 });
    await start.click();

    await expect(page.getByTestId("deal-room-knowledge-desk")).toBeVisible();
    await page.getByLabel("Question").fill("What is the valuation cap?");
    await page.getByTestId("deal-room-knowledge-ask").click();

    await expect(page.getByText(/Grounded answer for: What is the valuation cap/i)).toBeVisible({
      timeout: 10000,
    });
    await expect(page.getByTestId("grounded-chat-follow-ups")).toBeVisible();
    await expect(page.getByTestId("deal-room-knowledge-hit")).toBeVisible();

    await page.getByTestId("knowledge-feedback-wrong_citation").click();
    await expect(page.getByTestId("knowledge-feedback-wrong_citation")).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    await page.getByTestId("knowledge-feedback-note").fill("page locus looks off");
    await page.getByTestId("knowledge-feedback-note").blur();
    await expect(page.getByTestId("knowledge-feedback-note")).toHaveValue("page locus looks off");

    // Hard refresh — Zustand cleared; hydrate from MSW active session (A1 + C1).
    await page.reload();
    await expect(page.getByTestId("deal-room-knowledge-desk")).toBeVisible({
      timeout: 15000,
    });
    const turn = page.getByTestId("grounded-chat-turn");
    await expect(turn).toHaveCount(1);
    await expect(turn).toContainText("What is the valuation cap?");
    await expect(turn).toContainText(/Grounded answer for: What is the valuation cap/i);
    await expect(page.getByTestId("knowledge-feedback-wrong_citation")).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    await expect(page.getByTestId("knowledge-feedback-note")).toHaveValue("page locus looks off");

    // Switch feedback kind after refresh.
    await page.getByTestId("knowledge-feedback-helpful").click();
    await expect(page.getByTestId("knowledge-feedback-helpful")).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  test("hides feedback controls when the desk has no turns", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=knowledge`);
    const start = page.getByTestId("deal-room-knowledge-ask-entry-start");
    await expect(start).toBeEnabled({ timeout: 10000 });
    await start.click();
    await expect(page.getByTestId("deal-room-knowledge-desk")).toBeVisible();
    await expect(page.getByTestId("grounded-chat-dock")).toBeVisible();
    await expect(page.locator("[data-testid^='knowledge-turn-feedback-']")).toHaveCount(0);
  });

  test("refuses without evidence rail (A2)", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await openKnowledgeDesk(page);
    await page.getByLabel("Question").fill("@refuse What is the secret sauce?");
    await page.getByTestId("deal-room-knowledge-ask").click();

    await expect(page.getByText(/does not contain an answer/i)).toBeVisible({
      timeout: 10000,
    });
    await expect(page.getByTestId("deal-room-knowledge-hit")).toHaveCount(0);
  });

  test("opens citation page and restores desk after browser Back (A3)", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await openKnowledgeDesk(page);
    await askOnDesk(page, "What is the valuation cap?");
    await expect(page.getByTestId("deal-room-knowledge-jump")).toBeVisible();

    await page.getByTestId("deal-room-knowledge-jump").click();
    await expect(page).toHaveURL(/\/viewer\/[^/?]+\?/, { timeout: 15000 });
    await expect(page).toHaveURL(/page=3/);
    await expect(page).toHaveURL(/roomId=room_1/);
    await expect(page).toHaveURL(new RegExp(`ws=${WORKSPACE_SLUG}`));
    await expect(page.getByTestId("viewer-knowledge-rail")).toBeVisible({
      timeout: 10000,
    });
    await expect(page.getByTestId("viewer-knowledge-trust-chip")).toBeVisible();

    await page.goBack();
    await expect(page).toHaveURL(/deal-rooms\/room_1.*tab=knowledge/, {
      timeout: 15000,
    });
    await expect(page.getByTestId("deal-room-knowledge-desk")).toBeVisible({
      timeout: 15000,
    });
    const turn = page.getByTestId("grounded-chat-turn");
    await expect(turn).toHaveCount(1);
    await expect(turn).toContainText("What is the valuation cap?");
    await expect(turn).toContainText(/Grounded answer for: What is the valuation cap/i);
    await expect(page.getByTestId("deal-room-knowledge-hit")).toBeVisible();
  });

  test("keeps two turns in one session (B1)", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await openKnowledgeDesk(page);
    await askOnDesk(page, "What is the valuation cap?");
    await expect(page.getByTestId("grounded-chat-turn")).toHaveCount(1);

    await askOnDesk(page, "What about the option pool?");
    await expect(page.getByTestId("grounded-chat-turn")).toHaveCount(2);
    await expect(page.getByTestId("grounded-chat-dock")).toBeVisible();
    await expect(page.getByTestId("grounded-chat-follow-ups")).toBeVisible();

    const turns = page.getByTestId("grounded-chat-turn");
    await expect(turns.nth(0)).toContainText("What is the valuation cap?");
    await expect(turns.nth(1)).toContainText("What about the option pool?");
  });

  test("sends a room-scoped follow-up chip immediately (B2)", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await openKnowledgeDesk(page);
    await askOnDesk(page, "What is the valuation cap?");
    const chip = page.getByTestId("grounded-chat-follow-up-liability-in-source");
    await expect(chip).toBeVisible();
    await expect(chip).toContainText(/Acme Seed Round Pitch Deck/i);

    await chip.click();
    // Chip auto-asks — same session gains a second turn without clicking Ask.
    await expect(page.getByTestId("grounded-chat-turn")).toHaveCount(2, { timeout: 15000 });
    await expect(page.getByTestId("grounded-chat-turn").nth(1)).toContainText(/liability/i);
  });

  test("surfaces answer-quota 429 before SSE opens", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);
    await setMockKnowledgeAskGate(page, {
      code: "knowledge_query_quota_exceeded",
      httpStatus: 429,
    });

    await openKnowledgeDesk(page);
    await page.getByLabel("Question").fill("What is the valuation cap?");
    await page.getByTestId("deal-room-knowledge-ask").click();

    // Prefer role=status (sonner); fall back to copy match for locale variants.
    await expect(
      page.getByText(/answer quota is used up|回答额度已用完/i),
    ).toBeVisible({ timeout: 10000 });
    await expect(page.getByTestId("grounded-chat-turn")).toHaveCount(0);
    // Ask control returns (not stuck on Stop).
    await expect(page.getByTestId("deal-room-knowledge-ask")).toBeVisible();
  });

  test("disables start-ask when corpus is not ready (A5)", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    // room_2 has an empty document tree → corpus stage "empty" (not ready).
    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_2?tab=knowledge`);
    await expect(page.getByTestId("deal-room-knowledge-corpus")).toBeVisible({
      timeout: 15000,
    });
    await expect(page.getByTestId("deal-room-knowledge-corpus")).toHaveAttribute(
      "data-corpus-stage",
      "empty",
    );
    const start = page.getByTestId("deal-room-knowledge-ask-entry-start");
    await expect(start).toBeVisible();
    await expect(start).toBeDisabled();
    await expect(page.getByTestId("deal-room-knowledge-desk")).toHaveCount(0);

    // Syncing override (building) also keeps the CTA disabled.
    await setMockKnowledgeCorpus(page, "room_1", {
      status: "syncing",
      documentStatus: "syncing",
      jobStatus: "running",
    });
    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=knowledge`);
    await expect(page.getByTestId("deal-room-knowledge-corpus")).toBeVisible({
      timeout: 15000,
    });
    await expect(page.getByTestId("deal-room-knowledge-corpus")).toHaveAttribute(
      "data-corpus-stage",
      "building",
    );
    await expect(page.getByTestId("deal-room-knowledge-ask-entry-start")).toBeDisabled();
  });

  test("starts a new session and can reopen the previous one (A4)", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await openKnowledgeDesk(page);
    await askOnDesk(page, "What is the valuation cap?");
    await expect(page.getByTestId("grounded-chat-turn")).toHaveCount(1);

    await page.getByTestId("deal-room-knowledge-new-session").click();
    await expect(page.getByTestId("grounded-chat-turn")).toHaveCount(0);
    await expect(page.getByTestId("grounded-chat-dock")).toBeVisible();

    await askOnDesk(page, "What about the option pool?");
    await expect(page.getByTestId("grounded-chat-turn")).toHaveCount(1);
    await expect(page.getByTestId("grounded-chat-turn")).toContainText(
      "What about the option pool?",
    );
    await expect(page.getByTestId("grounded-chat-turn")).not.toContainText(
      "What is the valuation cap?",
    );

    await page.getByTestId("deal-room-knowledge-session-history").click();
    const historyItems = page.locator('[data-testid^="deal-room-knowledge-session-kqa_sess"]');
    await expect(historyItems).toHaveCount(2, { timeout: 10000 });
    // Newest-first: open the closed prior session (second row).
    await historyItems.nth(1).click();

    await expect(page.getByTestId("grounded-chat-turn")).toHaveCount(1, { timeout: 10000 });
    await expect(page.getByTestId("grounded-chat-turn")).toContainText(
      "What is the valuation cap?",
    );
    await expect(page.getByTestId("grounded-chat-turn")).toContainText(
      /Grounded answer for: What is the valuation cap/i,
    );
  });

  test("owner viewer with roomId mounts knowledge rail without cite (Phase X)", async ({
    page,
  }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/viewer/doc_1?roomId=room_1&ws=${WORKSPACE_SLUG}`);
    await expect(page.getByTestId("viewer-knowledge-rail")).toBeVisible({
      timeout: 15000,
    });
    await expect(page.getByTestId("viewer-knowledge-trust-chip")).toBeVisible();

    await page.getByLabel("Question").fill("What is the valuation cap?");
    await page.getByTestId("deal-room-knowledge-ask").click();
    await expect(page.getByTestId("grounded-chat-turn").last()).toContainText(
      "What is the valuation cap?",
      { timeout: 15000 },
    );
  });

  test("owner viewer without roomId does not mount knowledge rail", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/viewer/doc_1?ws=${WORKSPACE_SLUG}`);
    await expect(page.getByTestId("viewer-knowledge-rail")).toHaveCount(0, {
      timeout: 10000,
    });
    await expect(page.getByText(/Failed to load|No workspace selected/i)).toHaveCount(0);
  });

  test("cold archive: list → view preview → download pack (Phase U)", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=knowledge`);
    await expect(page.getByTestId("deal-room-knowledge-corpus")).toBeVisible({
      timeout: 15000,
    });
    const archives = page.getByTestId("knowledge-cold-archives");
    await expect(archives).toBeVisible({ timeout: 10000 });
    await expect(page.getByTestId("knowledge-cold-archives-list")).toBeVisible();

    await page.getByTestId("knowledge-cold-archive-open-kqa_arch_1").click();
    await expect(page.getByTestId("knowledge-cold-archive-preview")).toBeVisible({
      timeout: 10000,
    });
    await expect(page.getByTestId("knowledge-cold-archive-preview")).toContainText(
      /valuation cap/i,
    );

    const [download] = await Promise.all([
      page.waitForEvent("download"),
      page.getByTestId("knowledge-cold-archive-download-kqa_arch_1").click(),
    ]);
    expect(download.suggestedFilename()).toMatch(/diligence-archive-.*\.json$/);
  });

  test("gold review: wrong_citation → accept → export seeds JSON (Phase O/Q/R)", async ({
    page,
  }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await openKnowledgeDesk(page);
    await askOnDesk(page, "What is the purchase price?");
    await expect(page.getByTestId("deal-room-knowledge-hit")).toBeVisible({
      timeout: 10000,
    });

    await page.getByTestId("knowledge-feedback-wrong_citation").click();
    await expect(page.getByTestId("knowledge-feedback-wrong_citation")).toHaveAttribute(
      "aria-pressed",
      "true",
    );

    await page.getByTestId("deal-room-knowledge-back-to-corpus").click();
    await expect(page.getByTestId("deal-room-knowledge-corpus")).toBeVisible({
      timeout: 10000,
    });

    const gold = page.getByTestId("knowledge-eval-gold-review");
    await expect(gold).toBeVisible({ timeout: 10000 });
    await expect(page.getByTestId("knowledge-eval-gold-list")).toBeVisible();

    const accept = page.locator('[data-testid^="knowledge-eval-gold-accept-"]').first();
    await expect(accept).toBeVisible();
    await accept.click();

    const exportBtn = page.getByTestId("knowledge-eval-gold-export");
    await expect(exportBtn).toBeVisible({ timeout: 10000 });
    await expect(exportBtn).toBeEnabled();

    const [download] = await Promise.all([
      page.waitForEvent("download"),
      exportBtn.click(),
    ]);
    expect(download.suggestedFilename()).toMatch(/knowledge-eval-seeds-.*\.json$/);

    const filePath = await download.path();
    expect(filePath).toBeTruthy();
    const pack = JSON.parse(await readFile(filePath!, "utf8")) as {
      seeds?: Array<{ kind?: string; expect?: string; question?: string }>;
    };
    expect(pack.seeds?.length).toBeGreaterThanOrEqual(1);
    expect(pack.seeds?.[0]?.kind).toBe("wrong_citation");
    expect(pack.seeds?.[0]?.expect).toBe("reject_or_rebind");
  });
});
