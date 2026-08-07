/**
 * Visitor Ask AI lane on live API + docling-rag (optional).
 *
 * Skips when knowledge is disabled (no DOCLING_RAG_* on API).
 *
 * Run:
 *   REAL_API_BASE_URL=http://localhost:8080 ./e2e-visitor-ask-real.sh --ai
 */
import { test, expect } from "@playwright/test";
import {
  seedRealBackend,
  seedDocument,
  seedDealRoom,
  seedDealRoomLink,
  waitForKnowledgeCorpusReady,
  snapshotCookieJar,
  restoreCookieJar,
  accessPublicLinkApi,
  submitPublicAsk,
  streamPublicAskTurn,
  updateLinkAskPolicy,
  probeKnowledgeEnabled,
  listMyPublicAskTurns,
  parseVisitorAskSSE,
} from "./real-helpers";

let workspaceSlug: string;
let knowledgeEnabled = false;
let publicToken = "";
let linkId = "";

test.describe("Visitor Ask AI stream (real backend)", () => {
  test.beforeAll(async () => {
    test.setTimeout(300_000);
    const seed = await seedRealBackend();
    workspaceSlug = seed.workspaceSlug;
    const doc = await seedDocument(workspaceSlug);
    const room = await seedDealRoom(workspaceSlug, {
      name: "Visitor Ask AI Room",
      templateType: "seed",
      documentIds: [doc.id],
    });

    knowledgeEnabled = await probeKnowledgeEnabled(workspaceSlug, room.id);
    if (!knowledgeEnabled) return;

    await waitForKnowledgeCorpusReady(workspaceSlug, room.id, 180);
    knowledgeEnabled = true;

    const authCookies = snapshotCookieJar();
    const link = await seedDealRoomLink(workspaceSlug, room.id, {
      name: `Ask AI Link ${Date.now()}`,
    });
    linkId = link.id;
    publicToken = link.publicToken;
    await updateLinkAskPolicy(workspaceSlug, linkId, { askAiEnabled: true });
    restoreCookieJar(authCookies);
  });

  test("submit routes to AI lane and stream returns grounded answer", async () => {
    test.skip(!knowledgeEnabled, "docling-rag not configured on API");
    test.setTimeout(180_000);

    const authCookies = snapshotCookieJar();
    restoreCookieJar([]);
    await accessPublicLinkApi(publicToken, `ai-visitor-${Date.now()}@example.com`);

    const question = "What is the valuation cap?";
    const created = await submitPublicAsk(publicToken, question);
    expect(created.lane).toBe("ai");
    expect(["ai_streaming", "routing"]).toContain(created.status);

    const streamRes = await streamPublicAskTurn(publicToken, created.id);
    expect(streamRes.status).toBe(200);
    expect(streamRes.headers.get("content-type") ?? "").toContain("text/event-stream");
    const sseBody = await streamRes.text();
    const parsed = parseVisitorAskSSE(sseBody);
    expect(parsed.answer.length).toBeGreaterThan(0);

    const mine = await listMyPublicAskTurns(publicToken);
    const turn = mine.find((t) => t.id === created.id);
    expect(turn?.status).toBe("ai_answered");
    restoreCookieJar(authCookies);
  });
});
