/**
 * Visitor Ask smoke (MSW) — Ask Host empty state, submit question, pending badge.
 */
import { test, expect } from "@playwright/test";
import { resetMockState, attachDebug } from "./helpers";

const SMOKE_TOKEN = "AskSmoke1";

test.describe("Visitor Ask smoke (MSW)", () => {
  test("Ask Host empty → submit → awaiting reply", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);

    await page.goto(`/l/${SMOKE_TOKEN}`);
    await expect(page.locator("img[alt*='Page']").first()).toBeVisible({ timeout: 15000 });

    const openSidebar = page.getByRole("button", { name: /Open sidebar/i });
    if (await openSidebar.isVisible().catch(() => false)) {
      await openSidebar.click();
    }
    const askTab = page.locator('button[type="button"]').filter({ hasText: /^Ask$/ });
    await expect(askTab).toBeVisible({ timeout: 10000 });
    await askTab.click();

    const hostInput = page.getByPlaceholder(/Ask the host a question/i);
    await expect(hostInput).toBeVisible({ timeout: 5000 });

    await hostInput.fill("Can you share the full model?");
    await page.getByRole("button", { name: "Ask", exact: true }).and(page.locator('[type="submit"]')).click();

    await expect(page.getByText("Can you share the full model?")).toBeVisible({ timeout: 10000 });
    await expect(page.getByText(/Awaiting reply/i)).toBeVisible({ timeout: 10000 });
  });

  test("Ask Host rate limit shows distinct error", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);

    await page.goto(`/l/${SMOKE_TOKEN}`);
    await expect(page.locator("img[alt*='Page']").first()).toBeVisible({ timeout: 15000 });
    const openSidebar = page.getByRole("button", { name: /Open sidebar/i });
    if (await openSidebar.isVisible().catch(() => false)) {
      await openSidebar.click();
    }
    await page.locator('button[type="button"]').filter({ hasText: /^Ask$/ }).click();

    const hostInput = page.getByPlaceholder(/Ask the host a question/i);
    await hostInput.fill("__rate_limit__ spam");
    await page.getByRole("button", { name: "Ask", exact: true }).and(page.locator('[type="submit"]')).click();

    await expect(page.getByText(/Too many Ask Host questions/i)).toBeVisible({ timeout: 10000 });
  });
});
