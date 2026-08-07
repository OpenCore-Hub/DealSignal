/**
 * Spec 459 — Visitor Ask closed loop (MSW): visitor submits → owner replies → visitor sees answer.
 */
import { test, expect } from "@playwright/test";
import { resetMockState, setupAuthenticatedPage, attachDebug, WORKSPACE_SLUG } from "./helpers";

const SMOKE_TOKEN = "AskSmoke1";
const VISITOR_QUESTION = "E2E closed loop: when is the next update?";
const HOST_ANSWER = "We will publish an update next Monday.";

test.describe("Visitor Ask owner reply loop (MSW)", () => {
  test("visitor ask → owner reply → visitor sees answer", async ({ browser }) => {
    const visitorContext = await browser.newContext();
    const ownerContext = await browser.newContext();
    const visitorPage = await visitorContext.newPage();
    const ownerPage = await ownerContext.newPage();

    attachDebug(visitorPage);
    attachDebug(ownerPage);

    await resetMockState(visitorPage);

    await visitorPage.goto(`/l/${SMOKE_TOKEN}`);
    await expect(visitorPage.locator("img[alt*='Page']").first()).toBeVisible({ timeout: 15000 });

    const openSidebar = visitorPage.getByRole("button", { name: /Open sidebar/i });
    if (await openSidebar.isVisible().catch(() => false)) {
      await openSidebar.click();
    }
    await visitorPage.locator('button[type="button"]').filter({ hasText: /^Ask$/ }).click();

    const hostInput = visitorPage.getByPlaceholder(/Ask the host a question/i);
    await expect(hostInput).toBeVisible({ timeout: 5000 });
    await hostInput.fill(VISITOR_QUESTION);
    await visitorPage
      .getByRole("button", { name: "Ask", exact: true })
      .and(visitorPage.locator('[type="submit"]'))
      .click();

    await expect(visitorPage.getByText(VISITOR_QUESTION)).toBeVisible({ timeout: 10000 });
    await expect(visitorPage.getByText(/Awaiting reply/i)).toBeVisible({ timeout: 10000 });

    await setupAuthenticatedPage(ownerPage);
    await ownerPage.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=qa`);
    await expect(ownerPage.getByText("Ask Host inbox")).toBeVisible({ timeout: 15000 });
    await expect(ownerPage.getByText(VISITOR_QUESTION)).toBeVisible({ timeout: 10000 });

    await ownerPage.getByPlaceholder(/Type your answer/i).fill(HOST_ANSWER);
    await ownerPage.getByRole("button", { name: /Send answer/i }).click();
    await expect(ownerPage.getByText(HOST_ANSWER)).toBeVisible({ timeout: 10000 });

    await visitorPage.reload();
    await expect(visitorPage.locator("img[alt*='Page']").first()).toBeVisible({ timeout: 15000 });
    if (await openSidebar.isVisible().catch(() => false)) {
      await openSidebar.click();
    }
    await visitorPage.locator('button[type="button"]').filter({ hasText: /^Ask$/ }).click();
    await expect(visitorPage.getByText(HOST_ANSWER)).toBeVisible({ timeout: 15000 });

    await visitorContext.close();
    await ownerContext.close();
  });
});
