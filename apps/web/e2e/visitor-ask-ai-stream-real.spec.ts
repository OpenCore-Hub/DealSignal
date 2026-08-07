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

  test("re-ask same question after enabling AI routes new turn to AI lane", async () => {
    test.skip(!knowledgeEnabled, "docling-rag not configured on API");
    test.setTimeout(180_000);

    const authCookies = snapshotCookieJar();
    await updateLinkAskPolicy(workspaceSlug, linkId, { askAiEnabled: false });

    const visitorEmail = `repeat-ask-${Date.now()}@example.com`;
    restoreCookieJar([]);
    await accessPublicLinkApi(publicToken, visitorEmail);
    const visitorCookies = snapshotCookieJar();

    const qProfit = "What is the valuation cap in the materials?";
    const qOther = "Summarize the key financial metrics.";

    const first = await submitPublicAsk(publicToken, qProfit);
    expect(first.lane).toBe("host");
    expect(first.status).toBe("host_pending");

    restoreCookieJar(authCookies);
    await updateLinkAskPolicy(workspaceSlug, linkId, { askAiEnabled: true });

    restoreCookieJar(visitorCookies);
    const second = await submitPublicAsk(publicToken, qOther);
    expect(second.lane).toBe("ai");

    const streamRes = await streamPublicAskTurn(publicToken, second.id);
    expect(streamRes.status).toBe(200);

    const third = await submitPublicAsk(publicToken, qProfit);
    expect(third.lane).toBe("ai");
    expect(third.id).not.toBe(first.id);

    const mine = await listMyPublicAskTurns(publicToken);
    const profitTurns = mine.filter((t) => t.question === qProfit);
    expect(profitTurns).toHaveLength(2);
    expect(profitTurns.some((t) => t.lane === "host" && t.id === first.id)).toBe(true);
    expect(profitTurns.some((t) => t.lane === "ai" && t.id === third.id)).toBe(true);

    restoreCookieJar(authCookies);
  });
});
