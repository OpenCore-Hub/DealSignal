/**
 * Phase B — Owner Engage tab grounded AI policy toggle (MSW).
 */
import { test, expect } from "@playwright/test";
import { setupAuthenticatedPage, attachDebug, WORKSPACE_SLUG } from "./helpers";

const SMOKE_LINK_ID = "link_visitor_ask_smoke";

test.describe("Visitor Ask Engage policy (MSW)", () => {
  test("owner can enable grounded AI from Engage tab", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=links`);
    await page.getByTestId(`deal-room-link-row-${SMOKE_LINK_ID}`).click();
    await expect(page.getByRole("dialog")).toBeVisible({ timeout: 10000 });
    await page.getByRole("tab", { name: /Engage/i }).click();

    const policyCard = page.getByTestId("link-ask-ai-enabled");
    await expect(policyCard).toBeVisible({ timeout: 10000 });

    const toggle = policyCard.getByRole("switch");
    await expect(toggle).toBeChecked();

    const patchPromise = page.waitForResponse(
      (res) =>
        res.request().method() === "PATCH" &&
        res.url().includes(`/links/${SMOKE_LINK_ID}/ask-policy`) &&
        res.ok(),
    );
    await toggle.click();
    await patchPromise;
    await expect(toggle).not.toBeChecked();

    const enablePromise = page.waitForResponse(
      (res) =>
        res.request().method() === "PATCH" &&
        res.url().includes(`/links/${SMOKE_LINK_ID}/ask-policy`) &&
        res.ok(),
    );
    await toggle.click();
    await enablePromise;
    await expect(toggle).toBeChecked();
  });
});
