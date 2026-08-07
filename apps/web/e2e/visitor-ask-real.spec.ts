/**
 * Visitor Ask — API contract on real backend.
 *
 * Covers deal-room host lane (public ask → owner inbox → host answer → visitor me)
 * and document-link qa_disabled boundary.
 *
 * Run:
 *   REAL_API_BASE_URL=http://localhost:8090 pnpm test:e2e:visitor-ask-real
 */
import { test, expect } from "@playwright/test";
import {
  seedRealBackend,
  seedDocument,
  seedDealRoom,
  seedLink,
  seedDealRoomLink,
  snapshotCookieJar,
  restoreCookieJar,
  accessPublicLinkApi,
  submitPublicAsk,
  updateLinkAskPolicy,
  streamPublicAskTurn,
  fetchLinkById,
  fetchLinkAnalytics,
  listMyPublicAskTurns,
  listOwnerLinkAsk,
  listOwnerRoomAsk,
  answerOwnerAskTurn,
  fetchDashboardActionItems,
  apiFetch,
} from "./real-helpers";

let workspaceSlug: string;

test.describe("Visitor Ask (real backend API)", () => {
  test.beforeAll(async () => {
    const seed = await seedRealBackend();
    workspaceSlug = seed.workspaceSlug;
  });

  test("deal-room link: public ask → owner inbox → host answer → visitor sees reply", async () => {
    const authCookies = snapshotCookieJar();
    const doc = await seedDocument(workspaceSlug);
    const room = await seedDealRoom(workspaceSlug, {
      name: "Visitor Ask API Room",
      templateType: "seed",
      documentIds: [doc.id],
    });
    const link = await seedDealRoomLink(workspaceSlug, room.id, {
      name: `Ask API Link ${Date.now()}`,
    });

    const visitorEmail = `ask-visitor-${Date.now()}@example.com`;
    const question = "Real API: when is the next investor update?";
    const answer = "We will share an update next Monday.";

    restoreCookieJar([]);
    await accessPublicLinkApi(link.publicToken, visitorEmail);
    const created = await submitPublicAsk(link.publicToken, question);
    expect(created.lane).toBe("host");
    expect(created.status).toBe("host_pending");
    expect(created.question).toBe(question);
    const visitorCookies = snapshotCookieJar();

    restoreCookieJar(authCookies);
    const linkInbox = await listOwnerLinkAsk(workspaceSlug, link.id, {
      lane: "host",
      status: "host_pending",
    });
    expect(linkInbox.some((t) => t.id === created.id && t.question === question)).toBe(true);

    const roomInbox = await listOwnerRoomAsk(workspaceSlug, room.id, {
      linkId: link.id,
      lane: "host",
      status: "host_pending",
    });
    expect(roomInbox.some((t) => t.id === created.id)).toBe(true);

    const answered = await answerOwnerAskTurn(workspaceSlug, link.id, created.id, answer);
    expect(answered.status).toBe("host_answered");
    expect(answered.host_answer).toBe(answer);

    restoreCookieJar(visitorCookies);
    const mine = await listMyPublicAskTurns(link.publicToken);
    const turn = mine.find((t) => t.id === created.id);
    expect(turn?.status).toBe("host_answered");
    expect(turn?.host_answer).toBe(answer);

    restoreCookieJar(authCookies);
  });

  test("document-only link rejects public ask with qa_disabled", async () => {
    const authCookies = snapshotCookieJar();
    const doc = await seedDocument(workspaceSlug);
    const docLink = await seedLink(workspaceSlug, doc.id, {
      name: `Doc Ask Block ${Date.now()}`,
      permissionType: "public",
    });

    restoreCookieJar([]);
    await accessPublicLinkApi(docLink.publicToken, `doc-visitor-${Date.now()}@example.com`);
    try {
      await submitPublicAsk(docLink.publicToken, "Should be blocked");
      throw new Error("expected qa_disabled");
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      expect(message).toContain("403");
      expect(message.toLowerCase()).toContain("qa_disabled");
    } finally {
      restoreCookieJar(authCookies);
    }
  });

  test("PATCH ask-policy rejects ask_ai_enabled on document-only link", async () => {
    const authCookies = snapshotCookieJar();
    const doc = await seedDocument(workspaceSlug);
    const docLink = await seedLink(workspaceSlug, doc.id, {
      name: `Doc Ask Policy ${Date.now()}`,
      permissionType: "public",
    });

    const res = await apiFetch(
      `/api/workspaces/${workspaceSlug}/links/${docLink.id}/ask-policy`,
      {
        method: "PATCH",
        body: JSON.stringify({ ask_ai_enabled: true }),
      },
    );
    expect(res.status).toBe(400);
    const body = (await res.json()) as { code?: string };
    expect(body.code).toBe("invalid_input");
    restoreCookieJar(authCookies);
  });

  test("deal-room link rejects AI stream when ask_ai_enabled is false", async () => {
    const authCookies = snapshotCookieJar();
    const doc = await seedDocument(workspaceSlug);
    const room = await seedDealRoom(workspaceSlug, {
      name: "Visitor Ask AI Gate Room",
      templateType: "seed",
      documentIds: [doc.id],
    });
    const link = await seedDealRoomLink(workspaceSlug, room.id, {
      name: `Ask AI Gate Link ${Date.now()}`,
    });

    restoreCookieJar([]);
    await accessPublicLinkApi(link.publicToken, `ai-gate-${Date.now()}@example.com`);
    const created = await submitPublicAsk(link.publicToken, "Should not stream AI");
    const streamRes = await streamPublicAskTurn(link.publicToken, created.id);
    expect(streamRes.status).toBe(403);
    const body = (await streamRes.json()) as { code?: string };
    expect(body.code).toBe("ai_not_enabled");
    restoreCookieJar(authCookies);
  });

  test("PATCH ask-policy enables AI and GET link reflects askAiEnabled", async () => {
    const authCookies = snapshotCookieJar();
    const doc = await seedDocument(workspaceSlug);
    const room = await seedDealRoom(workspaceSlug, {
      name: "Visitor Ask Policy Room",
      templateType: "seed",
      documentIds: [doc.id],
    });
    const link = await seedDealRoomLink(workspaceSlug, room.id, {
      name: `Ask Policy Link ${Date.now()}`,
    });

    const before = await fetchLinkById(workspaceSlug, link.id);
    expect(before.askAiEnabled).not.toBe(true);

    await updateLinkAskPolicy(workspaceSlug, link.id, { askAiEnabled: true });
    const after = await fetchLinkById(workspaceSlug, link.id);
    expect(after.askAiEnabled).toBe(true);

    await updateLinkAskPolicy(workspaceSlug, link.id, { askAiEnabled: false });
    const disabled = await fetchLinkById(workspaceSlug, link.id);
    expect(disabled.askAiEnabled).toBe(false);
    restoreCookieJar(authCookies);
  });

  test("AI stream is not ai_not_enabled after ask-policy enable", async () => {
    const authCookies = snapshotCookieJar();
    const doc = await seedDocument(workspaceSlug);
    const room = await seedDealRoom(workspaceSlug, {
      name: "Visitor Ask Policy Stream Room",
      templateType: "seed",
      documentIds: [doc.id],
    });
    const link = await seedDealRoomLink(workspaceSlug, room.id, {
      name: `Ask Policy Stream ${Date.now()}`,
    });

    restoreCookieJar([]);
    await accessPublicLinkApi(link.publicToken, `policy-stream-${Date.now()}@example.com`);
    const created = await submitPublicAsk(link.publicToken, "Policy stream gate question?");

    const blocked = await streamPublicAskTurn(link.publicToken, created.id);
    expect(blocked.status).toBe(403);
    const blockedBody = (await blocked.json()) as { code?: string };
    expect(blockedBody.code).toBe("ai_not_enabled");
    const visitorCookies = snapshotCookieJar();

    restoreCookieJar(authCookies);
    await updateLinkAskPolicy(workspaceSlug, link.id, { askAiEnabled: true });

    restoreCookieJar(visitorCookies);
    const allowed = await streamPublicAskTurn(link.publicToken, created.id);
    if (allowed.status === 403) {
      const allowedBody = (await allowed.json()) as { code?: string };
      expect(allowedBody.code).not.toBe("ai_not_enabled");
    }
    restoreCookieJar(authCookies);
  });

  test("AI quota exceeded falls back to host lane", async () => {
    const authCookies = snapshotCookieJar();
    const doc = await seedDocument(workspaceSlug);
    const room = await seedDealRoom(workspaceSlug, {
      name: "Visitor Ask Quota Room",
      templateType: "seed",
      documentIds: [doc.id],
    });
    const link = await seedDealRoomLink(workspaceSlug, room.id, {
      name: `Ask Quota Link ${Date.now()}`,
    });

    await updateLinkAskPolicy(workspaceSlug, link.id, {
      askAiEnabled: true,
      askAiMonthlyQuota: 0,
    });

    restoreCookieJar([]);
    await accessPublicLinkApi(link.publicToken, `quota-${Date.now()}@example.com`);
    const created = await submitPublicAsk(link.publicToken, "Quota exceeded question?");
    expect(created.lane).toBe("host");
    expect(created.status).toBe("host_pending");
    expect(created.route_reason).toBe("ai_quota_exceeded");

    restoreCookieJar(authCookies);
  });

  test("link analytics ask_summary reflects host ask activity", async () => {
    const authCookies = snapshotCookieJar();
    const doc = await seedDocument(workspaceSlug);
    const room = await seedDealRoom(workspaceSlug, {
      name: "Visitor Ask Analytics Room",
      templateType: "seed",
      documentIds: [doc.id],
    });
    const link = await seedDealRoomLink(workspaceSlug, room.id, {
      name: `Ask Analytics Link ${Date.now()}`,
    });

    restoreCookieJar([]);
    await accessPublicLinkApi(link.publicToken, `analytics-${Date.now()}@example.com`);
    await submitPublicAsk(link.publicToken, "Analytics summary question?");

    restoreCookieJar(authCookies);
    const analytics = await fetchLinkAnalytics(workspaceSlug, link.id);
    expect(analytics.ask_summary?.host_pending).toBe(1);
    expect(analytics.ask_summary?.host_answered).toBe(0);
    expect(analytics.ask_summary?.deflection_rate).toBeUndefined();

    restoreCookieJar(authCookies);
  });

  test("dashboard action sync creates deal_room_link_question with room/link target", async () => {
    const authCookies = snapshotCookieJar();
    const doc = await seedDocument(workspaceSlug);
    const room = await seedDealRoom(workspaceSlug, {
      name: "Visitor Ask Action Room",
      templateType: "seed",
      documentIds: [doc.id],
    });
    const link = await seedDealRoomLink(workspaceSlug, room.id, {
      name: `Ask Action Link ${Date.now()}`,
    });

    const visitorEmail = `ask-action-${Date.now()}@example.com`;
    const question = "Dashboard action contract question?";

    restoreCookieJar([]);
    await accessPublicLinkApi(link.publicToken, visitorEmail);
    const created = await submitPublicAsk(link.publicToken, question);
    restoreCookieJar(authCookies);

    const actions = await fetchDashboardActionItems(workspaceSlug);
    const actionSourceId = created.host_question_id ?? created.id;
    const todo = actions.find(
      (a) =>
        a.sourceType === "deal_room_link_question" &&
        a.sourceId === actionSourceId &&
        a.status === "pending",
    );
    expect(todo).toBeTruthy();
    expect(todo?.targetId).toBe(`${room.id}/${link.id}`);
    expect(todo?.title).toMatch(/Answer visitor Ask from/i);
  });
});
