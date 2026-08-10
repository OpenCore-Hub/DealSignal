/**
 * Owner enables grounded AI via ask-policy API against live backend.
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
  enableGroundedAiForLink,
  fetchLinkById,
  updateLinkAskPolicy,
  attachDebug,
} from "./real-helpers";

let workspaceSlug: string;
let _roomId: string;
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
    _roomId = room.id;
    const link = await seedDealRoomLink(workspaceSlug, room.id, {
      name: `Ask Policy UI Link ${Date.now()}`,
    });
    linkId = link.id;

    // Deal-room links default ask_ai_enabled=true; disable first so the enable path is exercised.
    await updateLinkAskPolicy(workspaceSlug, linkId, { askAiEnabled: false });
    const detail = await fetchLinkById(workspaceSlug, linkId);
    expect(detail.askAiEnabled).toBe(false);
  });

  test("owner enables grounded AI via ask-policy API", async ({ page }) => {
    test.setTimeout(120_000);
    attachDebug(page);

    await enableGroundedAiForLink(workspaceSlug, linkId);

    const detail = await fetchLinkById(workspaceSlug, linkId);
    expect(detail.askAiEnabled).toBe(true);
  });
});
