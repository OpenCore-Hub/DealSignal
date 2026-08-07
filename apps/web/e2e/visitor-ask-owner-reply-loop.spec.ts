/**
 * Spec 459 — Visitor Ask closed loop (MSW): visitor submits → owner replies → visitor sees answer.
 */
import { test, expect } from "@playwright/test";
import { resetMockState, attachDebug, openVisitorAskPanel, gotoAuthenticatedWaitForApi, WORKSPACE_SLUG } from "./helpers";

const SMOKE_TOKEN = "AskSmoke1";
const VISITOR_QUESTION = "E2E closed loop: when is the next update?";
const HOST_ANSWER = "We will publish an update next Monday.";

test.describe("Visitor Ask owner reply loop (MSW)", () => {
  test.setTimeout(60_000);

  test("visitor ask → owner reply → visitor sees answer", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);

    const hostInput = await openVisitorAskPanel(page, SMOKE_TOKEN);
    await hostInput.fill(VISITOR_QUESTION);
    await page
      .getByRole("button", { name: "Ask", exact: true })
      .and(page.locator('[type="submit"]'))
      .click();

    await expect(page.getByText(VISITOR_QUESTION)).toBeVisible({ timeout: 10000 });
    await expect(page.getByText(/Awaiting reply/i)).toBeVisible({ timeout: 10000 });

    await gotoAuthenticatedWaitForApi(
      page,
      `/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=qa`,
      "/deal-rooms/room_1/ask",
    );
    await expect(page.getByText("Ask inbox")).toBeVisible({ timeout: 15000 });
    await expect(page.getByText(VISITOR_QUESTION)).toBeVisible({ timeout: 10000 });

    const questionCard = page.locator("li").filter({ hasText: VISITOR_QUESTION });
    await questionCard.getByPlaceholder(/Type your answer/i).fill(HOST_ANSWER);
    const answerPromise = page.waitForResponse(
      (res) =>
        res.request().method() === "PATCH" &&
        res.url().includes("/host-answer") &&
        res.ok(),
    );
    await questionCard.getByRole("button", { name: /Send answer/i }).click();
    await answerPromise;
    await expect(page.getByText(VISITOR_QUESTION)).toHaveCount(0);

    await openVisitorAskPanel(page, SMOKE_TOKEN);
    await expect(page.getByText(HOST_ANSWER)).toBeVisible({ timeout: 15000 });
  });
});
