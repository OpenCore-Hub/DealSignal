/**
 * Deal-room knowledge base panel (MSW) — create empty KB and rebuild.
 */
import { test, expect } from "@playwright/test";
import { setupAuthenticatedPage, attachDebug, WORKSPACE_SLUG } from "./helpers";

test.describe("Knowledge base panel (MSW)", () => {
  test("creates an empty knowledge base from the documents tab", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=documents`);
    const panel = page.getByTestId("knowledge-base-panel");
    await expect(panel).toBeVisible({ timeout: 15000 });
    await expect(panel.getByText(/Not created/i)).toBeVisible({ timeout: 10000 });

    await panel.getByRole("button", { name: /Create knowledge base/i }).click();
    // Inline wizard — confirm create with empty selection.
    await panel.getByRole("button", { name: /^Create$/i }).click();

    await expect(panel.getByText(/Ready/i)).toBeVisible({ timeout: 10000 });
  });

  test("rebuilds knowledge base and keeps panel ready", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=documents`);
    const panel = page.getByTestId("knowledge-base-panel");
    await expect(panel).toBeVisible({ timeout: 15000 });

    // Ensure a ready KB exists (seed through the live page so MSW intercepts).
    if (await panel.getByText(/Not created/i).isVisible().catch(() => false)) {
      await panel.getByRole("button", { name: /Create knowledge base/i }).click();
      await panel.getByRole("button", { name: /^Create$/i }).click();
      await expect(panel.getByText(/Ready/i)).toBeVisible({ timeout: 10000 });
    } else {
      await expect(panel.getByText(/Ready/i)).toBeVisible({ timeout: 10000 });
    }

    await panel.getByRole("button", { name: /Rebuild knowledge base/i }).click();
    await panel.getByRole("button", { name: /^Rebuild$/i }).click();
    await expect(page.getByRole("dialog")).toBeVisible({ timeout: 5000 });
    await page.getByRole("dialog").getByRole("button", { name: /^Rebuild$/i }).click();

    await expect(panel.getByText(/Ready/i)).toBeVisible({ timeout: 10000 });
  });
});
