/**
 * Q&A strategy on share link Access settings (MSW).
 */
import { test, expect } from "@playwright/test";
import { setupAuthenticatedPage, attachDebug, WORKSPACE_SLUG } from "./helpers";

const EDIT_LINK_ID = "link_room_1";

test.describe("Visitor Ask Access policy (MSW)", () => {
  test("edit link shows Q&A strategy and AI quota readout in Advanced", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=links`);
    const row = page.getByTestId(`deal-room-link-row-${EDIT_LINK_ID}`);
    await expect(row).toBeVisible({ timeout: 15000 });
    await row.getByRole("button", { name: "Edit" }).click();

    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible({ timeout: 10000 });
    await dialog.getByRole("button", { name: /^Advanced$/i }).click();

    await expect(dialog.getByTestId("visitor-ask-experience")).toBeVisible({ timeout: 5000 });
    await expect(dialog.getByText(/Q&A strategy/i)).toBeVisible();
    await expect(dialog.getByTestId("link-ask-policy-quota")).toBeVisible({ timeout: 10000 });
  });

  test("selecting Host replies persists ask_ai_enabled false on save and reload", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=links`);
    await page.getByTestId("deal-room-create-new-link").click();

    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible({ timeout: 10000 });
    const linkName = `Host only ${Date.now()}`;
    await dialog.getByLabel(/Link name/i).fill(linkName);

    await dialog.getByRole("button", { name: /^Advanced$/i }).click();
    await dialog.getByTestId("visitor-ask-experience-host_only").click();

    await dialog.getByRole("button", { name: /Create link/i }).click();
    await expect(dialog).toBeHidden({ timeout: 15000 });

    const createdRow = page.locator('[data-testid^="deal-room-link-row-"]').filter({
      hasText: linkName,
    });
    await expect(createdRow).toBeVisible({ timeout: 10000 });
    await createdRow.getByRole("button", { name: "Edit" }).click();

    const editDialog = page.getByRole("dialog");
    await expect(editDialog).toBeVisible({ timeout: 10000 });
    await editDialog.getByRole("button", { name: /^Advanced$/i }).click();

    const hostOnly = editDialog.getByTestId("visitor-ask-experience-host_only");
    await expect(hostOnly).toHaveAttribute("aria-checked", "true");
    await expect(editDialog.getByTestId("link-ask-policy-quota")).toHaveCount(0);
  });
});
