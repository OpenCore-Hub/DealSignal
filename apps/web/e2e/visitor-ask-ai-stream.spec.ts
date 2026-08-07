/**
 * Phase B — Visitor Ask AI lane (MSW): submit → SSE stream → grounded answer + evidence;
 * owner can review in AI handled inbox.
 */
import { test, expect } from "@playwright/test";
import {
  resetMockState,
  attachDebug,
  openVisitorAskPanel,
  gotoAuthenticated,
  WORKSPACE_SLUG,
} from "./helpers";

const SMOKE_TOKEN = "AskSmoke1";
const AI_QUESTION = "__ai__ What was revenue growth last year?";
const SLOW_AI_QUESTION = "__slow__ __ai__ What was revenue growth last year?";
const DISPLAY_QUESTION = "What was revenue growth last year?";
const AI_ANSWER_SNIPPET = /revenue grew 12%/i;
const EVIDENCE_SNIPPET = /Revenue increased 12% YoY/i;

test.describe("Visitor Ask AI stream (MSW)", () => {
  test("visitor receives streamed AI answer with evidence", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);

    const input = await openVisitorAskPanel(page, SMOKE_TOKEN);
    await input.fill(AI_QUESTION);
    await page
      .getByRole("button", { name: /Ask/i })
      .and(page.locator('[type="submit"]'))
      .click();

    await expect(page.getByText(DISPLAY_QUESTION)).toBeVisible({ timeout: 10000 });
    await expect(page.getByText(AI_ANSWER_SNIPPET)).toBeVisible({ timeout: 15000 });
    await expect(page.getByText(EVIDENCE_SNIPPET)).toBeVisible({ timeout: 10000 });
    await expect(page.getByText("AI", { exact: true }).first()).toBeVisible();
  });

  test("visitor can jump to cited page from AI evidence", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);

    const input = await openVisitorAskPanel(page, SMOKE_TOKEN);
    await input.fill(AI_QUESTION);
    await page
      .getByRole("button", { name: /Ask/i })
      .and(page.locator('[type="submit"]'))
      .click();

    await expect(page.getByText(AI_ANSWER_SNIPPET)).toBeVisible({ timeout: 15000 });
    await page.getByRole("button", { name: /Open page 3 in Content/i }).click();
    await expect(page.getByText("3/18")).toBeVisible({ timeout: 10000 });
  });

  test("visitor can stop an in-flight AI stream", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);

    const input = await openVisitorAskPanel(page, SMOKE_TOKEN);
    await input.fill(SLOW_AI_QUESTION);
    await page
      .getByRole("button", { name: /Ask/i })
      .and(page.locator('[type="submit"]'))
      .click();

    await expect(page.getByText(/Searching your authorized documents|正在检索/i)).toBeVisible({
      timeout: 10000,
    });
    await page.getByRole("button", { name: /^Stop$|停止/i }).click();
    await expect(page.getByText(/Generation stopped|已停止生成/i)).toBeVisible({
      timeout: 10000,
    });
  });

  test("owner sees AI-handled turn in link Engage inbox", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);

    const input = await openVisitorAskPanel(page, SMOKE_TOKEN);
    await input.fill(AI_QUESTION);
    await page
      .getByRole("button", { name: /Ask/i })
      .and(page.locator('[type="submit"]'))
      .click();
    await expect(page.getByText(AI_ANSWER_SNIPPET)).toBeVisible({ timeout: 15000 });

    await gotoAuthenticated(page, `/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=links`);
    await page.getByTestId("deal-room-link-row-link_visitor_ask_smoke").click();
    await expect(page.getByRole("dialog")).toBeVisible({ timeout: 10000 });
    const engageInboxPromise = page.waitForResponse(
      (res) =>
        res.request().method() === "GET" &&
        res.url().includes("/links/link_visitor_ask_smoke/ask") &&
        res.ok(),
    );
    await page.getByRole("tab", { name: /Engage/i }).click();
    await engageInboxPromise;
    const aiInboxPromise = page.waitForResponse(
      (res) =>
        res.request().method() === "GET" &&
        res.url().includes("/links/link_visitor_ask_smoke/ask") &&
        res.url().includes("lane=ai") &&
        res.ok(),
    );
    await page.getByRole("tab", { name: /AI handled/i }).click();
    await aiInboxPromise;

    await expect(page.getByText(DISPLAY_QUESTION)).toBeVisible({ timeout: 15000 });
    await expect(page.getByText(AI_ANSWER_SNIPPET)).toBeVisible({ timeout: 10000 });
    await page.getByRole("button", { name: /Open page 3/i }).click();
    await expect(page).toHaveURL(/\/viewer\/doc_1\?.*page=3/, { timeout: 10000 });
  });
});
