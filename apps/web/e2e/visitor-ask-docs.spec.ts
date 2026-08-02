/**
 * Visitor Ask Docs (MSW) — public Ask Docs chat success, no_evidence CTA, rate limit.
 * Complements visitor-ask-smoke.spec.ts (Ask Host path).
 */
import { test, expect } from "@playwright/test";
import { resetMockState, attachDebug } from "./helpers";

const SMOKE_TOKEN = "AskSmoke1";

async function openAskSidebar(page: import("@playwright/test").Page) {
  await page.goto(`/l/${SMOKE_TOKEN}`);
  await expect(page.locator("img[alt*='Page']").first()).toBeVisible({ timeout: 15000 });
  // Deal-room links auto-open the sidebar; standalone links need the toolbar toggle.
  const openSidebar = page.getByRole("button", { name: /Open sidebar/i });
  if (await openSidebar.isVisible().catch(() => false)) {
    await openSidebar.click();
  }
  const askTab = page.locator('button[type="button"]').filter({ hasText: /^Ask$/ });
  await expect(askTab).toBeVisible({ timeout: 10000 });
  await askTab.click();
  await expect(
    page.getByText(/Ask Docs first; switch to Ask Host if you need missing materials/i),
  ).toBeVisible({ timeout: 10000 });
}

test.describe("Visitor Ask Docs (MSW)", () => {
  test("Ask Docs success returns grounded answer with evidence", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);
    await openAskSidebar(page);

    const input = page.getByPlaceholder(/Ask about materials authorized for this link/i);
    await expect(input).toBeVisible({ timeout: 5000 });
    await input.fill("What is the growth rate?");
    await page.getByRole("button", { name: "Ask", exact: true }).and(page.locator('[type="submit"]')).click();

    await expect(page.getByText(/Based on authorized materials/i)).toBeVisible({ timeout: 10000 });
    await expect(page.getByText(/Revenue grew 3x year over year/i)).toBeVisible({ timeout: 5000 });
  });

  test("Ask Docs no_evidence shows refusal and Ask Host CTA", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);
    await openAskSidebar(page);

    const input = page.getByPlaceholder(/Ask about materials authorized for this link/i);
    await input.fill("__no_evidence__ Is there an SOC2 report?");
    await page.getByRole("button", { name: "Ask", exact: true }).and(page.locator('[type="submit"]')).click();

    await expect(
      page.getByText(/couldn't find supporting material in the documents you can access/i),
    ).toBeVisible({ timeout: 10000 });
    await expect(page.getByRole("button", { name: /Ask the host instead/i })).toBeVisible({
      timeout: 5000,
    });
  });

  test("Ask Docs rate limit surfaces error in chat", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);
    await openAskSidebar(page);

    const input = page.getByPlaceholder(/Ask about materials authorized for this link/i);
    await input.fill("__rate_limit__ spam");
    await page.getByRole("button", { name: "Ask", exact: true }).and(page.locator('[type="submit"]')).click();

    // Distinct i18n for visitor rate abuse (not generic search failed).
    await expect(page.getByText(/Too many Ask Docs requests/i).first()).toBeVisible({
      timeout: 10000,
    });
  });

  test("Ask Docs limiter unavailable surfaces error without rate-limit abuse UX", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);
    await openAskSidebar(page);

    const input = page.getByPlaceholder(/Ask about materials authorized for this link/i);
    await input.fill("__limiter_down__ ping");
    await page.getByRole("button", { name: "Ask", exact: true }).and(page.locator('[type="submit"]')).click();

    await expect(page.getByText(/Ask Docs is temporarily unavailable/i).first()).toBeVisible({
      timeout: 10000,
    });
    await expect(page.getByText(/Too many Ask Docs requests/i)).toHaveCount(0);
  });
});
