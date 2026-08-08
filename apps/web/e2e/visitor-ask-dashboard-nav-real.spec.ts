/**
 * Dashboard deal_room_link_question → Deal Room QA inbox (real backend).
 *
 * Run via:
 *   REAL_API_BASE_URL=http://localhost:8080 ./e2e-visitor-ask-real.sh --ui
 */
import { test, expect } from "@playwright/test";
import {
  seedRealBackend,
  seedDocument,
  seedDealRoom,
  seedDealRoomLink,
  authenticatePage,
  attachDebug,
  snapshotCookieJar,
  restoreCookieJar,
  accessPublicLinkApi,
  submitPublicAsk,
} from "./real-helpers";
import { ASK_INBOX_TITLE } from "./helpers";

let workspaceSlug: string;
let roomId: string;
let linkId: string;

test.describe("Visitor Ask dashboard navigation (real backend UI)", () => {
  test.beforeAll(async () => {
    test.setTimeout(180_000);
    const seed = await seedRealBackend();
    workspaceSlug = seed.workspaceSlug;
    const doc = await seedDocument(workspaceSlug);
    const room = await seedDealRoom(workspaceSlug, {
      name: "Dashboard Ask Nav Room",
      templateType: "seed",
      documentIds: [doc.id],
    });
    roomId = room.id;
    const link = await seedDealRoomLink(workspaceSlug, room.id, {
      name: `Dashboard Ask Link ${Date.now()}`,
    });
    linkId = link.id;

    const visitorEmail = `dashboard-ask-${Date.now()}@example.com`;
    const authCookies = snapshotCookieJar();
    restoreCookieJar([]);
    await accessPublicLinkApi(link.publicToken, visitorEmail);
    await submitPublicAsk(link.publicToken, "Dashboard nav contract question?");
    restoreCookieJar(authCookies);
  });

  test("deal-room visitor Ask todo opens QA inbox with link filter", async ({ page }) => {
    test.setTimeout(90_000);
    attachDebug(page);

    await authenticatePage(page);
    await page.goto(`/${workspaceSlug}/dashboard`);
    await expect(page.getByText(/今日关注|Today's focus|Attention/i).first()).toBeVisible({
      timeout: 20000,
    });

    const actionsTab = page.getByRole("tab").filter({ hasText: /待办|Actions|To-do/i }).first();
    if (await actionsTab.isVisible().catch(() => false)) {
      await actionsTab.click();
    }

    const askTodo = page.getByText(/Answer visitor Ask from dashboard-ask-/i);
    await expect(askTodo).toBeVisible({ timeout: 20000 });
    await askTodo.click();

    await expect(page).toHaveURL(
      new RegExp(`/${workspaceSlug}/deal-rooms/${roomId}\\?tab=qa&linkId=${linkId}`),
    );
    await expect(page.getByText(ASK_INBOX_TITLE)).toBeVisible({
      timeout: 15000,
    });
    await expect(page).not.toHaveURL(/\/documents/);
    await expect(page).not.toHaveURL(/\/links\//);
  });
});
