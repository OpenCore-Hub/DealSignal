/**
 * Visitor Ask smoke (MSW) — dual-channel empty prompts, switch to Ask Host, pending badge.
 */
import { test, expect } from "@playwright/test";
import { resetMockState, attachDebug } from "./helpers";

const SMOKE_TOKEN = "AskSmoke1";

test.describe("Visitor Ask smoke (MSW)", () => {
  test("dual-on empty → switch Ask Host → awaiting reply", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);

    await page.goto(`/l/${SMOKE_TOKEN}`);
    await expect(page.locator("img[alt*='Page']").first()).toBeVisible({ timeout: 15000 });

    await page.getByRole("button", { name: /Open sidebar/i }).click();
    // Sidebar tab is type=button; composer submit is type=submit aria-label="Ask".
    const askTab = page.locator('button[type="button"]').filter({ hasText: /^Ask$/ });
    await expect(askTab).toBeVisible({ timeout: 10000 });
    await askTab.click();

    await expect(
      page.getByText(/Ask Docs first; switch to Ask Host if you need missing materials/i),
    ).toBeVisible({ timeout: 10000 });
    await expect(
      page.getByRole("button", { name: /Summarize key points from authorized materials/i }),
    ).toBeVisible();
    await expect(page.getByRole("link", { name: /file request/i })).toHaveCount(0);

    await page.getByRole("button", { name: /Materials seem to be missing/i }).click();
    const hostInput = page.getByPlaceholder(/Ask the host a question/i);
    await expect(hostInput).toBeVisible({ timeout: 5000 });

    await hostInput.fill("Can you share the full model?");
    await page.getByRole("button", { name: "Ask", exact: true }).and(page.locator('[type="submit"]')).click();

    await expect(page.getByText("Can you share the full model?")).toBeVisible({ timeout: 10000 });
    await expect(page.getByText(/Awaiting reply/i)).toBeVisible({ timeout: 10000 });
  });
});
