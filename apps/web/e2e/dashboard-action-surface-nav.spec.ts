/**
 * Dashboard operational-action deep links must stay surface-isolated:
 * document share → Document Library Share
 * deal-room share / membership → Deal Room Access
 * Never cross-contaminate inboxes.
 */
import { test, expect } from "@playwright/test";
import { setupAuthenticatedPage, attachDebug, WORKSPACE_SLUG } from "./helpers";

test.describe("Dashboard action surface navigation (MSW)", () => {
  test("document share todo opens Document Library Share, not a deal room", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/dashboard`);
    await expect(page.getByText(/今日关注|Today's focus|Attention/i).first()).toBeVisible({
      timeout: 15000,
    });

    // Actions tab may be labeled 待办 / Actions depending on locale.
    const actionsTab = page.getByRole("tab").filter({ hasText: /待办|Actions|To-do/i }).first();
    if (await actionsTab.isVisible().catch(() => false)) {
      await actionsTab.click();
    }

    const docTodo = page.getByText(
      /Approve access request from doc-share-applicant@example\.com/i,
    );
    await expect(docTodo).toBeVisible({ timeout: 15000 });
    await docTodo.click();

    await expect(page).toHaveURL(/\/documents\?tab=shared&linkId=link_1/);
    await expect(page).not.toHaveURL(/\/deal-rooms\//);
    await expect(page.getByRole("tab", { name: /Shared|分享|已分享/i })).toBeVisible({
      timeout: 15000,
    });
  });

  test("deal-room share todo opens Deal Room Access with link focus", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/dashboard`);
    await expect(page.getByText(/今日关注|Today's focus|Attention/i).first()).toBeVisible({
      timeout: 15000,
    });
    const actionsTab = page.getByRole("tab").filter({ hasText: /待办|Actions|To-do/i }).first();
    if (await actionsTab.isVisible().catch(() => false)) {
      await actionsTab.click();
    }

    const roomShareTodo = page.getByText(
      /Approve deal room share access from room-share-applicant@example\.com/i,
    );
    await expect(roomShareTodo).toBeVisible({ timeout: 15000 });
    await roomShareTodo.click();

    await expect(page).toHaveURL(
      new RegExp(`/${WORKSPACE_SLUG}/deal-rooms/room_1\\?tab=access&linkId=link_room_1`),
    );
    await expect(page.getByTestId("deal-room-access-control-tab")).toBeVisible({
      timeout: 15000,
    });
    await expect(page.getByTestId("deal-room-access-requests-panel")).toBeVisible();
    await expect(page.getByText("room-share-applicant@example.com")).toBeVisible();
    await expect(
      page.getByTestId("deal-room-access-request-lar_room_1"),
    ).toHaveAttribute("data-focused", "true");
  });

  test("room membership todo opens Deal Room Access by room id", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/dashboard`);
    await expect(page.getByText(/今日关注|Today's focus|Attention/i).first()).toBeVisible({
      timeout: 15000,
    });
    const actionsTab = page.getByRole("tab").filter({ hasText: /待办|Actions|To-do/i }).first();
    if (await actionsTab.isVisible().catch(() => false)) {
      await actionsTab.click();
    }

    const memberTodo = page.getByText(
      /Approve room access request from marcus@boldstart\.vc/i,
    );
    await expect(memberTodo).toBeVisible({ timeout: 15000 });
    await memberTodo.click();

    await expect(page).toHaveURL(
      new RegExp(`/${WORKSPACE_SLUG}/deal-rooms/room_1\\?tab=access`),
    );
    await expect(page.getByTestId("deal-room-access-control-tab")).toBeVisible({
      timeout: 15000,
    });
    await expect(page.getByText(/room not found/i)).toHaveCount(0);
  });

  test("deal-room visitor Ask todo opens Deal Room QA inbox with link filter", async ({
    page,
  }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/dashboard`);
    await expect(page.getByText(/今日关注|Today's focus|Attention/i).first()).toBeVisible({
      timeout: 15000,
    });
    const actionsTab = page.getByRole("tab").filter({ hasText: /待办|Actions|To-do/i }).first();
    if (await actionsTab.isVisible().catch(() => false)) {
      await actionsTab.click();
    }

    const askTodo = page.getByText(/Answer visitor Ask from investor@example\.com/i);
    await expect(askTodo).toBeVisible({ timeout: 15000 });
    await askTodo.click();

    await expect(page).toHaveURL(
      new RegExp(`/${WORKSPACE_SLUG}/deal-rooms/room_1\\?tab=qa&linkId=link_room_1`),
    );
    await expect(page.getByText("Ask inbox", { exact: true })).toBeVisible({
      timeout: 15000,
    });
    await expect(page).not.toHaveURL(/\/documents/);
    await expect(page).not.toHaveURL(/\/links\//);
  });
});
