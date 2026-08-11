/**
 * Dashboard operational-action deep links must stay surface-isolated:
 * document share → Document Library Share
 * deal-room share / membership → Deal Room Access
 * Never cross-contaminate inboxes.
 */
import { test, expect } from "@playwright/test";
import {
  setupAuthenticatedPage,
  attachDebug,
  ASK_INBOX_TITLE,
  FORMAL_QUEUE_TAB,
  WORKSPACE_SLUG,
} from "./helpers";

test.describe("Dashboard action surface navigation (MSW)", () => {
  async function expandAllStrands(page: import("@playwright/test").Page) {
    const strands = page.getByTestId("radar-strand");
    const count = await strands.count();
    for (let index = 0; index < count; index += 1) {
      const header = strands.nth(index).locator("button").first();
      if ((await header.getAttribute("aria-expanded")) === "false") {
        await header.click();
      }
    }
  }

  function radarRow(page: import("@playwright/test").Page, text: RegExp) {
    return page.getByTestId("radar-row").filter({ hasText: text });
  }

  test("document share todo opens Document Library Share, not a deal room", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/dashboard`);
    await expect(page.getByText(/今日关注|Today's focus|Attention/i).first()).toBeVisible({
      timeout: 15000,
    });
    await expandAllStrands(page);

    // Actions tab may be labeled 待办 / Actions depending on locale.
    const actionsTab = page.getByRole("tab").filter({ hasText: /待办|Actions|To-do/i }).first();
    if (await actionsTab.isVisible().catch(() => false)) {
      await actionsTab.click();
    }

    const docTodo = radarRow(
      page,
      /Approve access request from doc-share-applicant@example\.com/i,
    );
    await expect(docTodo).toBeVisible({ timeout: 15000 });
    await docTodo.getByRole("button", { name: "Approve", exact: true }).click();

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
    await expandAllStrands(page);
    const actionsTab = page.getByRole("tab").filter({ hasText: /待办|Actions|To-do/i }).first();
    if (await actionsTab.isVisible().catch(() => false)) {
      await actionsTab.click();
    }

    const roomShareTodo = radarRow(
      page,
      /Approve deal room share access from room-share-applicant@example\.com/i,
    );
    await expect(roomShareTodo).toBeVisible({ timeout: 15000 });
    await roomShareTodo.getByRole("button", { name: "Approve", exact: true }).click();

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
    await expandAllStrands(page);
    const actionsTab = page.getByRole("tab").filter({ hasText: /待办|Actions|To-do/i }).first();
    if (await actionsTab.isVisible().catch(() => false)) {
      await actionsTab.click();
    }

    const memberTodo = radarRow(
      page,
      /Approve room access request from marcus@boldstart\.vc/i,
    );
    await expect(memberTodo).toBeVisible({ timeout: 15000 });
    await memberTodo.getByRole("button", { name: "Approve", exact: true }).click();

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
    await expandAllStrands(page);
    const actionsTab = page.getByRole("tab").filter({ hasText: /待办|Actions|To-do/i }).first();
    if (await actionsTab.isVisible().catch(() => false)) {
      await actionsTab.click();
    }

    const askTodo = radarRow(
      page,
      /Answer visitor Ask from investor@example\.com/i,
    );
    await expect(askTodo).toBeVisible({ timeout: 15000 });
    await askTodo.getByRole("button", { name: "Reply", exact: true }).click();

    await expect(page).toHaveURL(
      new RegExp(`/${WORKSPACE_SLUG}/deal-rooms/room_1\\?tab=qa&linkId=link_room_1`),
    );
    await expect(page.getByText(ASK_INBOX_TITLE)).toBeVisible({
      timeout: 15000,
    });
    await expect(page).not.toHaveURL(/\/documents/);
    await expect(page).not.toHaveURL(/\/links\//);
  });

  test("formal Ask review todo opens QA formal queue with link filter", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/dashboard`);
    await expect(page.getByText(/今日关注|Today's focus|Attention/i).first()).toBeVisible({
      timeout: 15000,
    });
    await expandAllStrands(page);
    const actionsTab = page.getByRole("tab").filter({ hasText: /待办|Actions|To-do/i }).first();
    if (await actionsTab.isVisible().catch(() => false)) {
      await actionsTab.click();
    }

    const formalTodo = radarRow(
      page,
      /Review formal Q&A from compliance@example\.com on Acme Seed Data Room/i,
    );
    await expect(formalTodo).toBeVisible({ timeout: 15000 });
    await formalTodo.getByRole("button", { name: "Reply", exact: true }).click();

    await expect(page).toHaveURL(
      new RegExp(
        `/${WORKSPACE_SLUG}/deal-rooms/room_1\\?tab=qa&linkId=link_room_1&askInbox=formal_queue`,
      ),
    );
    await expect(page.getByRole("tab", { name: FORMAL_QUEUE_TAB })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    await expect(
      page.getByText("What is the board-approved revenue guidance?"),
    ).toBeVisible({ timeout: 15000 });
  });
});
