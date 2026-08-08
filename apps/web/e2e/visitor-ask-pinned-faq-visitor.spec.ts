/**
 * Visitor-side pinned FAQ readout (MSW).
 */
import { test, expect } from "@playwright/test";
import { resetMockState, attachDebug, openVisitorAskPanel } from "./helpers";

const SMOKE_TOKEN = "AskSmoke1";
const PINNED_QUESTION = "What is the company burn rate?";
const PINNED_ANSWER = /Monthly burn is approximately \$420K/i;

test.describe("Visitor pinned FAQ (MSW)", () => {
  test("shows pinned FAQs above Ask input and supports Ask this", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);

    const faqPromise = page.waitForResponse(
      (res) =>
        res.request().method() === "GET" &&
        res.url().includes(`/public/links/${SMOKE_TOKEN}/ask/faq`) &&
        res.ok(),
    );
    const input = await openVisitorAskPanel(page, SMOKE_TOKEN);
    await faqPromise;

    const faqSection = page.getByRole("region", { name: /Common questions/i });
    await expect(faqSection).toBeVisible({ timeout: 10000 });
    await expect(faqSection).toContainText(PINNED_QUESTION);

    await faqSection.getByRole("button", { name: PINNED_QUESTION }).click();
    await expect(faqSection).toContainText(PINNED_ANSWER);

    await faqSection.getByRole("button", { name: /Ask this/i }).click();
    await expect(input).toHaveValue(PINNED_QUESTION);
  });
});
