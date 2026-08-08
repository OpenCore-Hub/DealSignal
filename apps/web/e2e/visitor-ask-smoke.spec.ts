/**
 * Visitor Ask smoke (MSW) — Ask Host empty state, submit question, pending badge.
 */
import { test, expect } from "@playwright/test";
import { resetMockState, attachDebug, openVisitorAskPanel, setMockLinkAskPolicy } from "./helpers";

const SMOKE_TOKEN = "AskSmoke1";
const SMOKE_LINK_ID = "link_visitor_ask_smoke";

test.describe("Visitor Ask smoke (MSW)", () => {
  test("Ask Host empty → submit → awaiting reply", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);
    await setMockLinkAskPolicy(page, SMOKE_LINK_ID, { askAiEnabled: false });

    const hostInput = await openVisitorAskPanel(page, SMOKE_TOKEN);

    await hostInput.fill("__host__ Can you share the full model?");
    await page.getByRole("button", { name: "Ask", exact: true }).and(page.locator('[type="submit"]')).click();

    await expect(page.getByText("Can you share the full model?")).toBeVisible({ timeout: 10000 });
    await expect(page.getByText(/Awaiting reply/i)).toBeVisible({ timeout: 10000 });
  });

  test("Ask Host rate limit shows distinct error", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);

    const hostInput = await openVisitorAskPanel(page, SMOKE_TOKEN);

    await hostInput.fill("__rate_limit__ spam");
    await page.getByRole("button", { name: "Ask", exact: true }).and(page.locator('[type="submit"]')).click();

    await expect(page.getByText(/Too many questions/i)).toBeVisible({ timeout: 10000 });
  });
});
