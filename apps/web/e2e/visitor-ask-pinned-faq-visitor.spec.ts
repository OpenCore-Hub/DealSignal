/**
 * Visitor-side pinned FAQ Help Center + Ask intercept (MSW).
 */
import { test, expect } from "@playwright/test";
import { resetMockState, attachDebug, openVisitorAskPanel } from "./helpers";

const SMOKE_TOKEN = "AskSmoke1";
const PINNED_QUESTION = "What is the company burn rate?";
const PINNED_ANSWER = /Monthly burn is approximately \$420K/i;

test.describe("Visitor pinned FAQ (MSW)", () => {
  test("shows FAQ tab search and Ask this replays without typing", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);

    const faqPromise = page.waitForResponse(
      (res) =>
        res.request().method() === "GET" &&
        res.url().includes(`/public/links/${SMOKE_TOKEN}/ask/faq`) &&
        res.ok(),
    );
    await openVisitorAskPanel(page, SMOKE_TOKEN);
    await faqPromise;

    const faqTab = page.getByRole("button", { name: /^FAQ$/ });
    await expect(faqTab).toBeVisible({ timeout: 10000 });
    await faqTab.click();

    const search = page.getByPlaceholder(/Search common questions/i);
    await expect(search).toBeVisible();
    await expect(page.getByText(PINNED_QUESTION)).toBeVisible();

    await page.getByRole("button", { name: PINNED_QUESTION }).click();
    await expect(page.getByText(PINNED_ANSWER)).toBeVisible();

    const askThisPost = page.waitForResponse(
      (res) =>
        res.request().method() === "POST" &&
        res.url().includes(`/public/links/${SMOKE_TOKEN}/ask`) &&
        res.status() === 201,
    );
    await page.getByRole("button", { name: /Ask this/i }).click();
    const askThisRes = await askThisPost;
    const askThisBody = (await askThisRes.json()) as {
      data: { route_reason?: string; status: string };
    };
    expect(askThisBody.data.route_reason).toBe("pinned_faq");
    expect(askThisBody.data.status).not.toBe("ai_streaming");
    await expect(page.getByText(PINNED_ANSWER)).toBeVisible();
  });

  test("asking a pinned question replays the FAQ without SSE", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);
    await openVisitorAskPanel(page, SMOKE_TOKEN);

    let streamed = false;
    page.on("request", (req) => {
      if (req.url().includes(`/public/links/${SMOKE_TOKEN}/ask/`) && req.url().includes("/stream")) {
        streamed = true;
      }
    });

    const input = page.getByPlaceholder(/materials you can access|Ask the host/i);
    await input.fill(PINNED_QUESTION);
    const postPromise = page.waitForResponse(
      (res) =>
        res.request().method() === "POST" &&
        res.url().includes(`/public/links/${SMOKE_TOKEN}/ask`) &&
        res.status() === 201,
    );
    await page
      .getByRole("button", { name: /Ask/i })
      .and(page.locator('[type="submit"]'))
      .click();
    const res = await postPromise;
    const body = (await res.json()) as {
      data: { route_reason?: string; status: string; lane: string };
    };
    expect(body.data.route_reason).toBe("pinned_faq");
    expect(body.data.status).not.toBe("ai_streaming");
    await expect(page.getByText(PINNED_ANSWER)).toBeVisible();
    expect(streamed).toBe(false);
  });
});
