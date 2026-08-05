import { http, HttpResponse } from "msw";
import type {
  ActionItem,
  Contact,
  DealRoom,
  DealRoomDocumentItem,
  DealRoomFolder,
  DealRoomFolderDocs,
  DealRoomMember,
  Link,
  VisitorQuestion,
  WorkspaceMember,
} from "@/types";
import {
  mockAccessLogs,
  mockActionItems,
  mockActivities,
  mockContacts,
  mockDealRooms,
  mockDealRoomTemplates,
  mockDocuments,
  mockHeatAlerts,
  mockLinks,
  mockLinkAccessRequests,
  mockPageAnalytics,
  mockSignals,
  mockSuggestions,
  mockWorkspaceMembers,
  mockWorkspaces,
  defaultWorkspaceSettings,
  getMockDashboardStats,
  getMockSignalFeed,
} from "./data";

let workspaceSettings = { ...defaultWorkspaceSettings };

let integrationsStatus = {
  slack: false,
  hubspot: false,
  zapier: false,
};

let securitySettings = {
  forceEmailVerification: true,
  watermarkDownloads: false,
  twoFactorEnabled: false,
};

type MockLinkExt = Link & {
  _requireEmailVerification?: boolean;
  _requirePassword?: boolean;
  _requireNDA?: boolean;
  _password?: string;
  _allowedEmails?: string[];
  contactIds?: string[];
};

function normalizeMockEmail(email: string): string {
  return email.trim().toLowerCase();
}

/** Mirror backend document-link SoT: contact_ids drive the allowlist when present. */
function resolveDocumentAllowEmails(opts: {
  contactIds?: string[];
  allowedEmails?: string[];
}): string[] {
  if (opts.contactIds && opts.contactIds.length > 0) {
    const seen = new Set<string>();
    const out: string[] = [];
    for (const id of opts.contactIds) {
      const contact = mockContacts.find((c) => c.id === id);
      const email = contact?.email ? normalizeMockEmail(contact.email) : "";
      if (!email || seen.has(email)) continue;
      seen.add(email);
      out.push(email);
    }
    return out;
  }
  return (opts.allowedEmails ?? [])
    .map(normalizeMockEmail)
    .filter((email, index, arr) => email.length > 0 && arr.indexOf(email) === index);
}

function findMockLinkByPublicToken(token: string): MockLinkExt | undefined {
  return mockLinks.find((l) => l.shortUrl.endsWith(token)) as MockLinkExt | undefined;
}

// Snapshot of initial state so E2E tests can reset between cases.
const initialState = {
  workspaces: structuredClone(mockWorkspaces),
  documents: structuredClone(mockDocuments),
  links: structuredClone(mockLinks),
  dealRooms: structuredClone(mockDealRooms),
  members: structuredClone(mockWorkspaceMembers),
  settings: structuredClone(defaultWorkspaceSettings),
  integrations: structuredClone(integrationsStatus),
  security: structuredClone(securitySettings),
};

// In-memory auth store for the mock environment.
interface MockUser {
  id: string;
  email: string;
  password: string;
  name: string;
}
const mockUsers = new Map<string, MockUser>();
/** Per-link visitor Ask Host questions for public MSW e2e. */
const mockPublicQuestions = new Map<string, VisitorQuestion[]>();
/** Per-link Ask Host questions for owner inbox (room + link). */
const mockOwnerQuestions = new Map<string, VisitorQuestion[]>();

/** In-memory knowledge Q&A sessions/turns/feedback for MSW e2e (Phase A–C). */
type MockKnowledgeFeedback = { kind: string; note?: string };
type MockKnowledgeTurn = {
  id: string;
  sessionId: string;
  sequence: number;
  question: string;
  answer?: string;
  refused: boolean;
  resultStatus: string;
  hits: Array<{
    chunkId: string;
    documentId?: string;
    text: string;
    score: number;
    sourceName?: string;
    pages?: number[];
    viewerPage?: number;
  }>;
  retrieveQuery?: string;
  rewriteApplied?: boolean;
  claims?: Array<{ text: string; hitIds?: string[]; confidence?: string }>;
  unresolved?: string[];
  createdAt: string;
  feedback?: MockKnowledgeFeedback;
};

function mockBindClaims(
  answer: string | undefined,
  hits: MockKnowledgeTurn["hits"],
  refused: boolean,
): Pick<MockKnowledgeTurn, "claims" | "unresolved"> {
  if (refused || !answer?.trim() || hits.length === 0) return {};
  const citeRe = /\[(\d+)\]/g;
  const sentences = answer
    .split(/(?<=[。！？.!?])\s*/)
    .map((s) => s.trim())
    .filter(Boolean);
  const claims: NonNullable<MockKnowledgeTurn["claims"]> = [];
  for (const sent of sentences.length ? sentences : [answer.trim()]) {
    const nums: number[] = [];
    let m: RegExpExecArray | null;
    const re = new RegExp(citeRe.source, "g");
    while ((m = re.exec(sent)) !== null) {
      const n = Number(m[1]);
      if (n > 0) nums.push(n);
    }
    const text = sent.replace(citeRe, "").replace(/\s+/g, " ").trim();
    if (!text) continue;
    const hitIds = [
      ...new Set(
        nums
          .map((n) => hits[n - 1]?.chunkId)
          .filter((id): id is string => !!id),
      ),
    ];
    claims.push({
      text,
      hitIds: hitIds.length ? hitIds : undefined,
      confidence: hitIds.length ? "grounded" : undefined,
    });
  }
  return claims.length ? { claims } : {};
}
type MockKnowledgeSessionState = {
  entities: Array<{
    name: string;
    type: string;
    firstTurnId: string;
    hitIds?: string[];
  }>;
  openQuestions: Array<{ text: string; sourceTurnId: string }>;
  coverageHints: Array<{ sourceNames: string[]; turnId: string }>;
};
type MockKnowledgeSession = {
  id: string;
  roomId: string;
  title: string;
  status: "active" | "closed";
  createdAt: string;
  updatedAt: string;
  lastTurnAt?: string;
  turnCount: number;
  state?: MockKnowledgeSessionState;
};

function emptyMockKnowledgeState(): MockKnowledgeSessionState {
  return { entities: [], openQuestions: [], coverageHints: [] };
}

function evolveMockKnowledgeState(
  prev: MockKnowledgeSessionState | undefined,
  turn: MockKnowledgeTurn,
): MockKnowledgeSessionState | undefined {
  const next: MockKnowledgeSessionState = {
    entities: [...(prev?.entities ?? [])],
    openQuestions: [...(prev?.openQuestions ?? [])],
    coverageHints: [...(prev?.coverageHints ?? [])],
  };
  const coverage: string[] = [];
  const seen = new Set<string>();
  for (const h of turn.hits) {
    const name = (h.sourceName || "").trim();
    if (!name) continue;
    const key = name.toLowerCase();
    if (!seen.has(key)) {
      seen.add(key);
      coverage.push(name);
    }
    const existing = next.entities.find((e) => e.name.toLowerCase() === key);
    if (existing) {
      const ids = new Set(existing.hitIds ?? []);
      if (h.chunkId) ids.add(h.chunkId);
      existing.hitIds = [...ids].slice(0, 8);
    } else {
      next.entities.push({
        name,
        type: "document",
        firstTurnId: turn.id,
        hitIds: h.chunkId ? [h.chunkId] : [],
      });
    }
  }
  if (coverage.length > 0) {
    next.coverageHints.push({ sourceNames: coverage.slice(0, 3), turnId: turn.id });
    if (next.coverageHints.length > 5) {
      next.coverageHints = next.coverageHints.slice(-5);
    }
  }
  if (
    turn.resultStatus === "refused" ||
    turn.resultStatus === "no_hits" ||
    turn.resultStatus === "error"
  ) {
    const q = turn.question.trim();
    if (q && !next.openQuestions.some((o) => o.text.toLowerCase() === q.toLowerCase())) {
      next.openQuestions.push({ text: q, sourceTurnId: turn.id });
      if (next.openQuestions.length > 12) {
        next.openQuestions = next.openQuestions.slice(-12);
      }
    }
  }
  if (
    next.entities.length === 0 &&
    next.openQuestions.length === 0 &&
    next.coverageHints.length === 0
  ) {
    return undefined;
  }
  return next;
}
const mockKnowledgeSessionsByRoom = new Map<string, MockKnowledgeSession[]>();
const mockKnowledgeTurnsBySession = new Map<string, MockKnowledgeTurn[]>();
/** E2E override for GET …/knowledge corpus status (A5). Survives reload via Cache. */
type MockKnowledgeCorpusOverride = {
  status: string;
  documentStatus: string;
  jobStatus: string;
};
const mockKnowledgeCorpusOverrideByRoom = new Map<string, MockKnowledgeCorpusOverride>();
/** roomId\\0clientRequestId → turn payload for ask idempotency replays. */
const mockKnowledgeTurnByClientRequest = new Map<string, Record<string, unknown>>();
/** roomId → bound mission pack id. */
const mockKnowledgeMissionByRoom = new Map<string, string>();

/** Builtin knowledge mission packs (mirrors apps/api/internal/knowledge/missions). */
const mockKnowledgeMissionCatalog: Array<{
  packId: string;
  title: string;
  items: Array<{ id: string; prompt: string }>;
}> = [
  {
    packId: "financing_dd_v1",
    title: "Financing due diligence",
    items: [
      {
        id: "valuation_cap",
        prompt: "What valuation cap or pre-money valuation appears in this room’s financing docs?",
      },
    ],
  },
  {
    packId: "first_fund_v1",
    title: "First-time fundraise",
    items: [
      {
        id: "fund_size_strategy",
        prompt: "What fund size and investment strategy are stated in this room’s fund docs?",
      },
    ],
  },
  {
    packId: "ma_redflag_v1",
    title: "M&A red-flag review",
    items: [
      {
        id: "change_of_control",
        prompt: "Which contracts have change-of-control termination rights in this room?",
      },
    ],
  },
  {
    packId: "series_a_plus_v1",
    title: "Series A+ growth diligence",
    items: [
      {
        id: "unit_economics",
        prompt: "What unit economics (LTV, CAC, payback) are stated in this room’s financials?",
      },
    ],
  },
  {
    packId: "real_estate_v1",
    title: "Real estate transaction diligence",
    items: [
      {
        id: "title_encumbrances",
        prompt: "What title exceptions, liens, or easements are disclosed in this room’s title docs?",
      },
    ],
  },
  {
    packId: "fund_mgmt_v1",
    title: "Fund operations & LP reporting",
    items: [
      {
        id: "capital_calls",
        prompt: "What capital-call or unfunded-commitment terms are in this room’s fund docs?",
      },
    ],
  },
  {
    packId: "portfolio_mgmt_v1",
    title: "Portfolio company monitoring",
    items: [
      {
        id: "portfolio_kpis",
        prompt: "What portfolio KPI or performance figures are reported in this room’s materials?",
      },
    ],
  },
  {
    packId: "project_mgmt_v1",
    title: "Project delivery diligence",
    items: [
      {
        id: "scope_objectives",
        prompt: "What project scope and objectives are defined in this room’s charter or overview?",
      },
    ],
  },
  {
    packId: "sales_dataroom_v1",
    title: "Enterprise sales diligence",
    items: [
      {
        id: "pricing_quote",
        prompt: "What pricing, quote amounts, or discount terms appear in this room’s proposals?",
      },
    ],
  },
];

function mockMissionPack(packId: string) {
  return (
    mockKnowledgeMissionCatalog.find((p) => p.packId === packId) ??
    mockKnowledgeMissionCatalog[0]!
  );
}
/** roomId → feedback→gold candidates (ceiling Phase O). */
type MockEvalCandidate = {
  id: string;
  roomId: string;
  turnId: string;
  feedbackKind: string;
  question: string;
  answer?: string;
  note?: string;
  reviewStatus: string;
  expect?: string;
  createdAt: string;
  reviewedAt?: string;
  snapshot?: {
    hits?: Array<{ chunkId?: string; sourceName?: string; excerpt?: string }>;
  };
};
const mockKnowledgeEvalCandidatesByRoom = new Map<string, MockEvalCandidate[]>();

/** roomId → cold archive tombstones + packs (ceiling Phase U). */
type MockKnowledgeArchive = {
  id: string;
  workspaceId: string;
  roomId: string;
  sessionId: string;
  title?: string;
  turnCount: number;
  corpusFingerprint?: string;
  status: string;
  archivedAt: string;
  pack: {
    schemaVersion: string;
    exportedAt: string;
    workspaceId: string;
    roomId: string;
    sessionId: string;
    corpusFingerprint?: string;
    session: { id: string; status: string; title?: string };
    turns: Array<{
      id: string;
      sequence: number;
      question: string;
      answer?: string;
      resultStatus: string;
    }>;
  };
};
const mockKnowledgeArchivesByRoom = new Map<string, MockKnowledgeArchive[]>();

function seedDefaultKnowledgeArchives() {
  // room_1 demo tombstone for corpus landing / Phase U e2e.
  mockKnowledgeArchivesByRoom.set("room_1", [
    {
      id: "kqa_arch_1",
      workspaceId: "ws_1",
      roomId: "room_1",
      sessionId: "kqa_sess_archived_1",
      title: "Archived diligence session",
      turnCount: 1,
      corpusFingerprint: "mockfp0123456789abcdef",
      status: "cold",
      archivedAt: "2026-07-01T12:00:00Z",
      pack: {
        schemaVersion: "1",
        exportedAt: "2026-07-01T12:00:00Z",
        workspaceId: "ws_1",
        roomId: "room_1",
        sessionId: "kqa_sess_archived_1",
        corpusFingerprint: "mockfp0123456789abcdef",
        session: {
          id: "kqa_sess_archived_1",
          status: "closed",
          title: "Archived diligence session",
        },
        turns: [
          {
            id: "kqa_turn_archived_1",
            sequence: 1,
            question: "What was the valuation cap?",
            answer: "Grounded answer for: What was the valuation cap? [1].",
            resultStatus: "answered",
          },
        ],
      },
    },
  ]);
}
seedDefaultKnowledgeArchives();

/** Test-only: force session ask to return JSON 429 before SSE (busy / rate / quota). */
let mockKnowledgeAskGate: { code: string; status: number } | null = null;

function clientRequestKey(roomId: string, clientRequestId: string) {
  return `${roomId}\0${clientRequestId}`;
}

function resetMockState() {
  mockUsers.clear();
  mockPublicQuestions.clear();
  mockOwnerQuestions.clear();
  void resetKnowledgeQAState();
  seedOwnerAskHostQuestions();
  mockWorkspaces.splice(0, mockWorkspaces.length, ...initialState.workspaces);
  mockDocuments.splice(0, mockDocuments.length, ...initialState.documents);
  mockLinks.splice(0, mockLinks.length, ...initialState.links);
  mockDealRooms.splice(0, mockDealRooms.length, ...initialState.dealRooms);
  mockWorkspaceMembers.splice(0, mockWorkspaceMembers.length, ...initialState.members);
  workspaceSettings = { ...initialState.settings };
  integrationsStatus = { ...initialState.integrations };
  securitySettings = { ...initialState.security };
}
const KNOWLEDGE_QA_CACHE = "msw-e2e-knowledge-qa";
const KNOWLEDGE_QA_STATE_URL = "https://msw.local/knowledge-qa-state";
let knowledgeQAHydrated = false;
let knowledgeQAHydratePromise: Promise<void> | null = null;

async function hydrateKnowledgeQAState() {
  if (knowledgeQAHydrated) return;
  if (typeof caches === "undefined") {
    knowledgeQAHydrated = true;
    return;
  }
  if (!knowledgeQAHydratePromise) {
    knowledgeQAHydratePromise = (async () => {
      try {
        const cache = await caches.open(KNOWLEDGE_QA_CACHE);
        const res = await cache.match(KNOWLEDGE_QA_STATE_URL);
        if (res) {
          const data = (await res.json()) as {
            sessions?: [string, MockKnowledgeSession[]][];
            turns?: [string, MockKnowledgeTurn[]][];
            corpusOverrides?: [string, MockKnowledgeCorpusOverride][];
            askGate?: { code: string; status: number } | null;
            clientRequests?: [string, Record<string, unknown>][];
          };
          mockKnowledgeSessionsByRoom.clear();
          mockKnowledgeTurnsBySession.clear();
          mockKnowledgeCorpusOverrideByRoom.clear();
          mockKnowledgeTurnByClientRequest.clear();
          mockKnowledgeAskGate = data.askGate ?? null;
          for (const [roomId, sessions] of data.sessions ?? []) {
            mockKnowledgeSessionsByRoom.set(roomId, sessions);
          }
          for (const [sessionId, turns] of data.turns ?? []) {
            mockKnowledgeTurnsBySession.set(sessionId, turns);
          }
          for (const [roomId, override] of data.corpusOverrides ?? []) {
            mockKnowledgeCorpusOverrideByRoom.set(roomId, override);
          }
          for (const [key, payload] of data.clientRequests ?? []) {
            mockKnowledgeTurnByClientRequest.set(key, payload);
          }
        }
      } catch {
        /* ignore cache hydrate failures in non-SW contexts */
      } finally {
        knowledgeQAHydrated = true;
      }
    })();
  }
  await knowledgeQAHydratePromise;
}

async function persistKnowledgeQAState() {
  if (typeof caches === "undefined") return;
  try {
    const cache = await caches.open(KNOWLEDGE_QA_CACHE);
    await cache.put(
      KNOWLEDGE_QA_STATE_URL,
      new Response(
        JSON.stringify({
          sessions: [...mockKnowledgeSessionsByRoom.entries()],
          turns: [...mockKnowledgeTurnsBySession.entries()],
          corpusOverrides: [...mockKnowledgeCorpusOverrideByRoom.entries()],
          askGate: mockKnowledgeAskGate,
          clientRequests: [...mockKnowledgeTurnByClientRequest.entries()],
        }),
        { headers: { "Content-Type": "application/json" } },
      ),
    );
  } catch {
    /* ignore */
  }
}

async function resetKnowledgeQAState() {
  mockKnowledgeSessionsByRoom.clear();
  mockKnowledgeTurnsBySession.clear();
  mockKnowledgeCorpusOverrideByRoom.clear();
  mockKnowledgeTurnByClientRequest.clear();
  mockKnowledgeMissionByRoom.clear();
  mockKnowledgeEvalCandidatesByRoom.clear();
  mockKnowledgeArchivesByRoom.clear();
  seedDefaultKnowledgeArchives();
  mockKnowledgeAskGate = null;
  knowledgeQAHydrated = true;
  knowledgeQAHydratePromise = null;
  if (typeof caches === "undefined") return;
  try {
    await caches.delete(KNOWLEDGE_QA_CACHE);
  } catch {
    /* ignore */
  }
}

/** Mirror apps/api answerTokenChunks — keep e2e stream contract aligned. */
function chunkMockAnswerTokens(answer: string, maxChars = 36): string[] {
  if (!answer) return [];
  if (maxChars < 8) maxChars = 8;
  if (answer.length <= maxChars) return [answer];
  const out: string[] = [];
  let i = 0;
  while (i < answer.length) {
    let end = Math.min(i + maxChars, answer.length);
    if (end < answer.length) {
      const minKeep = i + Math.floor(maxChars / 2);
      let j = end;
      while (j > minKeep && !/\s/.test(answer[j - 1]!)) j -= 1;
      if (j > minKeep) end = j;
    }
    out.push(answer.slice(i, end));
    i = end;
  }
  return out;
}

async function roomSessions(roomId: string): Promise<MockKnowledgeSession[]> {
  await hydrateKnowledgeQAState();
  let list = mockKnowledgeSessionsByRoom.get(roomId);
  if (!list) {
    list = [];
    mockKnowledgeSessionsByRoom.set(roomId, list);
  }
  return list;
}

/** Mirror production elliptical rewrite for MSW desk trust disclosure. */
function mockKnowledgeRewrite(
  displayQuery: string,
  prior?: MockKnowledgeTurn,
): Pick<MockKnowledgeTurn, "retrieveQuery" | "rewriteApplied"> {
  if (!prior) return {};
  const q = displayQuery.trim();
  if (!q) return {};
  const lower = q.toLowerCase();
  const short = [...q].length <= 28;
  const pronoun =
    /它|他们|她们|这个|那个|上述|该|呢[？?]?$/.test(q) ||
    /\b(they|them|their|this|that|those|these|it|it's)\b/.test(lower) ||
    /\b(what about|how about|and the|same for)\b/.test(lower);
  if (!short && !pronoun) return {};
  const retrieveQuery = `${prior.question} — ${q}`.slice(0, 240).trim();
  if (!retrieveQuery || retrieveQuery === q) return {};
  return { retrieveQuery, rewriteApplied: true };
}

async function executeMockKnowledgeSessionQuery(
  roomId: string,
  body: {
    sessionId?: string;
    query?: string;
    answer?: boolean;
    top_k?: number;
    clientRequestId?: string;
  },
): Promise<
  | { ok: true; payload: Record<string, unknown> }
  | { ok: false; status: number; body: Record<string, unknown> }
> {
  await hydrateKnowledgeQAState();
  if (mockKnowledgeAskGate) {
    return {
      ok: false,
      status: mockKnowledgeAskGate.status,
      body: {
        code: mockKnowledgeAskGate.code,
        message: mockKnowledgeAskGate.code,
      },
    };
  }
  const room = findRoom(roomId);
  if (!room) return { ok: false, status: 404, body: { code: "not_found" } };
  const rawQuery = (body.query || "").trim();
  if (!rawQuery) {
    return {
      ok: false,
      status: 400,
      body: { code: "invalid_input", message: "query is required" },
    };
  }
  const clientRequestId = (body.clientRequestId || "").trim();
  if (!clientRequestId) {
    return {
      ok: false,
      status: 400,
      body: { code: "invalid_input", message: "clientRequestId is required" },
    };
  }
  const priorAsk = mockKnowledgeTurnByClientRequest.get(
    clientRequestKey(roomId, clientRequestId),
  );
  if (priorAsk) {
    const priorQ = String((priorAsk.turn as { question?: string } | undefined)?.question ?? "");
    if (priorQ && priorQ !== rawQuery.replace(/^@refuse\s+/i, "").trim() && priorQ !== rawQuery) {
      return {
        ok: false,
        status: 400,
        body: { code: "invalid_input", message: "clientRequestId reused with different question" },
      };
    }
    return { ok: true, payload: priorAsk };
  }
  // MSW refuse probe (A2): prefix `@refuse ` → refused turn with empty hits.
  const refuseProbe = /^@refuse\s+/i.test(rawQuery);
  const displayQuery = refuseProbe
    ? rawQuery.replace(/^@refuse\s+/i, "").trim() || rawQuery
    : rawQuery;
  const now = new Date().toISOString();
  const sessions = await roomSessions(roomId);
  let session = body.sessionId
    ? sessions.find((s) => s.id === body.sessionId)
    : undefined;
  if (!session || session.status !== "active") {
    for (const s of sessions) {
      if (s.status === "active") {
        s.status = "closed";
        s.updatedAt = now;
      }
    }
    session = {
      id: generateId("kqa_sess"),
      roomId,
      title: displayQuery.slice(0, 80),
      status: "active",
      createdAt: now,
      updatedAt: now,
      lastTurnAt: now,
      turnCount: 0,
      state: emptyMockKnowledgeState(),
    };
    sessions.unshift(session);
    mockKnowledgeTurnsBySession.set(session.id, []);
  }
  const doc = (room.documents ?? []).flatMap((folder) => folder.documents ?? [])[0];
  const hits =
    refuseProbe || !doc
      ? []
      : [
          {
            chunkId: generateId("chunk"),
            documentId: doc.document_id || doc.id,
            text: `Relevant passage from ${doc.title || "document"} about ${displayQuery}.`,
            score: 0.91,
            sourceName: doc.title || "Document",
            pages: [3, 4],
            viewerPage: 3,
          },
        ];
  const refused = refuseProbe;
  const answer = refused
    ? `The provided context does not contain an answer to the question '${displayQuery}'.`
    : body.answer === false
      ? undefined
      : hits.length > 0
        ? `Grounded answer for: ${displayQuery} [1].`
        : `Grounded answer for: ${displayQuery}`;
  const resultStatus = refused ? "refused" : hits.length ? "answered" : "no_hits";
  const turns = mockKnowledgeTurnsBySession.get(session.id) ?? [];
  const prior = turns.length > 0 ? turns[turns.length - 1] : undefined;
  const rewrite = mockKnowledgeRewrite(displayQuery, prior);
  const bound = mockBindClaims(answer, hits, refused);
  const turn: MockKnowledgeTurn = {
    id: generateId("kqa_turn"),
    sessionId: session.id,
    sequence: turns.length + 1,
    question: displayQuery,
    answer,
    refused,
    resultStatus,
    hits,
    ...rewrite,
    ...bound,
    createdAt: now,
  };
  turns.push(turn);
  mockKnowledgeTurnsBySession.set(session.id, turns);
  session.lastTurnAt = now;
  session.updatedAt = now;
  session.turnCount = turns.length;
  session.state = evolveMockKnowledgeState(session.state, turn);
  if (!session.title) session.title = displayQuery.slice(0, 80);
  const payload = {
    sessionId: session.id,
    turn,
    query: displayQuery,
    mode: "hybrid",
    answer: turn.answer,
    results: hits,
    refused: turn.refused,
    resultStatus: turn.resultStatus,
    sessionState: session.state,
  };
  mockKnowledgeTurnByClientRequest.set(clientRequestKey(roomId, clientRequestId), payload);
  await persistKnowledgeQAState();
  return {
    ok: true,
    payload,
  };
}

function seedOwnerAskHostQuestions() {
  mockOwnerQuestions.clear();
  const now = new Date().toISOString();
  mockOwnerQuestions.set("link_1", [
    {
      id: "owner_q_pending_1",
      link_id: "link_1",
      visitor_id: "visitor_owner_inbox",
      visitor_email: "lp@example.com",
      question: "Can you share the updated financial model?",
      status: "pending",
      created_at: now,
      updated_at: now,
    },
  ]);
}
seedOwnerAskHostQuestions();

function generateId(prefix: string): string {
  return `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2, 7)}`;
}

function createTokenResponse(userId: string, email: string) {
  return {
    user: { id: userId, email, name: email.split("@")[0] },
    expires_in: 900,
  };
}

function authSessionCookieHeader() {
  return { "Set-Cookie": "auth_session=1; Path=/; SameSite=Lax" };
}

function validatePassword(password: string): boolean {
  if (password.length < 8) return false;
  let hasUpper = false;
  let hasLower = false;
  let hasDigit = false;
  let hasSpecial = false;
  for (const ch of password) {
    if (/[A-Z]/.test(ch)) hasUpper = true;
    else if (/[a-z]/.test(ch)) hasLower = true;
    else if (/\d/.test(ch)) hasDigit = true;
    else if (/[^\w\s]/.test(ch)) hasSpecial = true;
  }
  return hasUpper && hasLower && hasDigit && hasSpecial;
}

function placeholdImageUrl(width: number, height: number): string {
  return `data:image/svg+xml,${encodeURIComponent(
    `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}"><rect width="100%" height="100%" fill="#e2e8f0"/><text x="50%" y="50%" dominant-baseline="middle" text-anchor="middle" fill="#64748b" font-size="24">Page</text></svg>`
  )}`;
}

function findRoom(roomId: string): DealRoom | undefined {
  return mockDealRooms.find((r) => r.id === roomId);
}

function getRoomFolders(room: DealRoom): DealRoomFolder[] {
  const folders = room.folders ?? [];
  if (folders.length === 0) {
    return [{ path: "/general", name: "General", sort_order: 0 }];
  }
  return folders.filter((f) => f.path !== "/");
}

function getRoomFolderDocs(room: DealRoom): DealRoomFolderDocs[] {
  return room.documents ?? [];
}

function nextSortOrder(arr: { sort_order: number }[]): number {
  return arr.length === 0 ? 0 : Math.max(...arr.map((x) => x.sort_order)) + 1;
}

function sanitizeFolderPath(name: string, parentPath = "/general"): string {
  const slug = name.toLowerCase().trim().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
  if (parentPath === "/") return `/${slug}`;
  return `${parentPath}/${slug}`;
}

function updateRoomDerivedFields(room: DealRoom) {
  const docs = getRoomFolderDocs(room);
  room.documentCount = docs.reduce((sum, fd) => sum + fd.documents.length, 0);
  room.memberCount = room.members?.length ?? 0;
  room.pendingApprovals = room.accessRequests?.filter((r) => r.status === "pending").length ?? 0;
  // Keep viewCount / activeLinkCount in sync with sensible mock defaults when not explicitly set.
  if (room.viewCount === undefined) room.viewCount = 0;
  if (room.activeLinkCount === undefined) room.activeLinkCount = 0;
  if (room.tags === undefined) room.tags = [];
}

export const handlers = [
  // Auth
  http.post("*/api/auth/register", async ({ request }) => {
    const body = (await request.json()) as { email?: string; password?: string };
    const email = body.email?.trim().toLowerCase();
    const password = body.password ?? "";
    if (!email || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      return HttpResponse.json({ code: "invalid_email", message: "invalid email address" }, { status: 400 });
    }
    if (!validatePassword(password)) {
      return HttpResponse.json(
        { code: "weak_password", message: "password must be at least 8 characters and include uppercase, lowercase, digit and special character" },
        { status: 400 }
      );
    }
    if (Array.from(mockUsers.values()).some((u) => u.email === email)) {
      return HttpResponse.json({ code: "email_conflict", message: "email already registered" }, { status: 409 });
    }
    const id = generateId("u");
    mockUsers.set(id, { id, email, password, name: email.split("@")[0] });
    return HttpResponse.json(createTokenResponse(id, email), { status: 201, headers: authSessionCookieHeader() });
  }),

  http.post("*/api/auth/login", async ({ request }) => {
    const body = (await request.json()) as { email?: string; password?: string };
    const email = body.email?.trim().toLowerCase();
    const user = Array.from(mockUsers.values()).find((u) => u.email === email);
    if (!user || user.password !== body.password) {
      return HttpResponse.json({ code: "unauthorized", message: "invalid email or password" }, { status: 401 });
    }
    return HttpResponse.json(createTokenResponse(user.id, user.email), { headers: authSessionCookieHeader() });
  }),

  http.post("*/api/auth/refresh", async () => {
    return HttpResponse.json({ expires_in: 900 }, { headers: authSessionCookieHeader() });
  }),

  http.post("*/api/auth/logout", async () => {
    return HttpResponse.json({ code: "ok", message: "logged out" }, {
      headers: { "Set-Cookie": "auth_session=; Path=/; Max-Age=0; SameSite=Lax" },
    });
  }),

  http.get("*/api/auth/verify-email/:token", () => {
    return HttpResponse.json({ code: "verified", message: "email verified successfully" });
  }),

  // Test-only reset (+ optional corpus override for A5). Same path always — MSW
  // already intercepts `/__e2e/reset` reliably in Playwright.
  http.post("*/__e2e/reset", async ({ request }) => {
    let body: {
      action?: string;
      roomId?: string;
      status?: string;
      documentStatus?: string;
      jobStatus?: string;
      code?: string;
      httpStatus?: number;
      clear?: boolean;
    } | null = null;
    try {
      const text = await request.text();
      if (text.trim()) body = JSON.parse(text) as NonNullable<typeof body>;
    } catch {
      body = null;
    }
    if (body?.action === "ping") {
      return new HttpResponse(null, { status: 204 });
    }
    if (body?.action === "knowledge-ask-gate") {
      await hydrateKnowledgeQAState();
      if (body.clear) {
        mockKnowledgeAskGate = null;
      } else {
        const code = (body.code || "knowledge_query_quota_exceeded").trim();
        const status = body.httpStatus && body.httpStatus >= 400 ? body.httpStatus : 429;
        mockKnowledgeAskGate = { code, status };
      }
      await persistKnowledgeQAState();
      return new HttpResponse(null, { status: 204 });
    }
    if (body?.action === "knowledge-corpus") {
      const roomId = (body.roomId || "").trim();
      if (!roomId) {
        return HttpResponse.json(
          { code: "invalid_input", message: "roomId is required" },
          { status: 400 },
        );
      }
      await hydrateKnowledgeQAState();
      mockKnowledgeCorpusOverrideByRoom.set(roomId, {
        status: body.status || "syncing",
        documentStatus: body.documentStatus || "syncing",
        jobStatus: body.jobStatus || "running",
      });
      await persistKnowledgeQAState();
      return new HttpResponse(null, { status: 204 });
    }
    resetMockState();
    return new HttpResponse(null, { status: 204 });
  }),

  // Workspaces
  http.get("*/api/workspaces", () => {
    return HttpResponse.json({ data: mockWorkspaces });
  }),

  http.post("*/api/workspaces", async ({ request }) => {
    const body = (await request.json()) as { name: string; slug: string; brand_color?: string };
    if (mockWorkspaces.some((w) => w.slug === body.slug)) {
      return HttpResponse.json(
        { code: "slug_conflict", message: "a workspace with this URL already exists" },
        { status: 409 }
      );
    }
    const newWorkspace = {
      id: generateId("ws"),
      name: body.name,
      slug: body.slug,
      logoUrl: "",
      brandColor: body.brand_color ?? "#0055ff",
    };
    mockWorkspaces.push(newWorkspace);
    workspaceSettings = { ...workspaceSettings, name: body.name, slug: body.slug };
    return HttpResponse.json(newWorkspace, { status: 201 });
  }),

  // Dashboard
  http.get("*/api/workspaces/:workspaceSlug/dashboard/stats", () => {
    return HttpResponse.json(getMockDashboardStats());
  }),

  // Documents
  http.get("*/api/workspaces/:workspaceSlug/documents", ({ request }) => {
    const url = new URL(request.url);
    const filter = url.searchParams.get("filter");
    const category = (url.searchParams.get("category") ?? "").toLowerCase();
    const excludeDealRoom = ["1", "true", "yes", "on"].includes(
      (url.searchParams.get("exclude_deal_room") ?? "").toLowerCase(),
    );
    const excludeAgreement = ["1", "true", "yes", "on"].includes(
      (url.searchParams.get("exclude_agreement") ?? "").toLowerCase(),
    );
    let docs: typeof mockDocuments;
    switch (filter) {
      case "recent": {
        const lastAccessedAt = (docId: string) => {
          const linkDates = mockLinks
            .filter((l) => l.documentId === docId && l.lastViewedAt)
            .map((l) => new Date(l.lastViewedAt!).getTime());
          return Math.max(...linkDates, 0);
        };
        docs = [...mockDocuments].sort(
          (a, b) => lastAccessedAt(b.id) - lastAccessedAt(a.id) || new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
        );
        break;
      }
      case "popular": {
        const totalViews = (docId: string) =>
          mockLinks.filter((l) => l.documentId === docId).reduce((sum, l) => sum + l.accessCount, 0);
        docs = [...mockDocuments]
          .filter((d) => d.status !== "archived" && totalViews(d.id) >= 30)
          .sort(
            (a, b) =>
              totalViews(b.id) - totalViews(a.id) || new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
          );
        break;
      }
      case "unshared":
        docs = mockDocuments.filter((d) => !mockLinks.some((l) => l.documentId === d.id && l.isActive));
        break;
      case "shared":
        docs = mockDocuments.filter(
          (d) => d.status !== "archived" && mockLinks.some((l) => l.documentId === d.id && l.isActive),
        );
        break;
      case "archived":
        docs = mockDocuments.filter((d) => d.status === "archived");
        break;
      default:
        docs = mockDocuments;
    }
    if (category) {
      docs = docs.filter((d) => (d.category ?? "general") === category);
    } else if (excludeAgreement) {
      docs = docs.filter((d) => (d.category ?? "general") !== "agreement");
    }
    if (excludeDealRoom) {
      const inRoom = new Set(
        mockDealRooms.flatMap((room) =>
          (room.documents ?? []).flatMap((fd) => fd.documents.map((d) => d.document_id || d.id)),
        ),
      );
      docs = docs.filter((d) => !inRoom.has(d.id));
    }
    return HttpResponse.json({ data: docs });
  }),

  http.get("*/api/workspaces/:workspaceSlug/documents/:id", ({ params }) => {
    const doc = mockDocuments.find((d) => d.id === params.id);
    if (!doc) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(doc);
  }),

  http.patch("*/api/workspaces/:workspaceSlug/documents/:id/category", async ({ params, request }) => {
    const doc = mockDocuments.find((d) => d.id === params.id);
    if (!doc) return new HttpResponse(null, { status: 404 });
    const body = (await request.json()) as { category?: string };
    const category = (body.category ?? "").toLowerCase();
    if (category !== "general" && category !== "agreement") {
      return HttpResponse.json({ code: "invalid_input", message: "invalid category" }, { status: 400 });
    }
    if (category === "agreement" && doc.fileType !== "pdf" && doc.sourceType !== "pdf") {
      return HttpResponse.json(
        { code: "agreement_pdf_required", message: "agreement documents must be PDF" },
        { status: 415 },
      );
    }
    doc.category = category;
    doc.updatedAt = new Date().toISOString();
    return HttpResponse.json(doc);
  }),

  http.get("*/api/workspaces/:workspaceSlug/documents/:id/delete-impact", ({ params }) => {
    const docId = String(params.id);
    if (!mockDocuments.some((d) => d.id === docId)) {
      return new HttpResponse(null, { status: 404 });
    }
    const activeLinkCount = mockLinks.filter((l) => l.documentId === docId && l.isActive).length;
    return HttpResponse.json({
      active_link_count: activeLinkCount,
      deal_room_count: 0,
    });
  }),

  http.delete("*/api/workspaces/:workspaceSlug/documents/:id", ({ params }) => {
    const index = mockDocuments.findIndex((d) => d.id === params.id);
    if (index === -1) return new HttpResponse(null, { status: 404 });
    const docId = String(params.id);
    mockDocuments.splice(index, 1);
    // Mirror API: revoking document share links on library delete.
    for (let i = mockLinks.length - 1; i >= 0; i--) {
      if (mockLinks[i]?.documentId === docId) {
        mockLinks.splice(i, 1);
      }
    }
    return new HttpResponse(null, { status: 204 });
  }),

  http.post("*/api/workspaces/:workspaceSlug/documents/:id/archive", ({ params }) => {
    const doc = mockDocuments.find((d) => d.id === params.id);
    if (!doc) return new HttpResponse(null, { status: 404 });
    doc.status = "archived";
    return HttpResponse.json(doc);
  }),

  http.post("*/api/workspaces/:workspaceSlug/documents/:id/unarchive", ({ params }) => {
    const doc = mockDocuments.find((d) => d.id === params.id);
    if (!doc) return new HttpResponse(null, { status: 404 });
    doc.status = "ready";
    return HttpResponse.json(doc);
  }),

  http.post("*/api/workspaces/:workspaceSlug/documents", async ({ request }) => {
    const formData = await request.formData();
    const file = formData.get("file") as File | null;
    const category = String(formData.get("category") ?? "").toLowerCase();
    const replace = ["1", "true", "yes", "on"].includes(
      String(formData.get("replace") ?? "").toLowerCase(),
    );
    const title = file?.name ?? "uploaded.pdf";
    const ext = title.split(".").pop()?.toLowerCase() ?? "pdf";
    if (category === "agreement" && ext !== "pdf") {
      return HttpResponse.json(
        {
          code: "agreement_pdf_required",
          message: "agreement documents must be PDF",
        },
        { status: 415 },
      );
    }
    const existing = mockDocuments.find((d) => d.title === title);
    if (existing && !replace) {
      return HttpResponse.json(
        {
          code: "document_exists",
          message: "a document with this filename already exists",
          document: { id: existing.id, title: existing.title },
        },
        { status: 409 },
      );
    }
    const fileType = (["pdf", "docx", "pptx", "xlsx"] as const).includes(ext as never) ? (ext as import("@/types").Document["fileType"]) : "pdf";
    if (existing && replace) {
      existing.fileSize = file?.size ?? existing.fileSize;
      existing.fileType = fileType;
      existing.sourceType = fileType;
      existing.fileName = title;
      existing.status = "ready";
      existing.category = category === "agreement" ? "agreement" : existing.category ?? "general";
      existing.updatedAt = new Date().toISOString();
      return HttpResponse.json(existing, { status: 201 });
    }
    const newDoc = {
      id: generateId("doc"),
      title,
      sourceType: fileType,
      fileName: title,
      fileType,
      fileSize: file?.size ?? 1_000_000,
      pageCount: 10,
      status: "ready" as const,
      category: (category === "agreement" ? "agreement" : "general") as import("@/types").DocumentCategory,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };
    mockDocuments.unshift(newDoc);
    return HttpResponse.json(newDoc, { status: 201 });
  }),

  http.get("*/api/workspaces/:workspaceSlug/documents/:id/pages", ({ params }) => {
    const doc = mockDocuments.find((d) => d.id === params.id);
    if (!doc) return new HttpResponse(null, { status: 404 });
    const pages = Array.from({ length: doc.pageCount }, (_, i) => ({
      page_number: i + 1,
      width: 800,
      height: 1000,
    }));
    return HttpResponse.json({ document_id: doc.id, pages, total: pages.length });
  }),

  http.post("*/api/workspaces/:workspaceSlug/documents/:id/pages/signed-url", async ({ params, request }) => {
    const doc = mockDocuments.find((d) => d.id === params.id);
    if (!doc) return new HttpResponse(null, { status: 404 });
    const body = (await request.json()) as { page_number?: number };
    const pageNumber = body.page_number ?? 1;
    return HttpResponse.json({
      page_number: pageNumber,
      image_url: placeholdImageUrl(800, 1000),
      expires_at: new Date(Date.now() + 5 * 60 * 1000).toISOString(),
      width: 800,
      height: 1000,
    });
  }),

  http.get("*/api/workspaces/:workspaceSlug/documents/:id/download-url", ({ params }) => {
    const doc = mockDocuments.find((d) => d.id === params.id);
    if (!doc) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({
      download_url: placeholdImageUrl(200, 200),
      expires_at: new Date(Date.now() + 5 * 60 * 1000).toISOString(),
      filename: doc.fileName,
      content_type: "application/pdf",
    });
  }),

  // Viewer events
  http.post("*/api/workspaces/:workspaceSlug/events", async () => {
    return new HttpResponse(null, { status: 204 });
  }),

  // Links
  http.get("*/api/workspaces/:workspaceSlug/links", ({ request }) => {
    const url = new URL(request.url);
    const documentId = url.searchParams.get("documentId");
    const data = documentId ? mockLinks.filter((l) => l.documentId === documentId) : mockLinks;
    return HttpResponse.json({ data });
  }),

  http.get("*/api/workspaces/:workspaceSlug/links/:id", ({ params }) => {
    const link = mockLinks.find((l) => l.id === params.id);
    if (!link) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(link);
  }),

  http.put("*/api/workspaces/:workspaceSlug/links/:id", async ({ request, params }) => {
    const link = mockLinks.find((l) => l.id === params.id);
    if (!link) return new HttpResponse(null, { status: 404 });
    const payload = (await request.json()) as {
      document_ids?: string[];
      folder_paths?: string[];
      folder_scope_mode?: "full" | "allowlist";
      name?: string;
      permission_type?: string;
      require_email_verification?: boolean;
      require_password?: boolean;
      require_nda?: boolean;
      allowed_emails?: string[];
      password?: string;
      contact_ids?: string[];
      expires_at?: string;
      max_access_count?: number;
      download_enabled?: boolean;
      watermark_enabled?: boolean;
      qa_enabled?: boolean;
    };
    // Update the in-memory link to reflect the edited values so subsequent reads
    // (including tests) see the new state.
    if (payload.folder_paths !== undefined) {
      link.folderPaths = payload.folder_paths;
    }
    if (payload.folder_scope_mode === "full" || payload.folder_scope_mode === "allowlist") {
      link.folderScopeMode = payload.folder_scope_mode;
    } else if (payload.folder_paths !== undefined) {
      link.folderScopeMode = "allowlist";
    }
    if (payload.document_ids && payload.document_ids.length > 0) {
      link.documentIds = payload.document_ids;
      link.documentId = payload.document_ids[0];
      const selectedDocs = payload.document_ids
        .map((id) => mockDocuments.find((d) => d.id === id))
        .filter(Boolean) as typeof mockDocuments;
      link.documents = selectedDocs.map((d) => ({
        id: d.id,
        title: d.title,
        sourceType: d.sourceType,
        pageCount: d.pageCount,
        status: d.status,
        fileSize: d.fileSize,
      }));
      link.documentTitle = selectedDocs.map((d) => d.title).join(", ") || link.documentTitle;
      link.isBundle = payload.document_ids.length > 1;
    }
    if (payload.permission_type) link.permissionType = payload.permission_type as Link["permissionType"];
    if (typeof payload.require_email_verification === "boolean") link.requireEmailVerification = payload.require_email_verification;
    if (typeof payload.require_password === "boolean") link.requirePassword = payload.require_password;
    if (typeof payload.require_nda === "boolean") link.requireNda = payload.require_nda;
    if (payload.expires_at) link.expiresAt = payload.expires_at;
    if (typeof payload.max_access_count === "number") link.maxAccessCount = payload.max_access_count;
    if (typeof payload.download_enabled === "boolean") link.downloadEnabled = payload.download_enabled;
    if (typeof payload.watermark_enabled === "boolean") link.watermarkEnabled = payload.watermark_enabled;
    if (typeof payload.qa_enabled === "boolean") link.qaEnabled = payload.qa_enabled;
    if (payload.contact_ids) link.contactIds = payload.contact_ids;
    const nextAllows = resolveDocumentAllowEmails({
      contactIds: payload.contact_ids ?? link.contactIds,
      allowedEmails: payload.allowed_emails ?? link.allowedEmails,
    });
    link.allowedEmails = nextAllows;
    (link as MockLinkExt)._allowedEmails = nextAllows;

    return HttpResponse.json(link);
  }),

  http.patch("*/api/workspaces/:workspaceSlug/links/:id", async ({ request, params }) => {
    const link = mockLinks.find((l) => l.id === params.id);
    if (!link) return new HttpResponse(null, { status: 404 });
    const patch = (await request.json()) as Partial<typeof link>;
    Object.assign(link, patch);
    return HttpResponse.json(link);
  }),

  http.delete("*/api/workspaces/:workspaceSlug/links/:id", ({ params }) => {
    const index = mockLinks.findIndex((l) => l.id === params.id);
    if (index === -1) return new HttpResponse(null, { status: 404 });
    mockLinks.splice(index, 1);
    return new HttpResponse(null, { status: 204 });
  }),

  http.get("*/api/workspaces/:workspaceSlug/links/pending-access-requests", () => {
    const data = mockLinkAccessRequests
      .filter((r) => r.status === "pending")
      .map((r) => {
        const link = mockLinks.find((l) => l.id === r.link_id);
        return {
          ...r,
          link_name: link?.name ?? "",
          document_title: link?.documentTitle ?? "",
          short_url: link?.shortUrl ?? "",
        };
      });
    return HttpResponse.json({ data });
  }),

  http.get("*/api/workspaces/:workspaceSlug/links/:id/access-requests", ({ params }) => {
    const linkId = params.id as string;
    if (linkId === "pending-access-requests") {
      return new HttpResponse(null, { status: 404 });
    }
    const data = mockLinkAccessRequests.filter((r) => r.link_id === linkId);
    return HttpResponse.json({ data });
  }),

  http.get("*/api/workspaces/:workspaceSlug/links/:id/access-rules", ({ params }) => {
    const link = mockLinks.find((l) => l.id === params.id) as MockLinkExt | undefined;
    if (!link) return new HttpResponse(null, { status: 404 });
    const allows = resolveDocumentAllowEmails({
      contactIds: link.contactIds,
      allowedEmails: link._allowedEmails ?? link.allowedEmails,
    });
    return HttpResponse.json({
      data: allows.map((value) => ({
        ruleType: "email" as const,
        value,
        action: "allow" as const,
      })),
    });
  }),

  http.put("*/api/workspaces/:workspaceSlug/links/:id/access-rules", async ({ params }) => {
    const link = mockLinks.find((l) => l.id === params.id);
    if (!link) return new HttpResponse(null, { status: 404 });
    return new HttpResponse(null, { status: 204 });
  }),

  http.post("*/api/workspaces/:workspaceSlug/links/:id/access-rules", async ({ params }) => {
    const link = mockLinks.find((l) => l.id === params.id);
    if (!link) return new HttpResponse(null, { status: 404 });
    return new HttpResponse(null, { status: 204 });
  }),

  http.post(
    "*/api/workspaces/:workspaceSlug/links/:id/access-requests/:requestId/approve",
    ({ params }) => {
      const req = mockLinkAccessRequests.find(
        (r) => r.id === params.requestId && r.link_id === params.id
      );
      if (!req) return new HttpResponse(null, { status: 404 });
      req.status = "approved";
      req.updated_at = new Date().toISOString();
      const existing = mockContacts.find(
        (c) => c.email.toLowerCase() === req.email.toLowerCase()
      );
      if (existing) {
        if (req.signer_name && !existing.name) {
          existing.name = req.signer_name;
        }
      } else {
        mockContacts.unshift({
          id: generateId("contact"),
          email: req.email,
          name: req.signer_name ?? "",
          organization: "",
          role: "",
          heatLevel: "cold",
          score: 0,
          scoreHistory: [],
          totalVisits: 0,
          totalDurationSeconds: 0,
          viewedDocuments: [],
        });
      }
      return HttpResponse.json({ data: req });
    }
  ),

  http.post(
    "*/api/workspaces/:workspaceSlug/links/:id/access-requests/:requestId/reject",
    ({ params }) => {
      const req = mockLinkAccessRequests.find(
        (r) => r.id === params.requestId && r.link_id === params.id
      );
      if (!req) return new HttpResponse(null, { status: 404 });
      req.status = "rejected";
      req.updated_at = new Date().toISOString();
      return HttpResponse.json({ data: req });
    }
  ),

  http.get("*/api/workspaces/:workspaceSlug/links/:id/access-logs", ({ params, request }) => {
    const url = new URL(request.url);
    const limitParam = Number(url.searchParams.get("limit"));
    const offsetParam = Number(url.searchParams.get("offset"));
    const limit = Number.isFinite(limitParam) && limitParam > 0 ? limitParam : 200;
    const offset = Number.isFinite(offsetParam) && offsetParam > 0 ? offsetParam : 0;
    const all = mockAccessLogs.filter((l) => l.linkId === params.id);
    const data = all.slice(offset, offset + limit);
    return HttpResponse.json({
      data,
      has_more: offset + data.length < all.length,
    });
  }),

  http.get("*/api/workspaces/:workspaceSlug/links/:id/analytics/visitors", ({ params, request }) => {
    const url = new URL(request.url);
    const limitParam = Number(url.searchParams.get("limit"));
    const offsetParam = Number(url.searchParams.get("offset"));
    const limit = Number.isFinite(limitParam) && limitParam > 0 ? limitParam : 10;
    const offset = Number.isFinite(offsetParam) && offsetParam > 0 ? offsetParam : 0;

    const byVisitor = new Map<
      string,
      {
        visitor_id: string;
        visitor_email?: string;
        first_access_at: string;
        last_access_at: string;
        total_views: number;
      }
    >();
    for (const log of mockAccessLogs.filter((l) => l.linkId === params.id)) {
      const visitorId = log.visitorEmail || log.id;
      const prev = byVisitor.get(visitorId);
      if (!prev) {
        byVisitor.set(visitorId, {
          visitor_id: visitorId,
          visitor_email: log.visitorEmail || undefined,
          first_access_at: log.timestamp,
          last_access_at: log.timestamp,
          total_views: 1,
        });
        continue;
      }
      prev.total_views += 1;
      if (log.timestamp < prev.first_access_at) prev.first_access_at = log.timestamp;
      if (log.timestamp > prev.last_access_at) prev.last_access_at = log.timestamp;
    }

    const all = [...byVisitor.values()].sort((a, b) => {
      const byTime = b.last_access_at.localeCompare(a.last_access_at);
      if (byTime !== 0) return byTime;
      return a.visitor_id.localeCompare(b.visitor_id);
    });
    const data = all.slice(offset, offset + limit);
    return HttpResponse.json({
      data,
      has_more: offset + data.length < all.length,
    });
  }),

  http.get(
    "*/api/workspaces/:workspaceSlug/links/:id/analytics/access-code-contacts",
    ({ request }) => {
      const url = new URL(request.url);
      const limitParam = Number(url.searchParams.get("limit"));
      const offsetParam = Number(url.searchParams.get("offset"));
      const limit = Number.isFinite(limitParam) && limitParam > 0 ? limitParam : 10;
      const offset = Number.isFinite(offsetParam) && offsetParam > 0 ? offsetParam : 0;
      // MSW fixture: empty by default; e2e seeds come from real API.
      const all: unknown[] = [];
      const data = all.slice(offset, offset + limit);
      return HttpResponse.json({
        data,
        has_more: offset + data.length < all.length,
      });
    },
  ),

  http.post("*/api/workspaces/:workspaceSlug/links", async ({ request }) => {
    const body = (await request.json()) as {
      document_id?: string;
      document_ids?: string[];
      name?: string;
      permission_type?: string;
      require_email?: boolean;
      require_email_verification?: boolean;
      require_password?: boolean;
      require_nda?: boolean;
      allowed_emails?: string[];
      contact_ids?: string[];
      password?: string;
      expires_at?: string;
      max_access_count?: number;
      download_enabled?: boolean;
      watermark_enabled?: boolean;
    };
    const primaryDocId = body.document_ids?.[0] ?? body.document_id ?? "";
    const doc = mockDocuments.find((d) => d.id === primaryDocId);
    const allowEmails = resolveDocumentAllowEmails({
      contactIds: body.contact_ids,
      allowedEmails: body.allowed_emails,
    });

    const requirePassword = body.require_password || body.permission_type === "password" || !!body.password;
    const requireNDA = body.require_nda || body.permission_type === "nda";
    const hasWhitelist = allowEmails.length > 0;
    const requireEmailVerification =
      body.require_email_verification ||
      body.permission_type === "email_required" ||
      body.permission_type === "whitelist" ||
      hasWhitelist ||
      requireNDA ||
      Boolean(body.contact_ids?.length) ||
      false;

    let permissionType: "public" | "email" | "password" | "nda" = "public";
    if (requirePassword) permissionType = "password";
    else if (requireNDA) permissionType = "nda";
    // Modern email verification uses permission_type "public" + require_email_verification.
    // Only the legacy "email_required" permission type maps to "email".
    else if (body.permission_type === "email_required" || body.require_email) permissionType = "email";

    const newLink = {
      id: generateId("link"),
      documentId: primaryDocId,
      documentIds: body.document_ids?.length ? body.document_ids : primaryDocId ? [primaryDocId] : [],
      documentTitle: doc?.title ?? "Untitled",
      shortUrl: `https://invest.acme.capital/d/${generateId("sh")}`,
      accessCount: 0,
      heatLevel: "cold" as const,
      createdAt: new Date().toISOString(),
      expiresAt: body.expires_at,
      isActive: true,
      avgDurationSeconds: 0,
      permissionType,
      contactIds: body.contact_ids ?? [],
      allowedEmails: allowEmails,
      requireEmailVerification,
      requireNda: requireNDA,
      _requireEmailVerification: requireEmailVerification,
      _requirePassword: requirePassword,
      _requireNDA: requireNDA,
      _password: body.password,
      _allowedEmails: allowEmails,
    } as MockLinkExt;
    mockLinks.unshift(newLink);
    return HttpResponse.json(newLink, { status: 201 });
  }),

  // Contacts
  http.get("*/api/workspaces/:workspaceSlug/contacts", () => {
    return HttpResponse.json({ data: mockContacts });
  }),

  http.post("*/api/workspaces/:workspaceSlug/contacts", async ({ request }) => {
    const body = (await request.json()) as { email: string; name?: string };
    const newContact: Contact = {
      id: generateId("contact"),
      email: body.email,
      name: body.name ?? "",
      organization: "",
      role: "",
      heatLevel: "cold",
      score: 0,
      scoreHistory: [],
      totalVisits: 0,
      totalDurationSeconds: 0,
      viewedDocuments: [],
    };
    mockContacts.unshift(newContact);
    return HttpResponse.json(newContact, { status: 201 });
  }),

  http.get("*/api/workspaces/:workspaceSlug/contacts/:id", ({ params }) => {
    const contact = mockContacts.find((c) => c.id === params.id);
    if (!contact) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(contact);
  }),

  http.get("*/api/workspaces/:workspaceSlug/contacts/:id/activities", ({ params }) => {
    return HttpResponse.json({ data: mockActivities.filter((a) => a.contactId === params.id) });
  }),

  // Deal rooms
  http.get("*/api/workspaces/:workspaceSlug/deal-rooms", () => {
    return HttpResponse.json({ data: mockDealRooms });
  }),

  http.get("*/api/workspaces/:workspaceSlug/deal-rooms/:id", ({ params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(room);
  }),

  http.get("*/api/workspaces/:workspaceSlug/deal-rooms/:id/analytics", ({ params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const roomLinks = mockLinks.filter(
      (l) =>
        l.dealRoomId === room.id &&
        l.status !== "deleted" &&
        l.status !== "disabled",
    );
    const activeLinkCount = roomLinks.filter((l) => {
      if (l.isActive === false) return false;
      return (
        l.status !== "revoked" &&
        l.status !== "expired" &&
        (l.status === "active" || l.status == null)
      );
    }).length;
    const totalViews = roomLinks.reduce((sum, l) => sum + (l.accessCount ?? 0), 0);
    const recentVisitors = (room.recentVisitors ?? []).map((v, idx) => ({
      visitorId: `visitor-${idx}-${v.email}`,
      visitorEmail: v.email,
      firstAccessAt: v.lastSeenAt,
      lastAccessAt: v.lastSeenAt,
      totalViews: 1,
    }));
    const uniqueVisitors = Math.max(room.visitorCount ?? 0, recentVisitors.length);
    const viewsOverTime =
      totalViews > 0
        ? [
            {
              day: new Date().toISOString().slice(0, 10),
              views: totalViews,
            },
          ]
        : [];
    return HttpResponse.json({
      totalViews: totalViews || room.viewCount || 0,
      uniqueVisitors,
      activeLinkCount: activeLinkCount || room.activeLinkCount || 0,
      documentCount: room.documentCount ?? 0,
      viewsOverTime,
      recentVisitors,
    });
  }),

  http.get("*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge", async ({ params }) => {
    await hydrateKnowledgeQAState();
    const room = findRoom(params.roomId as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const docs = (room.documents ?? []).flatMap((folder) => folder.documents ?? []).slice(0, 5);
    const override = mockKnowledgeCorpusOverrideByRoom.get(params.roomId as string);
    const status = override?.status ?? "ready";
    const documentStatus = override?.documentStatus ?? "synced";
    const jobStatus = override?.jobStatus ?? "done";
    const busy = status === "syncing" || status === "provisioning" || documentStatus === "syncing";
    return HttpResponse.json({
      enabled: true,
      status,
      lastSyncedAt: busy ? undefined : new Date().toISOString(),
      progress: {
        total: docs.length,
        pending: busy ? docs.length : 0,
        syncing: busy ? docs.length : 0,
        synced: busy ? 0 : docs.length,
        failed: 0,
        jobStatus,
      },
      documents: docs.map((d) => ({
        documentId: d.document_id || d.id,
        title: d.title || "Document",
        status: documentStatus,
        chunkCount: busy ? 0 : 3,
      })),
      quota: {
        planCode: "partner",
        knowledgeBases: { used: 1, limit: 100 },
        documents: { used: busy ? 0 : docs.length, limit: 5000 },
        answers: { used: 0, limit: 10_000 },
      },
    });
  }),

  http.post("*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/sync", ({ params }) => {
    const room = findRoom(params.roomId as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({ status: "queued" }, { status: 202 });
  }),

  http.post(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/query",
    async ({ params, request }) => {
      const room = findRoom(params.roomId as string);
      if (!room) return new HttpResponse(null, { status: 404 });
      const body = (await request.json()) as { query?: string; answer?: boolean; top_k?: number };
      const query = (body.query || "").trim() || "query";
      const doc = (room.documents ?? []).flatMap((folder) => folder.documents ?? [])[0];
      return HttpResponse.json({
        query,
        mode: "hybrid",
        answer: body.answer === false ? undefined : `Mock answer for: ${query}`,
        results: doc
          ? [
              {
                chunkId: "chunk-1",
                documentId: doc.document_id || doc.id,
                text: `Relevant passage from ${doc.title || "document"}.`,
                score: 0.91,
                sourceName: doc.title || "Document",
                pages: [1, 2],
                viewerPage: 1,
              },
            ]
          : [],
      });
    },
  ),

  http.get(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/sessions/active",
    async ({ params }) => {
      const roomId = params.roomId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      const sessions = await roomSessions(roomId);
      const active = sessions
        .filter((s) => s.status === "active")
        .sort((a, b) => (b.lastTurnAt || b.createdAt).localeCompare(a.lastTurnAt || a.createdAt))[0];
      if (!active) {
        return HttpResponse.json({ session: null, turns: [] });
      }
      return HttpResponse.json({
        session: active,
        turns: mockKnowledgeTurnsBySession.get(active.id) ?? [],
      });
    },
  ),

  http.get(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/sessions",
    async ({ params, request }) => {
      const roomId = params.roomId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      const url = new URL(request.url);
      const limit = Math.min(50, Math.max(1, Number(url.searchParams.get("limit") || 20)));
      const sessions = await roomSessions(roomId);
      const items = sessions
        .slice()
        .sort((a, b) => (b.lastTurnAt || b.createdAt).localeCompare(a.lastTurnAt || a.createdAt))
        .slice(0, limit)
        .map((s) => {
          const turns = mockKnowledgeTurnsBySession.get(s.id) ?? [];
          return {
            ...s,
            turnCount: turns.length,
            questionPreview: turns[0]?.question || s.title,
          };
        });
      return HttpResponse.json({ items });
    },
  ),

  http.post(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/sessions",
    async ({ params, request }) => {
      const roomId = params.roomId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      let title = "";
      try {
        const body = (await request.json()) as { title?: string };
        title = (body.title || "").trim();
      } catch {
        /* empty body ok */
      }
      const now = new Date().toISOString();
      const sessions = await roomSessions(roomId);
      for (const s of sessions) {
        if (s.status === "active") {
          s.status = "closed";
          s.updatedAt = now;
        }
      }
      const session: MockKnowledgeSession = {
        id: generateId("kqa_sess"),
        roomId,
        title,
        status: "active",
        createdAt: now,
        updatedAt: now,
        turnCount: 0,
      };
      sessions.unshift(session);
      mockKnowledgeTurnsBySession.set(session.id, []);
      await persistKnowledgeQAState();
      return HttpResponse.json(session, { status: 201 });
    },
  ),

  http.post(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/sessions/query",
    async ({ params, request }) => {
      const roomId = params.roomId as string;
      const body = (await request.json()) as {
        sessionId?: string;
        query?: string;
        answer?: boolean;
        top_k?: number;
      };
      const result = await executeMockKnowledgeSessionQuery(roomId, body);
      if (!result.ok) {
        return HttpResponse.json(result.body, { status: result.status });
      }
      return HttpResponse.json(result.payload);
    },
  ),

  http.post(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/sessions/query/stream",
    async ({ params, request }) => {
      const roomId = params.roomId as string;
      const body = (await request.json()) as {
        sessionId?: string;
        query?: string;
        answer?: boolean;
        top_k?: number;
      };
      const result = await executeMockKnowledgeSessionQuery(roomId, body);
      if (!result.ok) {
        return HttpResponse.json(result.body, { status: result.status });
      }
      const payload = result.payload;
      const turn = payload.turn as MockKnowledgeTurn;
      const hits = (payload.results as MockKnowledgeTurn["hits"]) ?? [];
      const grounded = !turn.refused && hits.length > 0;
      const answer = typeof turn.answer === "string" ? turn.answer : "";
      const tokenFrames = chunkMockAnswerTokens(answer, 36)
        .map((text) => `event: token\ndata: ${JSON.stringify({ text })}\n\n`)
        .join("");
      const generatingPhase: Record<string, unknown> = { phase: "generating" };
      if (turn.rewriteApplied) {
        generatingPhase.rewriteApplied = true;
        if (turn.retrieveQuery) generatingPhase.retrieveQuery = turn.retrieveQuery;
      }
      const frames = [
        `event: phase\ndata: ${JSON.stringify({ phase: "retrieving" })}\n\n`,
        answer ? `event: phase\ndata: ${JSON.stringify(generatingPhase)}\n\n` : "",
        grounded
          ? `event: sources\ndata: ${JSON.stringify({ results: hits, grounded: true })}\n\n`
          : "",
        tokenFrames,
        `event: done\ndata: ${JSON.stringify(payload)}\n\n`,
      ].join("");
      return new HttpResponse(frames, {
        status: 200,
        headers: {
          "Content-Type": "text/event-stream",
          "Cache-Control": "no-cache",
        },
      });
    },
  ),

  http.get(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/sessions/:sessionId",
    async ({ params }) => {
      const roomId = params.roomId as string;
      const sessionId = params.sessionId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      const session = (await roomSessions(roomId)).find((s) => s.id === sessionId);
      if (!session) return new HttpResponse(null, { status: 404 });
      return HttpResponse.json({
        session,
        turns: mockKnowledgeTurnsBySession.get(session.id) ?? [],
      });
    },
  ),

  http.post(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/sessions/:sessionId/close",
    async ({ params }) => {
      const roomId = params.roomId as string;
      const sessionId = params.sessionId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      const now = new Date().toISOString();
      let target: MockKnowledgeSession | undefined;
      for (const s of await roomSessions(roomId)) {
        if (s.status === "active") {
          s.status = "closed";
          s.updatedAt = now;
        }
        if (s.id === sessionId) target = s;
      }
      if (!target) return new HttpResponse(null, { status: 404 });
      await persistKnowledgeQAState();
      return HttpResponse.json(target);
    },
  ),

  http.post(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/events",
    async ({ params, request }) => {
      const roomId = params.roomId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      let body: { type?: string; turnOutcome?: string } = {};
      try {
        body = (await request.json()) as typeof body;
      } catch {
        /* empty */
      }
      const typ = (body.type || "").trim();
      if (typ !== "cite_open" && typ !== "followups_upgrade_failed") {
        return HttpResponse.json(
          { code: "invalid_input", message: "unknown desk event type" },
          { status: 400 },
        );
      }
      return new HttpResponse(null, { status: 204 });
    },
  ),

  http.get(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/missions",
    async ({ params }) => {
      const roomId = params.roomId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      return HttpResponse.json({
        items: mockKnowledgeMissionCatalog.map((p) => ({
          packId: p.packId,
          title: p.title,
          source: "catalog",
          items: p.items,
        })),
      });
    },
  ),

  http.get(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/mission/progress",
    async ({ params }) => {
      const roomId = params.roomId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      const bound = mockKnowledgeMissionByRoom.get(roomId);
      const pack = mockMissionPack(bound || "financing_dd_v1");
      return HttpResponse.json({
        packId: pack.packId,
        title: pack.title,
        source: bound ? "room" : "template_default",
        covered: 0,
        total: pack.items.length,
        items: pack.items.map((item) => ({ ...item, covered: false })),
      });
    },
  ),

  http.get(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/mission",
    async ({ params }) => {
      const roomId = params.roomId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      const bound = mockKnowledgeMissionByRoom.get(roomId);
      const pack = mockMissionPack(bound || "financing_dd_v1");
      return HttpResponse.json({
        packId: pack.packId,
        title: pack.title,
        source: bound ? "room" : "template_default",
        items: pack.items,
      });
    },
  ),

  http.put(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/mission",
    async ({ params, request }) => {
      const roomId = params.roomId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      let body: { packId?: string } = {};
      try {
        body = (await request.json()) as typeof body;
      } catch {
        /* empty */
      }
      const packId = (body.packId || "").trim();
      const pack = mockKnowledgeMissionCatalog.find((p) => p.packId === packId);
      if (!pack) {
        return HttpResponse.json(
          { code: "invalid_input", message: "unknown mission pack" },
          { status: 400 },
        );
      }
      mockKnowledgeMissionByRoom.set(roomId, packId);
      return HttpResponse.json({
        packId: pack.packId,
        title: pack.title,
        source: "room",
        items: pack.items,
      });
    },
  ),

  http.get(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/sessions/:sessionId/export",
    async ({ params }) => {
      const roomId = params.roomId as string;
      const sessionId = params.sessionId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      const sess = (mockKnowledgeSessionsByRoom.get(roomId) ?? []).find((s) => s.id === sessionId);
      if (!sess) {
        return HttpResponse.json({ code: "not_found", message: "session not found" }, { status: 404 });
      }
      const turns = mockKnowledgeTurnsBySession.get(sessionId) ?? [];
      return HttpResponse.json({
        schemaVersion: "knowledge_qa_diligence_v1",
        exportedAt: new Date().toISOString(),
        workspaceId: "ws-mock",
        roomId,
        sessionId,
        corpusFingerprint: "mockfp0123456789abcdef",
        session: sess,
        turns,
      });
    },
  ),

  http.get(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/archives",
    async ({ params, request }) => {
      const roomId = params.roomId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      await hydrateKnowledgeQAState();
      if (mockKnowledgeArchivesByRoom.size === 0) seedDefaultKnowledgeArchives();
      const url = new URL(request.url);
      const limit = Math.min(Number(url.searchParams.get("limit") || 20), 50);
      const items = (mockKnowledgeArchivesByRoom.get(roomId) ?? [])
        .slice(0, limit)
        .map(({ pack: _pack, ...tombstone }) => tombstone);
      return HttpResponse.json({ items });
    },
  ),

  http.get(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/archives/:archiveId",
    async ({ params }) => {
      const roomId = params.roomId as string;
      const archiveId = params.archiveId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      await hydrateKnowledgeQAState();
      if (mockKnowledgeArchivesByRoom.size === 0) seedDefaultKnowledgeArchives();
      const list = mockKnowledgeArchivesByRoom.get(roomId) ?? [];
      const row = list.find((a) => a.id === archiveId);
      if (!row) {
        return HttpResponse.json(
          { code: "not_found", message: "archive not found" },
          { status: 404 },
        );
      }
      row.status = "restored_readonly";
      const { pack, ...archive } = row;
      return HttpResponse.json({ archive, pack });
    },
  ),

  http.get(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/ops",
    async ({ params }) => {
      const roomId = params.roomId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      return HttpResponse.json({
        scope: "workspace",
        windowHours: 24,
        turnsTotal: 0,
        turnsByStatus: {},
        avgDurationMs: 0,
        p95DurationMs: 0,
        costUnitsTotal: 0,
        refusalsByKind: {},
        judgmentsByKind: {},
        evalCandidatesByStatus: {
          pending: (mockKnowledgeEvalCandidatesByRoom.get(roomId) ?? []).filter(
            (c) => c.reviewStatus === "pending",
          ).length,
          accepted: (mockKnowledgeEvalCandidatesByRoom.get(roomId) ?? []).filter(
            (c) => c.reviewStatus === "accepted",
          ).length,
        },
        pendingEvalCandidates: (mockKnowledgeEvalCandidatesByRoom.get(roomId) ?? []).filter(
          (c) => c.reviewStatus === "pending",
        ).length,
        answersQuota: { used: 0, limit: 10000, windowHours: 24 },
        retentionDays: 90,
        coldArchiveCount: (mockKnowledgeArchivesByRoom.get(roomId) ?? []).length,
        roomCorpusFingerprint: "mockfp0123456789abcdef",
        prometheusHints: [
          "dealsignal_knowledge_qa_turn_duration_seconds",
          "dealsignal_knowledge_qa_turns_total",
          "dealsignal_knowledge_qa_eval_candidates_total",
        ],
      });
    },
  ),

  http.get(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/eval/candidates/export",
    async ({ params, request }) => {
      const roomId = params.roomId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      const url = new URL(request.url);
      const limit = Math.min(Number(url.searchParams.get("limit") || 50), 200);
      const accepted = (mockKnowledgeEvalCandidatesByRoom.get(roomId) ?? [])
        .filter((c) => c.reviewStatus === "accepted")
        .slice(0, limit);
      return HttpResponse.json({
        description: "Accepted knowledge desk eval seeds (mock).",
        seeds: accepted.map((c) => ({
          id: `cand_${c.id}`,
          kind: c.feedbackKind,
          question: c.question,
          answer: c.answer,
          note: c.note,
          expect: c.expect || "reject_or_rebind",
        })),
      });
    },
  ),

  http.get(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/eval/candidates",
    async ({ params, request }) => {
      const roomId = params.roomId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      const url = new URL(request.url);
      const kind = (url.searchParams.get("kind") || "").trim();
      const status = (url.searchParams.get("status") || "").trim();
      let items = [...(mockKnowledgeEvalCandidatesByRoom.get(roomId) ?? [])];
      if (kind) items = items.filter((c) => c.feedbackKind === kind);
      if (status) items = items.filter((c) => c.reviewStatus === status);
      return HttpResponse.json({ items });
    },
  ),

  http.patch(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/eval/candidates/:candidateId",
    async ({ params, request }) => {
      const roomId = params.roomId as string;
      const candidateId = params.candidateId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      const body = (await request.json()) as { reviewStatus?: string; expect?: string };
      const status = (body.reviewStatus || "").trim();
      if (status !== "accepted" && status !== "rejected") {
        return HttpResponse.json(
          { code: "invalid_input", message: "invalid reviewStatus" },
          { status: 400 },
        );
      }
      const list = mockKnowledgeEvalCandidatesByRoom.get(roomId) ?? [];
      const row = list.find((c) => c.id === candidateId);
      if (!row) return new HttpResponse(null, { status: 404 });
      row.reviewStatus = status;
      row.expect =
        status === "accepted"
          ? (body.expect || "").trim() ||
            (row.feedbackKind === "not_answering" ? "refuse_or_ground" : "reject_or_rebind")
          : undefined;
      row.reviewedAt = new Date().toISOString();
      return HttpResponse.json(row);
    },
  ),

  http.post(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/turns/:turnId/follow-ups",
    async ({ params }) => {
      const roomId = params.roomId as string;
      const turnId = params.turnId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      await hydrateKnowledgeQAState();
      for (const session of await roomSessions(roomId)) {
        const turns = mockKnowledgeTurnsBySession.get(session.id) ?? [];
        const turn = turns.find((t) => t.id === turnId);
        if (!turn) continue;
        const coverage: string[] = [];
        const seen = new Set<string>();
        for (const h of turn.hits) {
          const n = (h.sourceName || "").trim();
          if (!n) continue;
          const key = n.toLowerCase();
          if (seen.has(key)) continue;
          seen.add(key);
          coverage.push(n);
          if (coverage.length >= 3) break;
        }
        const source = coverage[0] || "room document";
        if (turn.refused || turn.resultStatus === "no_hits" || turn.resultStatus === "error") {
          return HttpResponse.json({
            source: "template",
            items: [
              {
                id: "narrow-scope",
                text: "Try a more specific file name or clause title?",
              },
              {
                id: "name-clause",
                text: "Ask about a named clause in a room document?",
              },
            ],
          });
        }
        if (coverage.length >= 2) {
          const top2 = coverage[1]!;
          return HttpResponse.json({
            source: "llm",
            items: [
              {
                id: "llm-1",
                text: `What liability terms appear in “${source}”?`,
              },
              {
                id: "llm-2",
                text: `What exceptions does “${top2}” list?`,
              },
              {
                id: "llm-3",
                text: `Do “${source}” and “${top2}” agree on the same point?`,
              },
            ],
          });
        }
        return HttpResponse.json({
          source: "llm",
          items: [
            {
              id: "llm-1",
              text: `What liability terms appear in “${source}”?`,
            },
            {
              id: "llm-2",
              text: `How does “${source}” define the key obligations?`,
            },
            {
              id: "llm-3",
              text: `What exceptions does “${source}” list?`,
            },
          ],
        });
      }
      return new HttpResponse(null, { status: 404 });
    },
  ),

  http.put(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/turns/:turnId/feedback",
    async ({ params, request }) => {
      const roomId = params.roomId as string;
      const turnId = params.turnId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      const body = (await request.json()) as { kind?: string; note?: string };
      const kind = (body.kind || "").trim();
      if (!["helpful", "wrong_citation", "not_answering"].includes(kind)) {
        return HttpResponse.json(
          { code: "invalid_input", message: "invalid feedback kind" },
          { status: 400 },
        );
      }
      const note = (body.note || "").trim() || undefined;
      await hydrateKnowledgeQAState();
      for (const session of await roomSessions(roomId)) {
        const turns = mockKnowledgeTurnsBySession.get(session.id) ?? [];
        const turn = turns.find((t) => t.id === turnId);
        if (turn) {
          turn.feedback = { kind, note };
          if (kind === "wrong_citation" || kind === "not_answering") {
            const list = mockKnowledgeEvalCandidatesByRoom.get(roomId) ?? [];
            const existing = list.find(
              (c) => c.turnId === turnId && c.feedbackKind === kind,
            );
            const snap = {
              hits: (turn.hits || []).slice(0, 3).map((h) => ({
                chunkId: h.chunkId,
                sourceName: h.sourceName,
                excerpt: (h.text || "").slice(0, 280),
              })),
            };
            if (existing) {
              existing.question = turn.question;
              existing.answer = turn.answer;
              existing.note = note;
              existing.reviewStatus = "pending";
              existing.expect = undefined;
              existing.reviewedAt = undefined;
              existing.snapshot = snap;
            } else {
              list.unshift({
                id: crypto.randomUUID(),
                roomId,
                turnId,
                feedbackKind: kind,
                question: turn.question,
                answer: turn.answer,
                note,
                reviewStatus: "pending",
                createdAt: new Date().toISOString(),
                snapshot: snap,
              });
              mockKnowledgeEvalCandidatesByRoom.set(roomId, list);
            }
          }
          await persistKnowledgeQAState();
          return HttpResponse.json(turn.feedback);
        }
      }
      return new HttpResponse(null, { status: 404 });
    },
  ),

  http.get("*/api/workspaces/:workspaceSlug/deal-rooms/:id/links", ({ params, request }) => {
    const roomId = params.id as string;
    if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
    const url = new URL(request.url);
    const pageRaw = url.searchParams.get("page");
    let data = mockLinks.filter((l) => l.dealRoomId === roomId);
    const q = (url.searchParams.get("q") || "").trim().toLowerCase();
    if (q) {
      data = data.filter(
        (l) =>
          (l.name || "").toLowerCase().includes(q) ||
          l.shortUrl.toLowerCase().includes(q),
      );
    }
    const sortAsc = url.searchParams.get("sort") === "created_at_asc";
    data = [...data].sort((a, b) => {
      const delta = new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime();
      return sortAsc ? delta : -delta;
    });
    if (!pageRaw) {
      return HttpResponse.json({ data });
    }
    const page = Math.max(1, Number(pageRaw) || 1);
    const pageSize = Math.min(100, Math.max(1, Number(url.searchParams.get("page_size")) || 10));
    const total = data.length;
    const start = (page - 1) * pageSize;
    const slice = data.slice(start, start + pageSize);
    return HttpResponse.json({
      data: slice,
      pagination: {
        page,
        page_size: pageSize,
        total,
        has_more: start + pageSize < total,
      },
    });
  }),

  http.post("*/api/workspaces/:workspaceSlug/deal-rooms/:id/links", async ({ params, request }) => {
    const roomId = params.id as string;
    if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
    const body = (await request.json()) as {
      name?: string;
      qa_enabled?: boolean;
      folder_paths?: string[];
      document_ids?: string[];
      require_email?: boolean;
      require_email_verification?: boolean;
      require_password?: boolean;
      require_nda?: boolean;
      download_enabled?: boolean;
      watermark_enabled?: boolean;
    };
    const documentIds = body.document_ids?.length ? body.document_ids : ["doc_1"];
    const newLink: Link = {
      id: generateId("link"),
      name: body.name,
      documentId: documentIds[0],
      documentIds,
      folderPaths: body.folder_paths ?? [],
      documentTitle: "Deal room link",
      shortUrl: `https://invest.acme.capital/d/${generateId("sh")}`,
      accessCount: 0,
      heatLevel: "cold",
      createdAt: new Date().toISOString(),
      isActive: true,
      avgDurationSeconds: 0,
      permissionType: "public",
      isBundle: documentIds.length > 1,
      qaEnabled: !!body.qa_enabled,
      dealRoomId: roomId,
      requireEmail: !!body.require_email,
      requireEmailVerification: !!body.require_email_verification,
      requirePassword: !!body.require_password,
      requireNda: !!body.require_nda,
      downloadEnabled: body.download_enabled,
      watermarkEnabled: body.watermark_enabled,
      documents: [],
    };
    mockLinks.unshift(newLink);
    return HttpResponse.json(newLink, { status: 201 });
  }),

  http.get(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/visitor-questions",
    ({ params, request }) => {
      const roomId = params.roomId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      const filterLinkId = new URL(request.url).searchParams.get("link_id");
      const roomLinkIds = new Set(
        mockLinks.filter((l) => l.dealRoomId === roomId).map((l) => l.id),
      );
      const rows: VisitorQuestion[] = [];
      for (const [linkId, list] of mockOwnerQuestions) {
        if (!roomLinkIds.has(linkId)) continue;
        if (filterLinkId && linkId !== filterLinkId) continue;
        rows.push(...list);
      }
      rows.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
      return HttpResponse.json({ data: rows });
    },
  ),

  http.get("*/api/workspaces/:workspaceSlug/links/:id/questions", ({ params }) => {
    const linkId = params.id as string;
    const link = mockLinks.find((l) => l.id === linkId);
    if (!link) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({ data: mockOwnerQuestions.get(linkId) ?? [] });
  }),

  http.patch(
    "*/api/workspaces/:workspaceSlug/links/:id/questions/:questionId/answer",
    async ({ params, request }) => {
      const linkId = params.id as string;
      const questionId = params.questionId as string;
      const link = mockLinks.find((l) => l.id === linkId);
      if (!link) return new HttpResponse(null, { status: 404 });
      const body = (await request.json().catch(() => ({}))) as { answer?: string };
      const answer = (body.answer ?? "").trim();
      if (!answer) {
        return HttpResponse.json({ code: "invalid_input", message: "answer required" }, { status: 400 });
      }
      const list = mockOwnerQuestions.get(linkId) ?? [];
      const idx = list.findIndex((q) => q.id === questionId);
      if (idx < 0) {
        return HttpResponse.json({ code: "question_not_found", message: "question not found" }, { status: 404 });
      }
      const updated: VisitorQuestion = {
        ...list[idx],
        answer,
        status: "answered",
        answered_by: "user_1",
        updated_at: new Date().toISOString(),
      };
      list[idx] = updated;
      mockOwnerQuestions.set(linkId, list);
      return HttpResponse.json({ data: updated });
    },
  ),

  http.post("*/api/workspaces/:workspaceSlug/deal-rooms", async ({ request }) => {
    const body = (await request.json()) as {
      name: string;
      slug: string;
      description?: string;
      template_type?: string;
      requires_nda?: boolean;
      requires_approval?: boolean;
    };
    if (mockDealRooms.some((r) => r.slug === body.slug)) {
      return HttpResponse.json(
        { code: "duplicate_slug", message: "a deal room with this URL already exists" },
        { status: 409 }
      );
    }
    const scenario = body.template_type?.replace(/_/g, "-") ?? "custom";
    const template = mockDealRoomTemplates.find((t) => t.scenario === scenario);
    const folders: DealRoomFolder[] = template
      ? template.folderStructure.map((f, i) => ({
          path: `/${f.name.toLowerCase().replace(/[^a-z0-9]+/g, "-")}`,
          name: f.name,
          description: f.description,
          sort_order: i,
        }))
      : [{ path: "/general", name: "General", sort_order: 0 }];
    const newRoom: DealRoom = {
      id: generateId("dr"),
      name: body.name,
      description: body.description ?? "",
      slug: body.slug,
      template: (template?.scenario ?? scenario) as DealRoom["template"],
      ndaEnabled: body.requires_nda ?? false,
      requiresApproval: body.requires_approval ?? false,
      documentCount: 0,
      memberCount: 0,
      pendingApprovals: 0,
      createdAt: new Date().toISOString(),
      lastAccessedAt: undefined,
      status: "active",
      folders,
      documents: [],
      members: [],
      accessRequests: [],
    };
    mockDealRooms.unshift(newRoom);
    return HttpResponse.json(newRoom, { status: 201 });
  }),

  http.get("*/api/workspaces/:workspaceSlug/deal-room-templates", () => {
    return HttpResponse.json({ data: mockDealRoomTemplates });
  }),

  // Deal room folders
  http.get("*/api/workspaces/:workspaceSlug/deal-rooms/:id/folders", ({ params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({ data: getRoomFolders(room) });
  }),

  http.post("*/api/workspaces/:workspaceSlug/deal-rooms/:id/folders", async ({ request, params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const body = (await request.json()) as { name: string; parent_path?: string };
    const folders = getRoomFolders(room);
    const path = sanitizeFolderPath(body.name, body.parent_path ?? folders[0]?.path ?? "/general");
    if (folders.some((f) => f.path === path)) {
      return HttpResponse.json({ code: "folder_exists", message: "folder already exists" }, { status: 409 });
    }
    folders.push({
      path,
      name: body.name,
      sort_order: nextSortOrder(folders),
    });
    room.folders = folders;
    return HttpResponse.json({ data: folders }, { status: 201 });
  }),

  http.patch("*/api/workspaces/:workspaceSlug/deal-rooms/:id/folders/*path", async ({ request, params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const path = `/${params.path as string}`;
    const folders = getRoomFolders(room);
    const folder = folders.find((f) => f.path === path);
    if (!folder) return new HttpResponse(null, { status: 404 });
    const body = (await request.json()) as { name: string };
    const newPath = sanitizeFolderPath(body.name, "/");
    if (newPath !== path && folders.some((f) => f.path === newPath)) {
      return HttpResponse.json({ code: "folder_exists", message: "folder already exists" }, { status: 409 });
    }
    folder.name = body.name;
    folder.path = newPath;
    // Cascade update documents in this folder.
    const docs = getRoomFolderDocs(room);
    for (const fd of docs) {
      if (fd.folder === path) fd.folder = newPath;
      for (const doc of fd.documents) {
        if (doc.folder_path === path) doc.folder_path = newPath;
      }
    }
    room.folders = folders;
    return HttpResponse.json({ data: folders });
  }),

  http.delete("*/api/workspaces/:workspaceSlug/deal-rooms/:id/folders/*path", ({ params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const path = `/${params.path as string}`;
    const docs = getRoomFolderDocs(room);
    const hasDocs = docs.some((fd) => fd.folder === path && fd.documents.length > 0);
    if (hasDocs) {
      return HttpResponse.json({ code: "folder_not_empty", message: "folder is not empty" }, { status: 400 });
    }
    const folders = getRoomFolders(room).filter((f) => f.path !== path);
    room.folders = folders;
    return HttpResponse.json({ data: folders });
  }),

  http.post("*/api/workspaces/:workspaceSlug/deal-rooms/:id/resources/lock", async ({ request, params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const body = (await request.json()) as { folder_paths?: string[]; document_ids?: string[] };
    const folders = getRoomFolders(room).map((f) =>
      body.folder_paths?.includes(f.path) ? { ...f, locked: true } : f,
    );
    room.folders = folders;
    const docs = getRoomFolderDocs(room).map((fd) => ({
      ...fd,
      documents: fd.documents.map((d) =>
        body.document_ids?.includes(d.document_id) ? { ...d, locked: true } : d,
      ),
    }));
    room.documents = docs;
    return new HttpResponse(null, { status: 204 });
  }),

  http.post("*/api/workspaces/:workspaceSlug/deal-rooms/:id/resources/unlock", async ({ request, params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const body = (await request.json()) as { folder_paths?: string[]; document_ids?: string[] };
    const folders = getRoomFolders(room).map((f) =>
      body.folder_paths?.includes(f.path) ? { ...f, locked: false } : f,
    );
    room.folders = folders;
    const docs = getRoomFolderDocs(room).map((fd) => ({
      ...fd,
      documents: fd.documents.map((d) =>
        body.document_ids?.includes(d.document_id) ? { ...d, locked: false } : d,
      ),
    }));
    room.documents = docs;
    return new HttpResponse(null, { status: 204 });
  }),

  // Deal room documents
  http.get("*/api/workspaces/:workspaceSlug/deal-rooms/:id/documents", ({ params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({ data: getRoomFolderDocs(room) });
  }),

  // Visitor Ask high-risk security events (owner analytics)
  http.get("*/api/workspaces/:workspaceSlug/links/:id/ask-security-events", ({ params, request }) => {
    const linkId = params.id as string;
    const link = mockLinks.find((l) => l.id === linkId);
    if (!link) return new HttpResponse(null, { status: 404 });
    const url = new URL(request.url);
    const limit = Math.max(1, Number(url.searchParams.get("limit") || 20));
    const offset = Math.max(0, Number(url.searchParams.get("offset") || 0));
    const eventType = url.searchParams.get("event_type");
    const since = url.searchParams.get("since");
    const until = url.searchParams.get("until");
    const all = [
      {
        id: `ask-sec-${linkId}-1`,
        link_id: linkId,
        event_type: "rate_limit_exceeded",
        visitor_id: "visitor-ask-1",
        email: "visitor@example.com",
        reason: "ask_host",
        created_at: new Date().toISOString(),
      },
      {
        id: `ask-sec-${linkId}-2`,
        link_id: linkId,
        event_type: "not_in_allow_list",
        visitor_id: "visitor-ask-2",
        email: "removed@vc.com",
        created_at: new Date().toISOString(),
      },
    ].filter((ev) => {
      if (eventType && ev.event_type !== eventType) return false;
      const ts = Date.parse(ev.created_at);
      if (since && !(ts >= Date.parse(since))) return false;
      if (until && !(ts < Date.parse(until))) return false;
      return true;
    });
    const data = all.slice(offset, offset + limit);
    return HttpResponse.json({
      data,
      has_more: offset + data.length < all.length,
    });
  }),

  http.get("*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/ask-security-events", ({ params, request }) => {
    const roomId = params.roomId as string;
    const room = findRoom(roomId);
    if (!room) return new HttpResponse(null, { status: 404 });
    const url = new URL(request.url);
    const filterLinkId = url.searchParams.get("link_id");
    const limit = Math.max(1, Number(url.searchParams.get("limit") || 20));
    const offset = Math.max(0, Number(url.searchParams.get("offset") || 0));
    const eventType = url.searchParams.get("event_type");
    const since = url.searchParams.get("since");
    const until = url.searchParams.get("until");
    const roomLinks = mockLinks.filter((l) => l.dealRoomId === roomId);
    const source = roomLinks.length > 0
      ? roomLinks
      : [{ id: `${roomId}-synthetic-link` } as { id: string }];
    const all = source
      .filter((l) => !filterLinkId || l.id === filterLinkId)
      .flatMap((l, idx) => [
        {
          id: `ask-sec-room-${l.id}-rate`,
          link_id: l.id,
          event_type: "rate_limit_exceeded",
          visitor_id: `visitor-rate-${idx}`,
          email: `rate${idx}@example.com`,
          reason: "ask_host",
          created_at: new Date().toISOString(),
        },
        {
          id: `ask-sec-room-${l.id}-scope`,
          link_id: l.id,
          event_type: "scope_violation",
          visitor_id: `visitor-scope-${idx}`,
          email: `scope${idx}@example.com`,
          reason: "out_of_scope_evidence",
          created_at: new Date().toISOString(),
        },
        {
          id: `ask-sec-room-${l.id}-allow`,
          link_id: l.id,
          event_type: "not_in_allow_list",
          visitor_id: `visitor-allow-${idx}`,
          email: `removed${idx}@vc.com`,
          created_at: new Date().toISOString(),
        },
      ])
      .filter((ev) => {
        if (eventType && ev.event_type !== eventType) return false;
        const ts = Date.parse(ev.created_at);
        if (since && !(ts >= Date.parse(since))) return false;
        if (until && !(ts < Date.parse(until))) return false;
        return true;
      });
    const data = all.slice(offset, offset + limit);
    return HttpResponse.json({
      data,
      has_more: offset + data.length < all.length,
    });
  }),

  http.post("*/api/workspaces/:workspaceSlug/deal-rooms/:id/documents", async ({ request, params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const body = (await request.json()) as {
      document_id: string;
      folder_path?: string;
      sort_order?: number;
    };
    const doc = mockDocuments.find((d) => d.id === body.document_id);
    if (!doc) return new HttpResponse(null, { status: 404 });
    const folders = getRoomFolders(room);
    const folderPath = body.folder_path ?? folders[0]?.path ?? "/general";
    if (!folders.some((f) => f.path === folderPath)) {
      return HttpResponse.json({ code: "folder_not_found", message: "folder not found" }, { status: 404 });
    }
    const docs = getRoomFolderDocs(room);
    let fd = docs.find((d) => d.folder === folderPath);
    if (!fd) {
      fd = { folder: folderPath, permission: "view", documents: [] };
      docs.push(fd);
    }
    const item: DealRoomDocumentItem = {
      id: generateId("rd"),
      document_id: doc.id,
      title: doc.title,
      folder_path: folderPath,
      sort_order: body.sort_order ?? nextSortOrder(fd.documents),
      source_type: doc.sourceType,
      status: doc.status,
      page_count: doc.pageCount,
      file_size: doc.fileSize,
      created_at: doc.createdAt,
    };
    fd.documents.push(item);
    room.documents = docs;
    updateRoomDerivedFields(room);
    return HttpResponse.json(item, { status: 201 });
  }),

  http.patch("*/api/workspaces/:workspaceSlug/deal-rooms/:id/documents/:docId", async ({ request, params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const body = (await request.json()) as { folder_path?: string; sort_order?: number };
    const docs = getRoomFolderDocs(room);
    let item: DealRoomDocumentItem | undefined;
    let fromFd: DealRoomFolderDocs | undefined;
    for (const fd of docs) {
      const found = fd.documents.find((d) => d.id === params.docId);
      if (found) {
        item = found;
        fromFd = fd;
        break;
      }
    }
    if (!item || !fromFd) return new HttpResponse(null, { status: 404 });

    if (typeof body.sort_order === "number") {
      item.sort_order = body.sort_order;
      // Swap sort_order with adjacent document when moving up/down.
      const siblings = fromFd.documents.filter((d) => d.id !== item!.id).sort((a, b) => a.sort_order - b.sort_order);
      for (const sibling of siblings) {
        if (sibling.sort_order === item.sort_order) {
          sibling.sort_order = item.sort_order + (body.sort_order < sibling.sort_order ? 1 : -1);
        }
      }
    }

    if (body.folder_path !== undefined && body.folder_path !== item.folder_path) {
      fromFd.documents = fromFd.documents.filter((d) => d.id !== item!.id);
      if (fromFd.documents.length === 0) {
        room.documents = docs.filter((d) => d !== fromFd);
      }
      let toFd = docs.find((d) => d.folder === body.folder_path);
      if (!toFd) {
        toFd = { folder: body.folder_path, permission: "view", documents: [] };
        docs.push(toFd);
      }
      item.folder_path = body.folder_path;
      item.sort_order = nextSortOrder(toFd.documents);
      toFd.documents.push(item);
    }

    room.documents = docs;
    updateRoomDerivedFields(room);
    return HttpResponse.json(item);
  }),

  http.delete("*/api/workspaces/:workspaceSlug/deal-rooms/:id/documents/:docId", ({ params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const docs = getRoomFolderDocs(room);
    for (const fd of docs) {
      const idx = fd.documents.findIndex((d) => d.id === params.docId);
      if (idx !== -1) {
        fd.documents.splice(idx, 1);
        break;
      }
    }
    room.documents = docs.filter((fd) => fd.documents.length > 0);
    updateRoomDerivedFields(room);
    return new HttpResponse(null, { status: 204 });
  }),

  // Deal room members
  http.get("*/api/workspaces/:workspaceSlug/deal-rooms/:id/members", ({ params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({ data: room.members ?? [] });
  }),

  http.post("*/api/workspaces/:workspaceSlug/deal-rooms/:id/members", async ({ request, params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const body = (await request.json()) as { email: string; role: DealRoomMember["role"] };
    const members = room.members ?? [];
    const newMember: DealRoomMember = {
      id: generateId("rm"),
      email: body.email,
      role: body.role,
      nda_status: room.ndaEnabled ? "pending" : "none",
      status: "active",
    };
    members.push(newMember);
    room.members = members;
    updateRoomDerivedFields(room);
    return HttpResponse.json({ data: newMember }, { status: 201 });
  }),

  http.delete("*/api/workspaces/:workspaceSlug/deal-rooms/:id/members/:memberId", ({ params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const members = room.members ?? [];
    const index = members.findIndex((m) => m.id === params.memberId);
    if (index === -1) return new HttpResponse(null, { status: 404 });
    members.splice(index, 1);
    room.members = members;
    updateRoomDerivedFields(room);
    return new HttpResponse(null, { status: 204 });
  }),

  // Deal room access requests
  http.get("*/api/workspaces/:workspaceSlug/deal-rooms/:id/access-requests", ({ params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({ data: room.accessRequests ?? [] });
  }),

  http.post("*/api/workspaces/:workspaceSlug/deal-rooms/:id/access-requests/:requestId/approve", ({ params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const requests = room.accessRequests ?? [];
    const request = requests.find((r) => r.id === params.requestId);
    if (!request) return new HttpResponse(null, { status: 404 });
    request.status = "approved";
    request.reviewed_at = new Date().toISOString();
    // Promote to member if not already present.
    const members = room.members ?? [];
    if (!members.some((m) => m.email === request.email)) {
      members.push({
        id: generateId("rm"),
        email: request.email,
        role: "viewer",
        nda_status: room.ndaEnabled ? "pending" : "none",
        status: "active",
      });
      room.members = members;
    }
    updateRoomDerivedFields(room);
    return HttpResponse.json(request);
  }),

  http.post("*/api/workspaces/:workspaceSlug/deal-rooms/:id/access-requests/:requestId/reject", ({ params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const requests = room.accessRequests ?? [];
    const request = requests.find((r) => r.id === params.requestId);
    if (!request) return new HttpResponse(null, { status: 404 });
    request.status = "rejected";
    request.reviewed_at = new Date().toISOString();
    updateRoomDerivedFields(room);
    return HttpResponse.json(request);
  }),

  // Public deal room
  http.get("*/api/v1/public/deal-rooms/:slug", ({ request, params }) => {
    const url = new URL(request.url);
    const email = url.searchParams.get("email")?.toLowerCase();
    const slug = params.slug as string;
    const room = mockDealRooms.find((r) => r.slug === slug || r.id === slug);
    if (!room) return new HttpResponse(null, { status: 404 });

    const member = room.members?.find((m) => m.email.toLowerCase() === email) ?? null;
    const requests = room.accessRequests ?? [];
    const pendingRequest = email ? requests.find((r) => r.email.toLowerCase() === email && r.status === "pending") : undefined;

    // If email is not a member and room requires approval, show request form.
    // If member exists but status is not active, treat as pending.
    let effectiveMember = member;
    if (!member && pendingRequest) {
      effectiveMember = {
        id: pendingRequest.id,
        email: pendingRequest.email,
        role: "viewer",
        nda_status: "none",
        status: "pending",
      };
    }

    return HttpResponse.json({
      room: {
        id: room.id,
        name: room.name,
        description: room.description,
        ndaEnabled: room.ndaEnabled,
        requiresApproval: room.requiresApproval ?? false,
      },
      member: effectiveMember
        ? {
            id: effectiveMember.id,
            email: effectiveMember.email,
            role: effectiveMember.role,
            ndaStatus: effectiveMember.nda_status,
            status: effectiveMember.status,
          }
        : null,
      folders: getRoomFolders(room),
      documents: getRoomFolderDocs(room),
    });
  }),

  http.post("*/api/v1/public/deal-rooms/:slug/access-requests", async ({ request, params }) => {
    const slug = params.slug as string;
    const room = mockDealRooms.find((r) => r.slug === slug || r.id === slug);
    if (!room) return new HttpResponse(null, { status: 404 });
    const body = (await request.json()) as { email: string; reason?: string };
    const requests = room.accessRequests ?? [];
    if (!requests.some((r) => r.email.toLowerCase() === body.email.toLowerCase())) {
      requests.push({
        id: generateId("ra"),
        email: body.email,
        status: "pending",
        reason: body.reason,
      });
      room.accessRequests = requests;
      updateRoomDerivedFields(room);
    }
    return HttpResponse.json({ request_id: requests[requests.length - 1].id }, { status: 201 });
  }),

  http.post("*/api/v1/public/deal-rooms/:slug/nda", async ({ request, params }) => {
    const slug = params.slug as string;
    const room = mockDealRooms.find((r) => r.slug === slug || r.id === slug);
    if (!room) return new HttpResponse(null, { status: 404 });
    const body = (await request.json()) as { email: string };
    const members = room.members ?? [];
    const member = members.find((m) => m.email.toLowerCase() === body.email.toLowerCase());
    if (member) {
      member.nda_status = "signed";
      member.nda_signed_at = new Date().toISOString();
    }
    return new HttpResponse(null, { status: 204 });
  }),

  // Insights
  http.get("*/api/workspaces/:workspaceSlug/insights/overview", () => {
    const tierCounts = {
      hot: mockHeatAlerts.filter((a) => a.heatLevel === "hot").length + 2,
      warm: mockHeatAlerts.filter((a) => a.heatLevel === "warm").length + 1,
      cold: mockLinks.filter((l) => l.heatLevel === "cold").length,
    };
    const topDocuments = mockDocuments
      .map((d) => {
        const views = mockLinks
          .filter((l) => l.documentId === d.id)
          .reduce((sum, l) => sum + l.accessCount, 0);
        const heatLevel = views > 30 ? "hot" : views > 5 ? "warm" : "cold";
        return { id: d.id, title: d.title, views, heatLevel };
      })
      .sort((a, b) => b.views - a.views)
      .slice(0, 5);
    const topLinks = mockLinks
      .map((l) => ({
        id: l.id,
        shortUrl: l.shortUrl,
        views: l.accessCount,
        heatLevel: l.heatLevel,
      }))
      .sort((a, b) => b.views - a.views)
      .slice(0, 5);
    const topContacts = mockContacts
      .map((c) => ({
        id: c.id,
        email: c.email,
        score: c.score,
        heatLevel: c.heatLevel,
      }))
      .sort((a, b) => b.score - a.score)
      .slice(0, 5);
    return HttpResponse.json({ tierCounts, topDocuments, topLinks, topContacts });
  }),

  http.get("*/api/workspaces/:workspaceSlug/insights/pages/:documentId", ({ params }) => {
    return HttpResponse.json({ data: mockPageAnalytics[params.documentId as string] || [] });
  }),

  http.get("*/api/workspaces/:workspaceSlug/insights/suggestions", () => {
    return HttpResponse.json({ data: mockSuggestions });
  }),

  // Members
  http.get("*/api/workspaces/:workspaceSlug/members", () => {
    return HttpResponse.json({ data: mockWorkspaceMembers });
  }),

  http.post("*/api/workspaces/:workspaceSlug/invitations", async ({ request }) => {
    const body = (await request.json()) as { email: string; role: WorkspaceMember["role"] };
    const newMember = {
      id: generateId("wm"),
      userId: generateId("u"),
      email: body.email,
      name: body.email.split("@")[0],
      role: body.role,
      joinedAt: new Date().toISOString(),
      status: "pending" as const,
    };
    mockWorkspaceMembers.push(newMember);
    return HttpResponse.json({ data: newMember }, { status: 201 });
  }),

  // Workspace settings
  http.get("*/api/workspaces/:workspaceSlug/settings", () => {
    return HttpResponse.json({ data: workspaceSettings });
  }),

  http.put("*/api/workspaces/:workspaceSlug/settings", async ({ request }) => {
    const body = (await request.json()) as typeof workspaceSettings;
    workspaceSettings = { ...workspaceSettings, ...body };
    return HttpResponse.json({ data: workspaceSettings });
  }),

  http.post("*/api/workspaces/:workspaceSlug/logo", async () => {
    const mockLogoUrl = "https://placehold.co/128x128/0f172a/ffffff?text=Logo";
    workspaceSettings = { ...workspaceSettings, logoUrl: mockLogoUrl };
    return HttpResponse.json({ data: { logoUrl: mockLogoUrl } }, { status: 201 });
  }),

  http.get("*/api/workspaces/:workspaceSlug/billing", () => {
    const totalStorage = mockDocuments.reduce((sum, d) => sum + d.fileSize, 0);
    return HttpResponse.json({
      data: {
        plan: "Pro",
        period: "Annual",
        storageUsed: Math.round((totalStorage / 1024 / 1024) * 10) / 10,
        storageLimit: 50,
        linksUsed: mockLinks.length,
        linksLimit: 100,
        roomsUsed: mockDealRooms.length,
        roomsLimit: 10,
      },
    });
  }),

  // Integrations
  http.get("*/api/workspaces/:workspaceSlug/integrations/settings", () => {
    return HttpResponse.json({ data: integrationsStatus });
  }),

  http.put("*/api/workspaces/:workspaceSlug/integrations/settings", async ({ request }) => {
    const body = (await request.json()) as typeof integrationsStatus;
    integrationsStatus = { ...integrationsStatus, ...body };
    return HttpResponse.json({ data: integrationsStatus });
  }),

  http.post("*/api/workspaces/:workspaceSlug/integrations/slack/connect", () => {
    return HttpResponse.json({ url: "https://slack.com/oauth/v2/authorize?client_id=mock" });
  }),

  http.post("*/api/workspaces/:workspaceSlug/integrations/slack/disconnect", () => {
    integrationsStatus.slack = false;
    return HttpResponse.json({ code: "ok", message: "disconnected" });
  }),

  http.post("*/api/workspaces/:workspaceSlug/integrations/hubspot/connect", () => {
    return HttpResponse.json({ url: "https://app.hubspot.com/oauth/authorize?client_id=mock" });
  }),

  http.post("*/api/workspaces/:workspaceSlug/integrations/hubspot/disconnect", () => {
    integrationsStatus.hubspot = false;
    return HttpResponse.json({ code: "ok", message: "disconnected" });
  }),

  // Security
  http.get("*/api/workspaces/:workspaceSlug/security", () => {
    return HttpResponse.json({ data: securitySettings });
  }),

  http.put("*/api/workspaces/:workspaceSlug/security", async ({ request }) => {
    const body = (await request.json()) as typeof securitySettings;
    securitySettings = { ...securitySettings, ...body };
    return HttpResponse.json({ data: securitySettings });
  }),

  // Signals
  http.get("*/api/workspaces/:workspaceSlug/signals", () => {
    return HttpResponse.json(getMockSignalFeed());
  }),

  http.get("*/api/workspaces/:workspaceSlug/signals/:id", ({ params }) => {
    const signal = mockSignals.find((s) => s.id === params.id);
    if (!signal) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(signal);
  }),

  http.patch("*/api/workspaces/:workspaceSlug/signals/actions/:id", async ({ params, request }) => {
    const body = (await request.json()) as { status?: string };
    const action = mockActionItems.find((a) => a.id === params.id);
    if (!action) return new HttpResponse(null, { status: 404 });
    if (body?.status) action.status = body.status as ActionItem["status"];
    return HttpResponse.json(action);
  }),

  // Public viewer — pre-NDA allowlist check (parity with backend CheckPublicEmail).
  http.post("*/api/v1/public/links/:token/check-email", async ({ params, request }) => {
    const token = params.token as string;
    const body = (await request.json().catch(() => ({}))) as { email?: string };
    const email = normalizeMockEmail(body.email ?? "");
    if (!email || !email.includes("@")) {
      return HttpResponse.json({ code: "invalid_input", message: "email required" }, { status: 400 });
    }
    const link = findMockLinkByPublicToken(token) ?? (mockLinks[0] as MockLinkExt | undefined);
    if (!link) {
      return HttpResponse.json({ code: "link_not_found", message: "link not found" }, { status: 404 });
    }
    const requiresEmailVerification =
      Boolean(link._requireEmailVerification) ||
      Boolean(link.requireEmailVerification) ||
      link.permissionType === "email" ||
      link.permissionType === "nda";
    const requiresNda = Boolean(link._requireNDA) || Boolean(link.requireNda) || link.permissionType === "nda";
    const requiresPassword = Boolean(link._requirePassword) || Boolean(link.requirePassword) || link.permissionType === "password";
    const allowEmails = resolveDocumentAllowEmails({
      contactIds: link.contactIds,
      allowedEmails: link._allowedEmails ?? link.allowedEmails,
    });
    if (allowEmails.length > 0 && !allowEmails.includes(email)) {
      return HttpResponse.json(
        {
          code: "not_allowed",
          message: "email is not allowed",
          requiresEmail: false,
          requiresEmailVerification,
          requiresPassword,
          requiresNda,
          isDealRoom: Boolean(link.dealRoomId),
        },
        { status: 403 },
      );
    }
    return HttpResponse.json({ ok: true });
  }),

  // Public viewer
  http.post("*/api/v1/public/links/:token", async ({ params, request }) => {
    const body = (await request.json().catch(() => ({}))) as {
      email?: string;
      email_code?: string;
      password?: string;
      nda_agreed?: boolean;
    };
    const token = params.token as string;
    const extended =
      findMockLinkByPublicToken(token) ??
      (mockLinks[0] as MockLinkExt);

    // The mock permissionType "email" corresponds to the legacy "email_required" type,
    // where the visitor must supply both email and code. Modern email verification uses
    // permissionType "public" + _requireEmailVerification and is code-only.
    const isLegacyEmailRequired = extended.permissionType === "email";
    const requiresEmailVerification =
      extended._requireEmailVerification ||
      extended.requireEmailVerification ||
      isLegacyEmailRequired ||
      extended.permissionType === "nda";
    const requiresPassword =
      extended._requirePassword || extended.requirePassword || extended.permissionType === "password";
    const requiresNda =
      extended._requireNDA || extended.requireNda || extended.permissionType === "nda";
    const allowEmails = resolveDocumentAllowEmails({
      contactIds: extended.contactIds,
      allowedEmails: extended._allowedEmails ?? extended.allowedEmails,
    });
    const hasWhitelist = allowEmails.length > 0;
    // Email is required for legacy email_required, whitelist matching, or NDA records.
    // Modern email verification (code-only) should not ask for email.
    const requiresEmail = isLegacyEmailRequired || hasWhitelist || requiresNda;

    if (requiresEmail && !body.email) {
      return HttpResponse.json(
        { code: "requires_email", message: "email required", requiresEmail, requiresEmailVerification, requiresPassword, requiresNda },
        { status: 403 }
      );
    }
    if (requiresEmailVerification && !body.email_code) {
      return HttpResponse.json(
        { code: "requires_email_code", message: "email code required", requiresEmail, requiresEmailVerification, requiresPassword, requiresNda },
        { status: 403 }
      );
    }
    if (requiresEmailVerification && body.email_code !== "123456") {
      return HttpResponse.json(
        { code: "invalid_email_code", message: "invalid email code", requiresEmail, requiresEmailVerification, requiresPassword, requiresNda },
        { status: 401 }
      );
    }
    if (hasWhitelist) {
      const allowed = allowEmails.includes(normalizeMockEmail(body.email ?? ""));
      if (!allowed) {
        // Match backend Access/CheckPublicEmail: not_allowed (not legacy whitelist_denied).
        return HttpResponse.json(
          {
            code: "not_allowed",
            message: "email not in whitelist",
            requiresEmail,
            requiresEmailVerification,
            requiresPassword,
            requiresNda,
            isDealRoom: Boolean(extended.dealRoomId),
          },
          { status: 403 },
        );
      }
    }
    if (requiresPassword && !body.password) {
      return HttpResponse.json(
        { code: "requires_password", message: "password required", requiresEmail, requiresEmailVerification, requiresPassword, requiresNda },
        { status: 403 }
      );
    }
    if (requiresPassword && body.password !== extended._password) {
      return HttpResponse.json(
        { code: "invalid_password", message: "invalid password", requiresEmail, requiresEmailVerification, requiresPassword, requiresNda },
        { status: 401 }
      );
    }
    if (requiresNda && !body.nda_agreed) {
      return HttpResponse.json(
        { code: "nda_required", message: "nda agreement required", requiresEmail, requiresEmailVerification, requiresPassword, requiresNda },
        { status: 403 }
      );
    }

    const doc = mockDocuments.find((d) => d.id === link.documentId) ?? mockDocuments[0];
    const publicDocument = {
      id: doc.id,
      title: doc.title,
      pageCount: doc.pageCount,
      status: doc.status,
      sourceType: doc.fileType,
      fileSize: doc.fileSize,
    };
    return HttpResponse.json({
      link: {
        id: link.id,
        name: link.documentTitle,
        documentId: link.documentId,
        permissionType: link.permissionType ?? "public",
        downloadEnabled: true,
        watermarkEnabled: false,
        qaEnabled: Boolean(link.qaEnabled),
        fileRequestsEnabled: Boolean(link.fileRequestsEnabled),
        isBundle: Boolean(link.isBundle),
        dealRoomId: link.dealRoomId,
      },
      document: publicDocument,
      documents: [publicDocument],
      visitorId: generateId("visitor"),
      requiresEmail,
      requiresEmailVerification,
      requiresPassword,
      requiresNda,
      sessionToken: "mock_session_token",
    });
  }),

  http.get("*/api/v1/public/links/:token/questions/me", ({ params }) => {
    const token = params.token as string;
    return HttpResponse.json({ data: mockPublicQuestions.get(token) ?? [] });
  }),

  http.post("*/api/v1/public/links/:token/questions", async ({ params, request }) => {
    const token = params.token as string;
    const body = (await request.json().catch(() => ({}))) as { question?: string };
    const question = (body.question ?? "").trim();
    if (!question) {
      return HttpResponse.json({ code: "invalid_request", message: "question required" }, { status: 400 });
    }
    const lower = question.toLowerCase();
    if (lower.includes("__rate_limit__")) {
      return HttpResponse.json(
        { code: "rate_limit_exceeded", message: "too many Ask Host requests, please try again later" },
        { status: 429 },
      );
    }
    if (lower.includes("__limiter_down__")) {
      return HttpResponse.json(
        {
          code: "limiter_unavailable",
          message: "Ask Host is temporarily unavailable, please try again later",
        },
        { status: 503 },
      );
    }
    const row: VisitorQuestion = {
      id: generateId("q"),
      link_id: token,
      visitor_id: "visitor_mock",
      question,
      status: "pending",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };
    const list = mockPublicQuestions.get(token) ?? [];
    list.push(row);
    mockPublicQuestions.set(token, list);
    return HttpResponse.json({ data: row }, { status: 201 });
  }),

  http.get("*/api/v1/public/documents/:documentId/pages", ({ params }) => {
    const doc = mockDocuments.find((d) => d.id === params.documentId);
    if (!doc) return new HttpResponse(null, { status: 404 });
    const pages = Array.from({ length: doc.pageCount }, (_, i) => ({
      pageNumber: i + 1,
      width: 800,
      height: 1000,
    }));
    return HttpResponse.json({ documentId: doc.id, pages, total: pages.length });
  }),

  http.get("*/api/v1/public/documents/:documentId/pages/signed-url", ({ params, request }) => {
    const doc = mockDocuments.find((d) => d.id === params.documentId);
    if (!doc) return new HttpResponse(null, { status: 404 });
    const url = new URL(request.url);
    const pageNumber = Number(url.searchParams.get("page_number") ?? "1");
    return HttpResponse.json({
      pageNumber,
      imageUrl: placeholdImageUrl(800, 1000),
      expiresAt: new Date(Date.now() + 5 * 60 * 1000).toISOString(),
      width: 800,
      height: 1000,
    });
  }),

  http.get("*/api/v1/public/documents/:documentId/download-url", ({ params }) => {
    const doc = mockDocuments.find((d) => d.id === params.documentId);
    if (!doc) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({
      downloadUrl: placeholdImageUrl(200, 200),
      expiresAt: new Date(Date.now() + 5 * 60 * 1000).toISOString(),
      filename: doc.fileName,
      contentType: "application/pdf",
    });
  }),

  http.post("*/api/v1/public/events", async () => {
    return new HttpResponse(null, { status: 204 });
  }),
];
