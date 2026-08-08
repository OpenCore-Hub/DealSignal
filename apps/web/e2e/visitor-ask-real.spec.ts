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
  pinOwnerAskTurnFAQ,
  unpinOwnerAskTurnFAQ,
  listPublicAskFAQs,
  listPublicFormalAsk,
  publishFormalAskTurn,
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

  test("host answered turn: pin FAQ → visitor sees it → unpin hides it", async () => {
    const authCookies = snapshotCookieJar();
    const doc = await seedDocument(workspaceSlug);
    const room = await seedDealRoom(workspaceSlug, {
      name: "Visitor Ask Pin FAQ Room",
      templateType: "seed",
      documentIds: [doc.id],
    });
    const link = await seedDealRoomLink(workspaceSlug, room.id, {
      name: `Ask Pin FAQ Link ${Date.now()}`,
    });

    const visitorEmail = `ask-faq-visitor-${Date.now()}@example.com`;
    const question = "Real API: what is the runway?";
    const answer = "Runway is 18 months at current burn.";

    restoreCookieJar([]);
    await accessPublicLinkApi(link.publicToken, visitorEmail);
    const created = await submitPublicAsk(link.publicToken, question);
    const visitorCookies = snapshotCookieJar();

    restoreCookieJar(authCookies);
    await answerOwnerAskTurn(workspaceSlug, link.id, created.id, answer);
    const pinned = await pinOwnerAskTurnFAQ(workspaceSlug, link.id, created.id);
    expect(pinned.pinned_faq_at).toBeTruthy();

    restoreCookieJar(visitorCookies);
    let faqs = await listPublicAskFAQs(link.publicToken);
    expect(faqs.some((faq) => faq.id === created.id && faq.answer === answer)).toBe(true);

    restoreCookieJar(authCookies);
    await unpinOwnerAskTurnFAQ(workspaceSlug, link.id, created.id);

    restoreCookieJar(visitorCookies);
    faqs = await listPublicAskFAQs(link.publicToken);
    expect(faqs.some((faq) => faq.id === created.id)).toBe(false);

    restoreCookieJar(authCookies);
  });

  test("formal mode: schedule hides board; immediate publish surfaces on visitor formal API", async () => {
    const authCookies = snapshotCookieJar();
    const doc = await seedDocument(workspaceSlug);
    const room = await seedDealRoom(workspaceSlug, {
      name: "Visitor Ask Formal Board Room",
      templateType: "seed",
      documentIds: [doc.id],
    });
    const link = await seedDealRoomLink(workspaceSlug, room.id, {
      name: `Ask Formal Board Link ${Date.now()}`,
    });

    await updateLinkAskPolicy(workspaceSlug, link.id, { askMode: "formal" });

    const visitorEmail = `ask-formal-visitor-${Date.now()}@example.com`;
    const question = "Real API formal: revenue guidance?";
    const answer = "Approved guidance: low-teens growth.";

    restoreCookieJar([]);
    await accessPublicLinkApi(link.publicToken, visitorEmail);
    const created = await submitPublicAsk(link.publicToken, question);
    expect(created.formal_status).toBe("pending_review");
    const visitorCookies = snapshotCookieJar();

    let formalBoard = await listPublicFormalAsk(link.publicToken);
    expect(formalBoard.some((entry) => entry.id === created.id)).toBe(false);

    restoreCookieJar(authCookies);
    const future = new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString();
    const scheduled = await publishFormalAskTurn(workspaceSlug, link.id, created.id, answer, {
      publishAt: future,
    });
    expect(scheduled.formal_status).toBe("scheduled");

    restoreCookieJar(visitorCookies);
    formalBoard = await listPublicFormalAsk(link.publicToken);
    expect(formalBoard.some((entry) => entry.id === created.id)).toBe(false);

    restoreCookieJar(authCookies);
    const published = await publishFormalAskTurn(workspaceSlug, link.id, created.id, answer);
    expect(published.formal_status).toBe("published");

    restoreCookieJar(visitorCookies);
    formalBoard = await listPublicFormalAsk(link.publicToken);
    expect(formalBoard.some((entry) => entry.id === created.id && entry.answer === answer)).toBe(
      true,
    );
    const publishedEntry = formalBoard.find((entry) => entry.id === created.id);
    // Default publish keeps formal_anonymize=true.
    expect(publishedEntry?.visitor_email).toBeUndefined();

    restoreCookieJar(authCookies);
  });

  test("formal schedule due is published by background worker without visitor lazy-on-read", async () => {
    test.setTimeout(90_000);
    const authCookies = snapshotCookieJar();
    const doc = await seedDocument(workspaceSlug);
    const room = await seedDealRoom(workspaceSlug, {
      name: "Visitor Ask Formal Worker Room",
      templateType: "seed",
      documentIds: [doc.id],
    });
    const link = await seedDealRoomLink(workspaceSlug, room.id, {
      name: `Ask Formal Worker Link ${Date.now()}`,
    });

    await updateLinkAskPolicy(workspaceSlug, link.id, { askMode: "formal" });

    const visitorEmail = `ask-formal-worker-${Date.now()}@example.com`;
    const question = "Real API formal worker: delayed disclosure?";
    const answer = "Worker-published guidance: mid-teens growth.";

    restoreCookieJar([]);
    await accessPublicLinkApi(link.publicToken, visitorEmail);
    const created = await submitPublicAsk(link.publicToken, question);

    restoreCookieJar(authCookies);
    const dueAt = new Date(Date.now() + 3_000).toISOString();
    const scheduled = await publishFormalAskTurn(workspaceSlug, link.id, created.id, answer, {
      publishAt: dueAt,
    });
    expect(scheduled.formal_status).toBe("scheduled");

    // Owner inbox does not lazy-publish; wait for due + FormalPublishWorker tick.
    const deadline = Date.now() + 45_000;
    let published = false;
    while (Date.now() < deadline) {
      const ownerTurns = await listOwnerLinkAsk(workspaceSlug, link.id);
      const turn = ownerTurns.find((row) => row.id === created.id);
      if (turn?.formal_status === "published") {
        published = true;
        break;
      }
      await new Promise((r) => setTimeout(r, 2_000));
    }
    expect(published, "expected FormalPublishWorker to publish due scheduled turn").toBe(true);

    restoreCookieJar([]);
    await accessPublicLinkApi(link.publicToken, visitorEmail);
    const formalBoard = await listPublicFormalAsk(link.publicToken);
    expect(formalBoard.some((entry) => entry.id === created.id && entry.answer === answer)).toBe(
      true,
    );

    restoreCookieJar(authCookies);
  });

  test("formal publish with anonymize false exposes visitor email on public board", async () => {
    const authCookies = snapshotCookieJar();
    const doc = await seedDocument(workspaceSlug);
    const room = await seedDealRoom(workspaceSlug, {
      name: "Visitor Ask Formal Attribution Room",
      templateType: "seed",
      documentIds: [doc.id],
    });
    const link = await seedDealRoomLink(workspaceSlug, room.id, {
      name: `Ask Formal Attribution Link ${Date.now()}`,
    });

    await updateLinkAskPolicy(workspaceSlug, link.id, { askMode: "formal" });

    const visitorEmail = `ask-formal-attribution-${Date.now()}@example.com`;
    const question = "Real API formal attribution: who asked?";
    const answer = "Attribution is allowed on this disclosure.";

    restoreCookieJar([]);
    await accessPublicLinkApi(link.publicToken, visitorEmail);
    const created = await submitPublicAsk(link.publicToken, question);
    const visitorCookies = snapshotCookieJar();

    restoreCookieJar(authCookies);
    await publishFormalAskTurn(workspaceSlug, link.id, created.id, answer, {
      anonymize: false,
    });

    restoreCookieJar(visitorCookies);
    const formalBoard = await listPublicFormalAsk(link.publicToken);
    const entry = formalBoard.find((row) => row.id === created.id);
    expect(entry?.answer).toBe(answer);
    expect(entry?.visitor_email).toBe(visitorEmail);

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
    const actionSourceId = created.id;
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
