/**
 * Owner-visible Visitor Ask high-risk security events (US#32 / B3) — MSW e2e.
 * Covers room analytics panel and link activity → Engage tab panel.
 */
import { test, expect } from "@playwright/test";
import { setupAuthenticatedPage, attachDebug, WORKSPACE_SLUG } from "./helpers";

test.describe("Ask security events panel (MSW)", () => {
  test("room analytics shows high-risk Ask security events", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=analytics`);
    const panel = page.getByTestId("ask-security-events-panel");
    await expect(panel).toBeVisible({ timeout: 15000 });
    await expect(panel.getByText(/Visitor Ask security events/i)).toBeVisible();
    await expect(panel.getByText(/Rate limit exceeded/i).first()).toBeVisible({
      timeout: 10000,
    });
    await expect(panel.getByText(/Scope violation/i).first()).toBeVisible();
    await expect(panel.getByText(/Removed from allowlist/i).first()).toBeVisible();
    await expect(panel.getByText(/High risk/i).first()).toBeVisible();
  });

  test("room analytics filters security events by link", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=analytics`);
    const panel = page.getByTestId("ask-security-events-panel");
    await expect(panel).toBeVisible({ timeout: 15000 });
    await expect(panel.getByText(/Rate limit exceeded/i).first()).toBeVisible({
      timeout: 10000,
    });

    const filter = panel.getByLabel(/Filter by link/i);
    await expect(filter).toBeVisible();
    // Prefer a concrete room link id from options (link_1 is seeded for room_1).
    await filter.selectOption("link_1");
    await expect(panel.getByTestId("ask-security-event-row").first()).toBeVisible({
      timeout: 10000,
    });
    await expect(panel.getByText(/Rate limit exceeded/i).first()).toBeVisible();
  });

  test("link activity Engage tab shows Ask security events", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=participants`);
    const row = page.getByTestId("deal-room-link-row-link_1");
    await expect(row).toBeVisible({ timeout: 15000 });
    // Row click opens LinkActivityDialog (AnalyticsTab).
    await row.click();

    await expect(page.getByRole("dialog")).toBeVisible({ timeout: 10000 });
    await page.getByRole("tab", { name: /Engage/i }).click();

    const panel = page.getByTestId("ask-security-events-panel");
    await expect(panel).toBeVisible({ timeout: 10000 });
    await expect(panel.getByText(/Rate limit exceeded/i)).toBeVisible({ timeout: 10000 });
    await expect(panel.getByText(/Removed from allowlist/i)).toBeVisible();
    await expect(panel.getByText(/visitor@example.com/i)).toBeVisible();
    await expect(panel.getByText(/Detail:\s*Ask Docs/i)).toBeVisible();
  });
});
