/**
 * Visitor-side published Formal Q&A board (MSW).
 */
import { test, expect } from "@playwright/test";
import {
  resetMockState,
  attachDebug,
  openVisitorAskPanel,
  PUBLISHED_QA_REGION,
} from "./helpers";

const SMOKE_TOKEN = "AskSmoke1";
const FORMAL_QUESTION = "What disclosures are board-approved?";
const FORMAL_ANSWER = /All revenue guidance is board-approved quarterly/i;

test.describe("Visitor published Formal Q&A (MSW)", () => {
  test("shows Published Q&A section with board-approved disclosures", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);

    const formalPromise = page.waitForResponse(
      (res) =>
        res.request().method() === "GET" &&
        res.url().includes(`/public/links/${SMOKE_TOKEN}/ask/formal`) &&
        res.ok(),
    );
    await openVisitorAskPanel(page, SMOKE_TOKEN);
    await formalPromise;

    const formalSection = page.getByRole("region", { name: PUBLISHED_QA_REGION });
    await expect(formalSection).toBeVisible({ timeout: 10000 });
    await expect(formalSection).toContainText(FORMAL_QUESTION);

    await formalSection.getByRole("button", { name: FORMAL_QUESTION }).click();
    await expect(formalSection).toContainText(FORMAL_ANSWER);
    await expect(formalSection.getByText(/Asked by|提问者/i)).toHaveCount(0);

    await formalSection.getByRole("button", { name: /Ask this/i }).click();
    await expect(page.getByPlaceholder(/materials you can access|Ask the host/i)).toHaveValue(
      FORMAL_QUESTION,
    );
  });
});
