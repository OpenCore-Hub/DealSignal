/**
 * Owner Engage grounded AI toggle against live API + Vite UI.
 *
 * Run:
 *   REAL_API_BASE_URL=http://localhost:8090 ./e2e-visitor-ask-real.sh --ui
 */
import { test, expect } from "@playwright/test";
import {
  seedRealBackend,
  seedDocument,
  seedDealRoom,
  seedDealRoomLink,
  enableGroundedAiInEngageTab,
  fetchLinkById,
  attachDebug,
} from "./real-helpers";

let workspaceSlug: string;
let roomId: string;
let linkId: string;

test.describe("Visitor Ask Engage policy (real backend UI)", () => {
  test.beforeAll(async () => {
    test.setTimeout(180_000);
    const seed = await seedRealBackend();
    workspaceSlug = seed.workspaceSlug;
    const doc = await seedDocument(workspaceSlug);
    const room = await seedDealRoom(workspaceSlug, {
      name: "Visitor Ask Policy UI Room",
      templateType: "seed",
      documentIds: [doc.id],
    });
    roomId = room.id;
    const link = await seedDealRoomLink(workspaceSlug, room.id, {
      name: `Ask Policy UI Link ${Date.now()}`,
    });
    linkId = link.id;

    const detail = await fetchLinkById(workspaceSlug, linkId);
    expect(detail.askAiEnabled).not.toBe(true);
  });

  test("owner enables grounded AI from Engage tab", async ({ page }) => {
    test.setTimeout(120_000);
    attachDebug(page);

    await enableGroundedAiInEngageTab(page, { workspaceSlug, roomId, linkId });

    const detail = await fetchLinkById(workspaceSlug, linkId);
    expect(detail.askAiEnabled).toBe(true);
  });
});
