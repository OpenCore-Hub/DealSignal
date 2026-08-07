/**
 * Engage tab no longer hosts Ask policy controls (moved to share link Access settings).
 */
import { test, expect } from "@playwright/test";
import { setupAuthenticatedPage, attachDebug, WORKSPACE_SLUG } from "./helpers";

const SMOKE_LINK_ID = "link_visitor_ask_smoke";

test.describe("Visitor Ask Engage tab (MSW)", () => {
  test("activity Engage tab shows inbox without grounded AI policy section", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=links`);
    await page.getByTestId(`deal-room-link-row-${SMOKE_LINK_ID}`).click();
    await expect(page.getByRole("dialog")).toBeVisible({ timeout: 10000 });
    await page.getByRole("tab", { name: /Engage/i }).click();

    await expect(page.getByTestId("visitor-ask-experience")).toHaveCount(0);
    await expect(page.getByText("Ask inbox", { exact: true })).toBeVisible({ timeout: 10000 });
  });
});
