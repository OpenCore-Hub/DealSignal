import { http, HttpResponse } from "msw";
import type {
  ActionItem,
  Circle,
  Contact,
  DealRoom,
  DealRoomAccessPolicy,
  DealRoomDocumentItem,
  DealRoomFolder,
  DealRoomFolderDocs,
  DealRoomMember,
  Link,
  OwnerAskTurn,
  PublicAskFAQ,
  PublicFormalAsk,
  PublicAskTurn,
  WorkspaceMember,
} from "@/types";
import { qaEnabledForLinkId } from "@/lib/qaScope";
import { keyPageRulesForCircle, keywordLangFromI18n } from "@/lib/heat/heatScore";

/** In-memory workspace key-page settings for MSW (additive extras + default circle). */
let mockKeyPageSettings = {
  defaultCircle: "founder" as "founder" | "investor_ir" | "sales",
  extraKeywords: {} as Record<string, string[]>,
};

function keyPageLangFromRequest(request: Request) {
  return keywordLangFromI18n(request.headers.get("Accept-Language") ?? undefined);
}
import { attachOwnerAskRepeatCounts, matchesOwnerAskInboxFilter, ownerAskTurnCanPinFAQ } from "@/lib/ownerAskInbox";
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
  mockNdaTemplates,
  mockDocumentVisitors,
  mockPageAnalytics,
  mockSignals,
  mockSuggestions,
  mockWorkspaceMembers,
  mockWorkspaces,
  defaultWorkspaceSettings,
  getMockDashboardStats,
  getMockRadarEvidence,
  getMockRadarFeed,
  getMockSignalFeed,
} from "./data";
import { computeMockLinkHeat } from "./mockHeat";

let workspaceSettings = { ...defaultWorkspaceSettings };

let integrationsStatus = {
  email_enabled: true,
  daily_digest_enabled: false,
  key_page_slack_enabled: false,
  slack_connected: false,
  hubspot_connected: false,
};

let outboundWebhook = {
  configured: false,
  enabled: false,
  url: "",
  event_types: ["key_page", "repeat_key_page"] as string[],
  secret_hint: "",
  secret: "",
  updated_at: "",
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

/**
 * Workspace-partitioned contact PII store for MSW.
 * Demo fixtures seed only the primary demo workspace; other slugs start empty
 * (fail closed — no cross-workspace contact visibility).
 */
const contactsByWorkspaceSlug = new Map<string, Contact[]>();

function workspaceContacts(slug: string | readonly string[] | undefined): Contact[] {
  const key = Array.isArray(slug) ? String(slug[0] ?? "") : String(slug ?? "");
  let list = contactsByWorkspaceSlug.get(key);
  if (!list) {
    list =
      key === defaultWorkspaceSettings.slug
        ? mockContacts.map((c) => ({ ...c }))
        : [];
    contactsByWorkspaceSlug.set(key, list);
  }
  return list;
}

function findMockContactById(id: string): Contact | undefined {
  for (const list of contactsByWorkspaceSlug.values()) {
    const hit = list.find((c) => c.id === id);
    if (hit) return hit;
  }
  return mockContacts.find((c) => c.id === id);
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
      const contact = findMockContactById(id);
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

/** E2E + PATCH overrides survive link array resets within a test run. */
const mockLinkAskPolicyOverrides = new Map<
  string,
  { askAiEnabled?: boolean; askMode?: string }
>();

function resolveLinkAskAiEnabled(link: MockLinkExt | undefined): boolean {
  if (!link) return false;
  const override = mockLinkAskPolicyOverrides.get(link.id);
  if (override?.askAiEnabled !== undefined) return override.askAiEnabled;
  return Boolean(link.askAiEnabled);
}

function resolveLinkAskMode(link: MockLinkExt | undefined): string {
  if (!link) return "supervised";
  const override = mockLinkAskPolicyOverrides.get(link.id);
  if (override?.askMode) return override.askMode;
  return link.askMode ?? "supervised";
}

function setLinkAskPolicyOverride(
  linkId: string,
  patch: { askAiEnabled?: boolean; askMode?: string },
) {
  const prev = mockLinkAskPolicyOverrides.get(linkId) ?? {};
  const next = { ...prev };
  if (patch.askAiEnabled !== undefined) next.askAiEnabled = patch.askAiEnabled;
  if (patch.askMode !== undefined) next.askMode = patch.askMode;
  mockLinkAskPolicyOverrides.set(linkId, next);
  const link = mockLinks.find((l) => l.id === linkId);
  if (link) {
    if (patch.askAiEnabled !== undefined) link.askAiEnabled = patch.askAiEnabled;
    if (patch.askMode !== undefined) link.askMode = patch.askMode as MockLinkExt["askMode"];
  }
}

// Snapshot of initial state so E2E tests can reset between cases.
const initialState = {
  workspaces: structuredClone(mockWorkspaces),
  documents: structuredClone(mockDocuments),
  links: structuredClone(mockLinks),
  dealRooms: structuredClone(mockDealRooms),
  members: structuredClone(mockWorkspaceMembers),
  suggestions: structuredClone(mockSuggestions),
  linkAccessRequests: structuredClone(mockLinkAccessRequests),
  ndaTemplates: structuredClone(mockNdaTemplates),
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
/** Active MSW auth session user id (set on login/register, cleared on logout). */
let mockAuthUserId: string | null = null;
/** Per-link visitor Ask turns for public MSW e2e. */
const mockPublicAskTurns = new Map<string, PublicAskTurn[]>();
/** Per-link AI Ask turns for owner inbox review. */
const mockOwnerAskAITurns = new Map<string, OwnerAskTurn[]>();
/** Per-link Formal queue turns for owner inbox (Phase C). */
const mockOwnerFormalTurns = new Map<string, OwnerAskTurn[]>();
/** Per-link pinned FAQ overrides for owner inbox (Phase B). */
const mockOwnerAskPinOverrides = new Map<
  string,
  Map<string, { pinned_faq_at: string; pinned_faq_by: string; pinned_faq_sort?: number }>
>();
/** Per-link formal Q&A overrides for owner inbox (Phase C). */
const mockOwnerAskFormalOverrides = new Map<string, Map<string, Partial<OwnerAskTurn>>>();
/** Per-link published formal Q&A visible to visitors. */
const mockPublicFormalAsk = new Map<string, PublicFormalAsk[]>();
/** Token-scoped pinned FAQ seeds for visitor public API (isolated from room inbox). */
const mockPublicAskFAQByToken = new Map<string, PublicAskFAQ[]>();
/** Thin room security: blocklist + outbound floors. */
const mockRoomAccessPolicies = new Map<string, DealRoomAccessPolicy>();

function defaultMockRoomAccessPolicy(roomId: string): DealRoomAccessPolicy {
  return {
    dealRoomId: roomId,
    configured: false,
    requireEmailVerificationFloor: false,
    requireNdaFloor: false,
    requireEmailVerification: false,
    requireNda: false,
    blockedEmails: [],
    allowedEmails: [],
  };
}

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

const VISITOR_ASK_CACHE = "msw-e2e-visitor-ask";
const VISITOR_ASK_STATE_URL = "https://msw.local/visitor-ask-state";

async function hydrateVisitorAskState() {
  if (typeof caches === "undefined") return;
  try {
    const cache = await caches.open(VISITOR_ASK_CACHE);
    const res = await cache.match(VISITOR_ASK_STATE_URL);
    if (!res) return;
    const data = (await res.json()) as {
      publicAskTurns?: [string, PublicAskTurn[]][];
      ownerAskAITurns?: [string, OwnerAskTurn[]][];
      linkAskPolicyOverrides?: [string, { askAiEnabled?: boolean; askMode?: string }][];
    };
    mockPublicAskTurns.clear();
    mockOwnerAskAITurns.clear();
    for (const [token, turns] of data.publicAskTurns ?? []) {
      mockPublicAskTurns.set(token, turns);
    }
    for (const [linkId, rows] of data.ownerAskAITurns ?? []) {
      mockOwnerAskAITurns.set(linkId, rows);
    }
    for (const [linkId, policy] of data.linkAskPolicyOverrides ?? []) {
      mockLinkAskPolicyOverrides.set(linkId, policy);
      const link = mockLinks.find((l) => l.id === linkId);
      if (link) {
        if (policy.askAiEnabled !== undefined) link.askAiEnabled = policy.askAiEnabled;
        if (policy.askMode !== undefined) {
          link.askMode = policy.askMode as MockLinkExt["askMode"];
        }
      }
    }
  } catch {
    /* ignore cache hydrate failures in non-SW contexts */
  }
}

async function persistVisitorAskState() {
  if (typeof caches === "undefined") return;
  try {
    const cache = await caches.open(VISITOR_ASK_CACHE);
    await cache.put(
      VISITOR_ASK_STATE_URL,
      new Response(
        JSON.stringify({
          publicAskTurns: [...mockPublicAskTurns.entries()],
          ownerAskAITurns: [...mockOwnerAskAITurns.entries()],
          linkAskPolicyOverrides: [...mockLinkAskPolicyOverrides.entries()],
        }),
        { headers: { "Content-Type": "application/json" } },
      ),
    );
  } catch {
    /* ignore */
  }
}

async function resetVisitorAskState() {
  mockPublicAskTurns.clear();
  mockOwnerAskAITurns.clear();
  if (typeof caches === "undefined") return;
  try {
    await caches.delete(VISITOR_ASK_CACHE);
  } catch {
    /* ignore */
  }
}

async function resetMockState() {
  mockUsers.clear();
  mockAuthUserId = null;
  mockLinkAskPolicyOverrides.clear();
  contactsByWorkspaceSlug.clear();
  await resetVisitorAskState();
  void resetKnowledgeQAState();
  seedOwnerAskHostQuestions();
  mockWorkspaces.splice(0, mockWorkspaces.length, ...initialState.workspaces);
  mockDocuments.splice(0, mockDocuments.length, ...initialState.documents);
  mockLinks.splice(0, mockLinks.length, ...initialState.links);
  mockDealRooms.splice(0, mockDealRooms.length, ...initialState.dealRooms);
  mockWorkspaceMembers.splice(0, mockWorkspaceMembers.length, ...initialState.members);
  mockSuggestions.splice(0, mockSuggestions.length, ...structuredClone(initialState.suggestions));
  mockRoomAccessPolicies.clear();
  mockLinkAccessRequests.splice(
    0,
    mockLinkAccessRequests.length,
    ...structuredClone(initialState.linkAccessRequests),
  );
  mockNdaTemplates.splice(
    0,
    mockNdaTemplates.length,
    ...structuredClone(initialState.ndaTemplates),
  );
  workspaceSettings = { ...initialState.settings };
  integrationsStatus = { ...initialState.integrations };
  securitySettings = { ...initialState.security };
  outboundWebhook = {
    configured: false,
    enabled: false,
    url: "",
    event_types: ["key_page", "repeat_key_page"],
    secret_hint: "",
    secret: "",
    updated_at: "",
  };
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
  const now = new Date().toISOString();
  mockOwnerAskPinOverrides.clear();
  mockOwnerAskFormalOverrides.clear();
  mockPublicFormalAsk.clear();
  mockPublicAskFAQByToken.clear();
  mockPublicAskTurns.set("RoomShare1", [
    {
      id: "owner_q_pending_1",
      session_id: "sess_host_1",
      link_id: "link_room_1",
      question: "Can you share the updated financial model?",
      lane: "host",
      status: "host_pending",
      visitor_email: "lp@example.com",
      created_at: now,
      updated_at: now,
    },
  ]);
  mockOwnerAskAITurns.clear();
  mockOwnerAskAITurns.set("link_room_1", [
    {
      id: "owner_ai_1",
      session_id: "sess_ai_1",
      link_id: "link_room_1",
      visitor_id: "visitor_ai",
      visitor_email: "analyst@example.com",
      question: "What was revenue growth last year?",
      lane: "ai",
      status: "ai_answered",
      ai_payload: {
        answer: "Revenue grew 12% year over year.",
        refused: false,
        resultStatus: "answered",
        hits: [
          {
            chunkId: "chunk_ai_seed",
            documentId: "doc_1",
            text: "Revenue increased 12% YoY in FY2024.",
            score: 0.9,
            sourceName: "Financial Summary.pdf",
            pages: [3],
            viewerPage: 3,
          },
        ],
      },
      created_at: now,
      updated_at: now,
    },
  ]);
  mockOwnerFormalTurns.clear();
  mockOwnerFormalTurns.set("link_room_1", [
    {
      id: "owner_formal_1",
      session_id: "sess_formal_1",
      link_id: "link_room_1",
      visitor_id: "visitor_formal",
      visitor_email: "compliance@example.com",
      question: "What is the board-approved revenue guidance?",
      lane: "host",
      status: "host_pending",
      route_reason: "policy_formal",
      formal_status: "pending_review",
      formal_anonymize: true,
      created_at: now,
      updated_at: now,
    },
  ]);
  mockPublicAskFAQByToken.set("AskSmoke1", [
    {
      id: "smoke_pinned_faq_1",
      question: "What is the company burn rate?",
      answer: "Monthly burn is approximately $420K.",
      source: "ai",
      pinned_at: now,
      link_id: "link_visitor_ask_smoke",
      link_name: "Visitor Ask Smoke Deck",
      ai_payload: {
        answer: "Monthly burn is approximately $420K.",
        refused: false,
        resultStatus: "answered",
      },
    },
  ]);
  mockPublicFormalAsk.set("AskSmoke1", [
    {
      id: "smoke_formal_published_1",
      question: "What disclosures are board-approved?",
      answer: "All revenue guidance is board-approved quarterly.",
      published_at: now,
      link_id: "link_visitor_ask_smoke",
      link_name: "Visitor Ask Smoke Deck",
    },
  ]);
}
seedOwnerAskHostQuestions();

function generateId(prefix: string): string {
  return `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2, 7)}`;
}

function publicTokenFromLink(link: Link): string {
  const parts = link.shortUrl.split("/");
  return parts[parts.length - 1] ?? "";
}

function syncPublicAskTurnAnswer(linkId: string, turnId: string, answer: string) {
  const link = mockLinks.find((l) => l.id === linkId);
  if (!link) return;
  const token = publicTokenFromLink(link);
  const now = new Date().toISOString();
  const turns = mockPublicAskTurns.get(token) ?? [];
  let changed = false;
  for (let i = 0; i < turns.length; i++) {
    const row = turns[i]!;
    if (row.id !== turnId) continue;
    turns[i] = {
      ...row,
      status: "host_answered",
      host_answer: answer,
      updated_at: now,
    };
    changed = true;
  }
  if (changed) {
    mockPublicAskTurns.set(token, turns);
  }
}

function syncOwnerAskAITurnFromPublic(token: string, turn: PublicAskTurn) {
  const link = findMockLinkByPublicToken(token);
  if (!link || turn.lane !== "ai") return;
  const row: OwnerAskTurn = {
    id: turn.id,
    session_id: turn.session_id,
    link_id: link.id,
    visitor_id: "visitor_mock",
    question: turn.question,
    lane: "ai",
    status: turn.status,
    ai_payload: turn.ai_payload,
    created_at: turn.created_at,
    updated_at: turn.updated_at,
  };
  const list = mockOwnerAskAITurns.get(link.id) ?? [];
  const idx = list.findIndex((item) => item.id === turn.id);
  if (idx >= 0) {
    list[idx] = row;
  } else {
    list.push(row);
  }
  mockOwnerAskAITurns.set(link.id, list);
}

function mockOwnerAskTurnsForLink(linkId: string): OwnerAskTurn[] {
  const link = mockLinks.find((l) => l.id === linkId);
  const byId = new Map<string, OwnerAskTurn>();

  if (link) {
    const token = publicTokenFromLink(link);
    for (const turn of mockPublicAskTurns.get(token) ?? []) {
      const extended = turn as PublicAskTurn & { visitor_email?: string };
      byId.set(turn.id, {
        id: turn.id,
        session_id: turn.session_id,
        link_id: linkId,
        visitor_id: "visitor_mock",
        visitor_email: extended.visitor_email,
        question: turn.question,
        lane: turn.lane,
        status: turn.status,
        host_answer: turn.host_answer,
        ai_payload: turn.ai_payload,
        route_reason: turn.route_reason,
        formal_status: turn.formal_status,
        formal_publish_at: turn.formal_publish_at,
        formal_published_at: turn.formal_published_at,
        formal_anonymize: turn.formal_anonymize,
        created_at: turn.created_at,
        updated_at: turn.updated_at,
      });
    }
  }

  for (const ai of mockOwnerAskAITurns.get(linkId) ?? []) {
    if (!byId.has(ai.id)) {
      byId.set(ai.id, ai);
    }
  }

  for (const formal of mockOwnerFormalTurns.get(linkId) ?? []) {
    if (!byId.has(formal.id)) {
      byId.set(formal.id, formal);
    }
  }

  const pinOverrides = mockOwnerAskPinOverrides.get(linkId);
  if (pinOverrides) {
    for (const [turnId, turn] of byId) {
      const pinned = pinOverrides.get(turnId);
      if (pinned) {
        byId.set(turnId, { ...turn, ...pinned });
      }
    }
  }

  const formalOverrides = mockOwnerAskFormalOverrides.get(linkId);
  if (formalOverrides) {
    for (const [turnId, turn] of byId) {
      const formal = formalOverrides.get(turnId);
      if (formal) {
        byId.set(turnId, { ...turn, ...formal });
      }
    }
  }

  return Array.from(byId.values());
}

function mockMaxLinkPinnedFAQSort(linkId: string): number {
  let max = -1;
  for (const turn of mockOwnerAskTurnsForLink(linkId)) {
    if (turn.pinned_faq_at && turn.pinned_faq_sort != null) {
      max = Math.max(max, turn.pinned_faq_sort);
    }
  }
  return max;
}

function mockSortOwnerAskPinnedFAQs(turns: OwnerAskTurn[]): OwnerAskTurn[] {
  return [...turns].sort((a, b) => {
    const aSort = a.pinned_faq_sort ?? Number.MAX_SAFE_INTEGER;
    const bSort = b.pinned_faq_sort ?? Number.MAX_SAFE_INTEGER;
    if (aSort !== bSort) return aSort - bSort;
    const aPinned = a.pinned_faq_at ? new Date(a.pinned_faq_at).getTime() : 0;
    const bPinned = b.pinned_faq_at ? new Date(b.pinned_faq_at).getTime() : 0;
    return bPinned - aPinned;
  });
}

function mockPublicAskFAQsForToken(token: string) {
  const link = findMockLinkByPublicToken(token);
  if (!link) return mockPublicAskFAQByToken.get(token) ?? [];
  const linkIds = link.dealRoomId
    ? mockLinks.filter((l) => l.dealRoomId === link.dealRoomId).map((l) => l.id)
    : [link.id];
  const rows: PublicAskFAQ[] = [...(mockPublicAskFAQByToken.get(token) ?? [])];
  const seen = new Set(rows.map((row) => row.id));
  for (const linkId of linkIds) {
    const sourceLink = mockLinks.find((l) => l.id === linkId);
    for (const turn of mockOwnerAskTurnsForLink(linkId).filter((t) => t.pinned_faq_at)) {
      if (seen.has(turn.id)) continue;
      const answer = (turn.host_answer ?? turn.ai_payload?.answer ?? "").trim();
      if (!answer) continue;
      rows.push({
        id: turn.id,
        question: turn.question,
        answer,
        source: turn.lane,
        ai_payload: turn.ai_payload,
        pinned_at: turn.pinned_faq_at!,
        link_id: linkId,
        link_name: sourceLink?.name ?? sourceLink?.documentTitle,
      });
      seen.add(turn.id);
    }
  }
  rows.sort((a, b) => {
    const aTurn = mockOwnerAskTurnsForLink(a.link_id!).find((t) => t.id === a.id);
    const bTurn = mockOwnerAskTurnsForLink(b.link_id!).find((t) => t.id === b.id);
    const aSort = aTurn?.pinned_faq_sort ?? a.pinned_faq_sort ?? Number.MAX_SAFE_INTEGER;
    const bSort = bTurn?.pinned_faq_sort ?? b.pinned_faq_sort ?? Number.MAX_SAFE_INTEGER;
    if (aSort !== bSort) return aSort - bSort;
    return new Date(b.pinned_at).getTime() - new Date(a.pinned_at).getTime();
  });
  return rows;
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
    mockAuthUserId = id;
    return HttpResponse.json(createTokenResponse(id, email), { status: 201, headers: authSessionCookieHeader() });
  }),

  http.post("*/api/auth/login", async ({ request }) => {
    const body = (await request.json()) as { email?: string; password?: string };
    const email = body.email?.trim().toLowerCase();
    const user = Array.from(mockUsers.values()).find((u) => u.email === email);
    if (!user || user.password !== body.password) {
      return HttpResponse.json({ code: "unauthorized", message: "invalid email or password" }, { status: 401 });
    }
    mockAuthUserId = user.id;
    return HttpResponse.json(createTokenResponse(user.id, user.email), { headers: authSessionCookieHeader() });
  }),

  http.get("*/api/auth/me", () => {
    const user = mockAuthUserId ? mockUsers.get(mockAuthUserId) : undefined;
    if (!user) {
      return HttpResponse.json({ code: "unauthorized", message: "missing authorization" }, { status: 401 });
    }
    return HttpResponse.json({
      user: { id: user.id, email: user.email, name: user.name },
    });
  }),

  http.post("*/api/auth/refresh", async () => {
    return HttpResponse.json({ expires_in: 900 }, { headers: authSessionCookieHeader() });
  }),

  http.post("*/api/auth/logout", async () => {
    mockAuthUserId = null;
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
    type E2EResetBody = {
      action?: string;
      roomId?: string;
      linkId?: string;
      askAiEnabled?: boolean;
      status?: string;
      documentStatus?: string;
      jobStatus?: string;
      code?: string;
      httpStatus?: number;
      clear?: boolean;
    };
    let body: E2EResetBody | null = null;
    try {
      const text = await request.text();
      // Do not use NonNullable<typeof body> — when body is null-narrowed that becomes never.
      if (text.trim()) body = JSON.parse(text) as E2EResetBody;
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
    if (body?.action === "link-ask-policy") {
      const linkId = (body.linkId || "").trim();
      if (!linkId) {
        return HttpResponse.json(
          { code: "invalid_input", message: "linkId is required" },
          { status: 400 },
        );
      }
      const link = mockLinks.find((l) => l.id === linkId);
      if (!link) {
        return HttpResponse.json({ code: "not_found", message: "link not found" }, { status: 404 });
      }
      await hydrateVisitorAskState();
      setLinkAskPolicyOverride(linkId, {
        ...(typeof body.askAiEnabled === "boolean" ? { askAiEnabled: body.askAiEnabled } : {}),
      });
      await persistVisitorAskState();
      return new HttpResponse(null, { status: 204 });
    }
    await resetMockState();
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
      docs = docs.filter((d) => (d.category ?? "general") !== "deal_room");
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
    if (doc.category === "deal_room") {
      return HttpResponse.json(
        { code: "category_immutable", message: "document category cannot be changed" },
        { status: 409 },
      );
    }
    const inRoom = mockDealRooms.some((r) =>
      (r.documents ?? []).some((fd) => fd.documents.some((d) => d.document_id === doc.id)),
    );
    if (inRoom) {
      return HttpResponse.json(
        {
          code: "category_while_in_room",
          message: "remove the document from deal rooms before changing category",
        },
        { status: 409 },
      );
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
    if (category === "deal_room") {
      return HttpResponse.json(
        {
          code: "category_deal_room_via_api",
          message: "deal room category is managed by membership",
        },
        { status: 400 },
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
      category: (category === "agreement"
        ? category
        : "general") as import("@/types").DocumentCategory,
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

  // NDA templates (workspace agreement documents). Static path before /nda/templates/:id.
  http.get("*/api/workspaces/:workspaceSlug/nda/templates", ({ request }) => {
    const url = new URL(request.url);
    const includeArchived = url.searchParams.get("include_archived") === "true";
    const data = includeArchived
      ? mockNdaTemplates
      : mockNdaTemplates.filter((tpl) => tpl.status === "active");
    return HttpResponse.json({ data });
  }),

  http.post("*/api/workspaces/:workspaceSlug/nda/templates", async ({ request }) => {
    const body = (await request.json()) as { document_id?: string; name?: string; require_signer_name?: boolean };
    const documentId = body.document_id?.trim() ?? "";
    const doc = mockDocuments.find((d) => d.id === documentId);
    if (!doc) {
      return HttpResponse.json({ code: "not_found", message: "document not found" }, { status: 404 });
    }
    if ((doc.category ?? "general") !== "agreement" && doc.fileType !== "pdf" && doc.sourceType !== "pdf") {
      return HttpResponse.json(
        { code: "invalid_input", message: "nda template source must be a PDF agreement" },
        { status: 400 },
      );
    }
    const existing = mockNdaTemplates.find(
      (tpl) => tpl.source_document_id === documentId && tpl.status === "active",
    );
    if (existing) {
      return HttpResponse.json({ data: existing }, { status: 201 });
    }
    const now = new Date().toISOString();
    const created = {
      id: generateId("nda_tpl"),
      name: (body.name?.trim() || doc.title || "NDA Agreement").trim(),
      source_document_id: documentId,
      content_sha256: `mock-sha-${documentId}`,
      require_signer_name: body.require_signer_name !== false,
      status: "active",
      response_count: 0,
      link_count: 0,
      created_at: now,
      updated_at: now,
    };
    mockNdaTemplates.unshift(created);
    return HttpResponse.json({ data: created }, { status: 201 });
  }),

  http.get("*/api/workspaces/:workspaceSlug/nda/templates/:templateId", ({ params }) => {
    const tpl = mockNdaTemplates.find((t) => t.id === params.templateId);
    if (!tpl) {
      return HttpResponse.json({ code: "not_found", message: "template not found" }, { status: 404 });
    }
    return HttpResponse.json({ data: tpl });
  }),

  // Viewer events
  http.post("*/api/workspaces/:workspaceSlug/events", async () => {
    return new HttpResponse(null, { status: 204 });
  }),

  // Links
  http.get("*/api/workspaces/:workspaceSlug/links", ({ request }) => {
    const url = new URL(request.url);
    const documentId = url.searchParams.get("documentId");
    // Document Library share list: never include deal-room shares.
    const documentLinks = mockLinks.filter((l) => !l.dealRoomId && l.documentId);
    const data = documentId
      ? documentLinks.filter((l) => l.documentId === documentId)
      : documentLinks;
    return HttpResponse.json({ data });
  }),

  // Static path MUST be registered before /links/:id or MSW treats it as an id (404).
  http.get("*/api/workspaces/:workspaceSlug/links/pending-access-requests", ({ request }) => {
    const url = new URL(request.url);
    const scope = url.searchParams.get("scope") || "document";
    const dealRoomId = url.searchParams.get("deal_room_id") || "";
    if (scope === "deal_room" && !dealRoomId) {
      return HttpResponse.json(
        { code: "invalid_input", message: "deal_room_id is required when scope=deal_room" },
        { status: 400 },
      );
    }
    if (scope !== "document" && scope !== "deal_room") {
      return HttpResponse.json(
        { code: "invalid_input", message: "scope must be document or deal_room" },
        { status: 400 },
      );
    }
    const data = mockLinkAccessRequests
      .filter((r) => r.status === "pending")
      .map((r) => {
        const link = mockLinks.find((l) => l.id === r.link_id);
        return {
          ...r,
          link_name: link?.name ?? "",
          document_title: link?.documentTitle ?? "",
          short_url: link?.shortUrl ?? "",
          _dealRoomId: link?.dealRoomId,
          _documentId: link?.documentId,
        };
      })
      .filter((r) => {
        if (scope === "deal_room") {
          return r._dealRoomId === dealRoomId;
        }
        return !r._dealRoomId && !!r._documentId;
      })
      .map(({ _dealRoomId: _dr, _documentId: _doc, ...rest }) => rest);
    return HttpResponse.json({ data });
  }),

  http.get("*/api/workspaces/:workspaceSlug/links/:id", ({ params }) => {
    if (params.id === "pending-access-requests") {
      return HttpResponse.json(
        { code: "not_found", message: "use /links/pending-access-requests" },
        { status: 404 },
      );
    }
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
      ask_ai_enabled?: boolean;
      ask_mode?: string;
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
    if (link.dealRoomId) {
      link.qaEnabled = true;
      if (typeof payload.ask_ai_enabled === "boolean") {
        link.askAiEnabled = payload.ask_ai_enabled;
      }
      const payloadAskMode = (payload as { ask_mode?: string }).ask_mode;
      if (
        payloadAskMode === "supervised" ||
        payloadAskMode === "self_serve" ||
        payloadAskMode === "formal"
      ) {
        link.askMode = payloadAskMode;
      }
    } else if (typeof payload.qa_enabled === "boolean") {
      link.qaEnabled = payload.qa_enabled;
    } else {
      link.qaEnabled = false;
    }
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
    const patchAskAi = (patch as { askAiEnabled?: boolean }).askAiEnabled;
    const patchAskAiSnake = (patch as { ask_ai_enabled?: boolean }).ask_ai_enabled;
    if (link.dealRoomId) {
      link.qaEnabled = true;
      if (typeof patchAskAi === "boolean") {
        link.askAiEnabled = patchAskAi;
      } else if (typeof patchAskAiSnake === "boolean") {
        link.askAiEnabled = patchAskAiSnake;
      }
      const patchAskMode = (patch as { ask_mode?: string }).ask_mode;
      if (
        patchAskMode === "supervised" ||
        patchAskMode === "self_serve" ||
        patchAskMode === "formal"
      ) {
        link.askMode = patchAskMode;
      }
    } else {
      const patchQa = patch.qaEnabled;
      const patchSnake = (patch as { qa_enabled?: boolean }).qa_enabled;
      if (typeof patchQa === "boolean") {
        link.qaEnabled = patchQa;
      } else if (typeof patchSnake === "boolean") {
        link.qaEnabled = patchSnake;
      } else {
        link.qaEnabled = false;
      }
    }
    return HttpResponse.json(link);
  }),

  http.delete("*/api/workspaces/:workspaceSlug/links/:id", ({ params }) => {
    const index = mockLinks.findIndex((l) => l.id === params.id);
    if (index === -1) return new HttpResponse(null, { status: 404 });
    mockLinks.splice(index, 1);
    return new HttpResponse(null, { status: 204 });
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
      const wsContacts = workspaceContacts(params.workspaceSlug);
      const existing = wsContacts.find(
        (c) => c.email.toLowerCase() === req.email.toLowerCase(),
      );
      if (existing) {
        if (req.signer_name && !existing.name) {
          existing.name = req.signer_name;
        }
      } else {
        wsContacts.unshift({
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
      qaEnabled: false,
    } as MockLinkExt;
    mockLinks.unshift(newLink);
    return HttpResponse.json(newLink, { status: 201 });
  }),

  // Contacts (workspace-scoped PII — never share across :workspaceSlug)
  http.get("*/api/workspaces/:workspaceSlug/contacts", ({ params }) => {
    return HttpResponse.json({ data: workspaceContacts(params.workspaceSlug) });
  }),

  http.post("*/api/workspaces/:workspaceSlug/contacts", async ({ params, request }) => {
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
    workspaceContacts(params.workspaceSlug).unshift(newContact);
    return HttpResponse.json(newContact, { status: 201 });
  }),

  http.get("*/api/workspaces/:workspaceSlug/contacts/:id", ({ params }) => {
    const contact = workspaceContacts(params.workspaceSlug).find((c) => c.id === params.id);
    if (!contact) {
      return HttpResponse.json(
        { code: "contact_not_found", message: "contact not found" },
        { status: 404 },
      );
    }
    return HttpResponse.json(contact);
  }),

  http.get("*/api/workspaces/:workspaceSlug/contacts/:id/activities", ({ params }) => {
    const contact = workspaceContacts(params.workspaceSlug).find((c) => c.id === params.id);
    if (!contact) {
      return HttpResponse.json(
        { code: "contact_not_found", message: "contact not found" },
        { status: 404 },
      );
    }
    return HttpResponse.json({ data: mockActivities.filter((a) => a.contactId === params.id) });
  }),

  http.post("*/api/workspaces/:workspaceSlug/marketing/send", async ({ params, request }) => {
    const body = (await request.json()) as {
      recipients?: string[];
      subject?: string;
    };
    const recipients = Array.isArray(body.recipients)
      ? body.recipients.map((e) => String(e).trim().toLowerCase()).filter(Boolean)
      : [];
    if (!body.subject?.trim() || recipients.length === 0) {
      return HttpResponse.json(
        { code: "invalid_input", message: "invalid input" },
        { status: 400 },
      );
    }
    const known = new Set(
      workspaceContacts(params.workspaceSlug).map((c) => c.email.trim().toLowerCase()),
    );
    const unknown = recipients.filter((email) => !known.has(email));
    if (unknown.length > 0) {
      return HttpResponse.json(
        {
          code: "recipients_not_in_workspace",
          message: "one or more recipients are not contacts in this workspace",
          unknown,
        },
        { status: 400 },
      );
    }
    return HttpResponse.json({
      data: {
        sent: recipients.length,
        failed: 0,
        log_ids: recipients.map((_, i) => `elog_${i}`),
        failed_recipients: [],
      },
    });
  }),

  // Deal rooms
  http.get("*/api/workspaces/:workspaceSlug/deal-rooms", ({ request }) => {
    const url = new URL(request.url);
    const q = (url.searchParams.get("q") || "").toLowerCase().trim();
    const filtered = q
      ? mockDealRooms.filter(
          (room) =>
            room.name.toLowerCase().includes(q) ||
            (room.description || "").toLowerCase().includes(q),
        )
      : mockDealRooms;
    const pageRaw = url.searchParams.get("page");
    if (!pageRaw) {
      return HttpResponse.json({ data: filtered });
    }
    const page = Math.max(1, Number(pageRaw) || 1);
    const pageSize = Math.min(100, Math.max(1, Number(url.searchParams.get("page_size")) || 24));
    const total = filtered.length;
    const start = (page - 1) * pageSize;
    const slice = filtered.slice(start, start + pageSize);
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

  http.get("*/api/workspaces/:workspaceSlug/deal-rooms/:id/access-policy", ({ params }) => {
    const roomId = params.id as string;
    if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({
      data: mockRoomAccessPolicies.get(roomId) ?? defaultMockRoomAccessPolicy(roomId),
    });
  }),

  http.put("*/api/workspaces/:workspaceSlug/deal-rooms/:id/access-policy", async ({ params, request }) => {
    const roomId = params.id as string;
    if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
    const body = (await request.json()) as {
      require_email_verification_floor?: boolean;
      require_nda_floor?: boolean;
      require_email_verification?: boolean;
      require_nda?: boolean;
      blocked_emails?: string[];
    };
    const verifyFloor = !!(body.require_email_verification_floor ?? body.require_email_verification);
    const ndaFloor = !!(body.require_nda_floor ?? body.require_nda);
    const blocked = (body.blocked_emails ?? []).map((e) => e.trim().toLowerCase()).filter(Boolean);
    const policy: DealRoomAccessPolicy = {
      dealRoomId: roomId,
      configured: true,
      requireEmailVerificationFloor: verifyFloor,
      requireNdaFloor: ndaFloor,
      requireEmailVerification: verifyFloor,
      requireNda: ndaFloor,
      allowedEmails: [],
      blockedEmails: blocked,
      updatedAt: new Date().toISOString(),
    };
    mockRoomAccessPolicies.set(roomId, policy);
    // Room save syncs blocklist only — never broadcast-overwrite link gates.
    return HttpResponse.json({ data: policy });
  }),

  http.get("*/api/workspaces/:workspaceSlug/deal-rooms/:id/links", ({ params, request }) => {
    const roomId = params.id as string;
    if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
    const url = new URL(request.url);
    const pageRaw = url.searchParams.get("page");
    // Deal-room share list: room-scoped links only (never document-library shares).
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
      ask_ai_enabled?: boolean;
      ask_mode?: string;
      qa_enabled?: boolean;
      folder_paths?: string[];
      folder_scope_mode?: "full" | "allowlist";
      document_ids?: string[];
      require_email?: boolean;
      require_email_verification?: boolean;
      require_password?: boolean;
      require_nda?: boolean;
      download_enabled?: boolean;
      watermark_enabled?: boolean;
      allowed_emails?: string[];
      blocked_emails?: string[];
    };
    const policy = mockRoomAccessPolicies.get(roomId);
    const floors = {
      verify: Boolean(policy?.requireEmailVerificationFloor ?? policy?.requireEmailVerification),
      nda: Boolean(policy?.requireNdaFloor ?? policy?.requireNda),
    };
    const documentIds = body.document_ids?.length ? body.document_ids : ["doc_1"];
    const folderScopeMode =
      body.folder_scope_mode === "allowlist" || (body.folder_paths?.length ?? 0) > 0
        ? "allowlist"
        : "full";
    // Allowlist is always link-scoped — never inherited from room policy.
    const allowedEmails = (body.allowed_emails ?? []).map((e) => e.trim().toLowerCase());
    const requireEmailVerification =
      floors.verify || !!body.require_email_verification;
    const newLink: Link = {
      id: generateId("link"),
      name: body.name,
      documentId: documentIds[0],
      documentIds,
      folderPaths: folderScopeMode === "allowlist" ? (body.folder_paths ?? []) : [],
      folderScopeMode,
      documentTitle: "Deal room link",
      shortUrl: `https://invest.acme.capital/d/${generateId("sh")}`,
      accessCount: 0,
      heatLevel: "cold",
      createdAt: new Date().toISOString(),
      isActive: true,
      avgDurationSeconds: 0,
      permissionType: "public",
      isBundle: documentIds.length > 1,
      qaEnabled: true,
      askAiEnabled:
        typeof body.ask_ai_enabled === "boolean" ? body.ask_ai_enabled : true,
      askMode:
        body.ask_mode === "self_serve" || body.ask_mode === "formal"
          ? body.ask_mode
          : "supervised",
      dealRoomId: roomId,
      requireEmail: requireEmailVerification ? false : !!body.require_email,
      requireEmailVerification,
      requirePassword: !!body.require_password,
      requireNda: floors.nda || !!body.require_nda,
      downloadEnabled: body.download_enabled,
      watermarkEnabled: body.watermark_enabled,
      documents: [],
    };
    (newLink as MockLinkExt)._allowedEmails = allowedEmails;
    mockLinks.unshift(newLink);
    if (!policy?.configured) {
      mockRoomAccessPolicies.set(roomId, {
        ...defaultMockRoomAccessPolicy(roomId),
        configured: true,
        requireEmailVerificationFloor: false,
        requireNdaFloor: false,
        requireEmailVerification: false,
        requireNda: false,
        allowedEmails: [],
        blockedEmails: (body.blocked_emails ?? []).map((e) => e.trim().toLowerCase()),
        updatedAt: new Date().toISOString(),
      });
    }
    return HttpResponse.json(newLink, { status: 201 });
  }),

  http.get("*/api/workspaces/:workspaceSlug/links/:id/ask", async ({ params, request }) => {
    await hydrateVisitorAskState();
    const linkId = params.id as string;
    const link = mockLinks.find((l) => l.id === linkId);
    if (!link) return new HttpResponse(null, { status: 404 });
    const lane = new URL(request.url).searchParams.get("lane");
    const status = new URL(request.url).searchParams.get("status");
    let rows = attachOwnerAskRepeatCounts(mockOwnerAskTurnsForLink(linkId));
    rows = rows.filter((r) => matchesOwnerAskInboxFilter(r, lane, status));
    return HttpResponse.json({ data: rows });
  }),

  http.get("*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/ask", async ({ params, request }) => {
    await hydrateVisitorAskState();
    const roomId = params.roomId as string;
    if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
    const filterLinkId = new URL(request.url).searchParams.get("link_id");
    const lane = new URL(request.url).searchParams.get("lane");
    const status = new URL(request.url).searchParams.get("status");
    const roomLinkIds = new Set(
      mockLinks.filter((l) => l.dealRoomId === roomId).map((l) => l.id),
    );
    const rows: OwnerAskTurn[] = [];
    for (const linkId of roomLinkIds) {
      if (filterLinkId && linkId !== filterLinkId) continue;
      rows.push(...mockOwnerAskTurnsForLink(linkId));
    }
    let filtered = attachOwnerAskRepeatCounts(rows);
    filtered = filtered.filter((r) => matchesOwnerAskInboxFilter(r, lane, status));
    filtered.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
    return HttpResponse.json({ data: filtered });
  }),

  http.patch(
    "*/api/workspaces/:workspaceSlug/links/:id/ask/:turnId/host-answer",
    async ({ params, request }) => {
      await hydrateVisitorAskState();
      const linkId = params.id as string;
      const turnId = params.turnId as string;
      const link = mockLinks.find((l) => l.id === linkId);
      if (!link) return new HttpResponse(null, { status: 404 });
      const body = (await request.json().catch(() => ({}))) as { answer?: string };
      const answer = (body.answer ?? "").trim();
      if (!answer) {
        return HttpResponse.json({ code: "invalid_input", message: "answer required" }, { status: 400 });
      }
      const turns = mockOwnerAskTurnsForLink(linkId);
      const turn = turns.find((t) => t.id === turnId);
      if (!turn) {
        return HttpResponse.json({ code: "not_found", message: "ask turn not found" }, { status: 404 });
      }
      syncPublicAskTurnAnswer(linkId, turnId, answer);
      await persistVisitorAskState();
      return HttpResponse.json({
        data: {
          ...turn,
          host_answer: answer,
          status: "host_answered" as const,
          updated_at: new Date().toISOString(),
        },
      });
    },
  ),

  http.patch(
    "*/api/workspaces/:workspaceSlug/links/:id/ask/:turnId/formal-publish",
    async ({ params, request }) => {
      await hydrateVisitorAskState();
      const linkId = params.id as string;
      const turnId = params.turnId as string;
      const link = mockLinks.find((l) => l.id === linkId);
      if (!link) return new HttpResponse(null, { status: 404 });
      const body = (await request.json().catch(() => ({}))) as {
        answer?: string;
        publish_at?: string;
        anonymize?: boolean;
      };
      const answer = (body.answer ?? "").trim();
      if (!answer) {
        return HttpResponse.json({ code: "invalid_input", message: "answer required" }, { status: 400 });
      }
      const turns = mockOwnerAskTurnsForLink(linkId);
      const turn = turns.find((t) => t.id === turnId);
      if (!turn) {
        return HttpResponse.json({ code: "not_found", message: "ask turn not found" }, { status: 404 });
      }
      const now = new Date().toISOString();
      const scheduled =
        Boolean(body.publish_at) && new Date(body.publish_at!).getTime() > Date.now();
      const formalStatus = scheduled ? "scheduled" : "published";
      const updated: OwnerAskTurn = {
        ...turn,
        host_answer: answer,
        formal_status: formalStatus,
        formal_publish_at: body.publish_at ?? now,
        formal_published_at: scheduled ? undefined : now,
        formal_anonymize: body.anonymize !== false,
        status: scheduled ? "host_pending" : "host_answered",
        updated_at: now,
      };
      const perLink = mockOwnerAskFormalOverrides.get(linkId) ?? new Map();
      perLink.set(turnId, {
        host_answer: updated.host_answer,
        formal_status: updated.formal_status,
        formal_publish_at: updated.formal_publish_at,
        formal_published_at: updated.formal_published_at,
        formal_anonymize: updated.formal_anonymize,
        status: updated.status,
        updated_at: updated.updated_at,
      });
      mockOwnerAskFormalOverrides.set(linkId, perLink);
      if (!scheduled) {
        const token = publicTokenFromLink(link);
        const published: PublicFormalAsk = {
          id: turnId,
          question: turn.question,
          answer,
          published_at: now,
          link_id: linkId,
          link_name: link.name,
          ...(updated.formal_anonymize
            ? {}
            : turn.visitor_email
              ? { visitor_email: turn.visitor_email }
              : {}),
        };
        const existing = mockPublicFormalAsk.get(token) ?? [];
        mockPublicFormalAsk.set(token, [
          published,
          ...existing.filter((entry) => entry.id !== turnId),
        ]);
        syncPublicAskTurnAnswer(linkId, turnId, answer);
      }
      await persistVisitorAskState();
      return HttpResponse.json({ data: updated });
    },
  ),

  http.post(
    "*/api/workspaces/:workspaceSlug/links/:id/ask/:turnId/pin-faq",
    async ({ params }) => {
      await hydrateVisitorAskState();
      const linkId = params.id as string;
      const turnId = params.turnId as string;
      const link = mockLinks.find((l) => l.id === linkId);
      if (!link) return new HttpResponse(null, { status: 404 });
      const turns = mockOwnerAskTurnsForLink(linkId);
      const turn = turns.find((t) => t.id === turnId);
      if (!turn) {
        return HttpResponse.json({ code: "not_found", message: "ask turn not found" }, { status: 404 });
      }
      if (turn.pinned_faq_at) {
        return HttpResponse.json({ data: turn });
      }
      if (!ownerAskTurnCanPinFAQ(turn)) {
        return HttpResponse.json(
          { code: "ask_turn_not_pinnable", message: "ask turn cannot be pinned as faq" },
          { status: 409 },
        );
      }
      const pinned = {
        pinned_faq_at: new Date().toISOString(),
        pinned_faq_by: "user_1",
        pinned_faq_sort: mockMaxLinkPinnedFAQSort(linkId) + 1,
      };
      const perLink = mockOwnerAskPinOverrides.get(linkId) ?? new Map();
      perLink.set(turnId, pinned);
      mockOwnerAskPinOverrides.set(linkId, perLink);
      return HttpResponse.json({
        data: {
          ...turn,
          ...pinned,
          updated_at: pinned.pinned_faq_at,
        },
      });
    },
  ),

  http.get("*/api/workspaces/:workspaceSlug/links/:id/ask/faq", async ({ params }) => {
    await hydrateVisitorAskState();
    const linkId = params.id as string;
    const link = mockLinks.find((l) => l.id === linkId);
    if (!link) return new HttpResponse(null, { status: 404 });
    const rows = mockSortOwnerAskPinnedFAQs(
      mockOwnerAskTurnsForLink(linkId).filter((t) => t.pinned_faq_at),
    );
    return HttpResponse.json({ data: rows });
  }),

  http.patch(
    "*/api/workspaces/:workspaceSlug/links/:id/ask/faq/order",
    async ({ params, request }) => {
      await hydrateVisitorAskState();
      const linkId = params.id as string;
      const link = mockLinks.find((l) => l.id === linkId);
      if (!link) return new HttpResponse(null, { status: 404 });
      const body = (await request.json().catch(() => ({}))) as { turn_ids?: string[] };
      const turnIds = body.turn_ids ?? [];
      const pinned = mockSortOwnerAskPinnedFAQs(
        mockOwnerAskTurnsForLink(linkId).filter((t) => t.pinned_faq_at),
      );
      if (turnIds.length === 0 || turnIds.length !== pinned.length) {
        return HttpResponse.json(
          { code: "ask_faq_reorder_invalid", message: "ask faq reorder invalid" },
          { status: 409 },
        );
      }
      const pinnedIdSet = new Set(pinned.map((t) => t.id));
      if (!turnIds.every((id) => pinnedIdSet.has(id))) {
        return HttpResponse.json(
          { code: "ask_faq_reorder_invalid", message: "ask faq reorder invalid" },
          { status: 409 },
        );
      }
      const perLink = mockOwnerAskPinOverrides.get(linkId) ?? new Map();
      for (let i = 0; i < turnIds.length; i++) {
        const turnId = turnIds[i];
        const turn = pinned.find((t) => t.id === turnId)!;
        const existing = perLink.get(turnId) ?? {
          pinned_faq_at: turn.pinned_faq_at!,
          pinned_faq_by: turn.pinned_faq_by ?? "user_1",
        };
        perLink.set(turnId, { ...existing, pinned_faq_sort: i });
      }
      mockOwnerAskPinOverrides.set(linkId, perLink);
      const updated = mockSortOwnerAskPinnedFAQs(
        mockOwnerAskTurnsForLink(linkId).filter((t) => t.pinned_faq_at),
      );
      return HttpResponse.json({ data: updated });
    },
  ),

  http.get("*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/ask/faq", async ({ params, request }) => {
    await hydrateVisitorAskState();
    const roomId = params.roomId as string;
    if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
    const filterLinkId = new URL(request.url).searchParams.get("link_id");
    const roomLinkIds = new Set(
      mockLinks.filter((l) => l.dealRoomId === roomId).map((l) => l.id),
    );
    const rows: OwnerAskTurn[] = [];
    for (const linkId of roomLinkIds) {
      if (filterLinkId && linkId !== filterLinkId) continue;
      rows.push(...mockOwnerAskTurnsForLink(linkId).filter((t) => t.pinned_faq_at));
    }
    rows.sort((a, b) => {
      const aSort = a.pinned_faq_sort ?? Number.MAX_SAFE_INTEGER;
      const bSort = b.pinned_faq_sort ?? Number.MAX_SAFE_INTEGER;
      if (aSort !== bSort) return aSort - bSort;
      return new Date(b.pinned_faq_at!).getTime() - new Date(a.pinned_faq_at!).getTime();
    });
    return HttpResponse.json({ data: rows });
  }),

  http.post(
    "*/api/workspaces/:workspaceSlug/links/:id/ask/:turnId/unpin-faq",
    async ({ params }) => {
      await hydrateVisitorAskState();
      const linkId = params.id as string;
      const turnId = params.turnId as string;
      const link = mockLinks.find((l) => l.id === linkId);
      if (!link) return new HttpResponse(null, { status: 404 });
      const turns = mockOwnerAskTurnsForLink(linkId);
      const turn = turns.find((t) => t.id === turnId);
      if (!turn) {
        return HttpResponse.json({ code: "not_found", message: "ask turn not found" }, { status: 404 });
      }
      if (!turn.pinned_faq_at) {
        return HttpResponse.json(
          { code: "ask_turn_not_pinned", message: "ask turn is not pinned as faq" },
          { status: 409 },
        );
      }
      const perLink = mockOwnerAskPinOverrides.get(linkId) ?? new Map();
      perLink.delete(turnId);
      mockOwnerAskPinOverrides.set(linkId, perLink);
      const { pinned_faq_at: _a, pinned_faq_by: _b, ...rest } = turn;
      return HttpResponse.json({
        data: {
          ...rest,
          updated_at: new Date().toISOString(),
        },
      });
    },
  ),

  http.get("*/api/workspaces/:workspaceSlug/links/:id/ask-policy", ({ params }) => {
    const linkId = params.id as string;
    const link = mockLinks.find((l) => l.id === linkId);
    if (!link) return new HttpResponse(null, { status: 404 });
    const used = 0;
    const limit = 500;
    return HttpResponse.json({
      data: {
        id: link.id,
        ask_mode: resolveLinkAskMode(link),
        ask_ai_enabled: resolveLinkAskAiEnabled(link),
        ask_ai_monthly_quota: null,
        ask_ai_monthly_used: used,
        ask_ai_monthly_limit: limit,
        ask_ai_quota_exceeded: used >= limit,
        ask_ai_entitled: Boolean(link.dealRoomId),
        formal_entitled: Boolean(link.dealRoomId),
      },
    });
  }),

  http.patch("*/api/workspaces/:workspaceSlug/links/:id/ask-policy", async ({ params, request }) => {
    const linkId = params.id as string;
    const link = mockLinks.find((l) => l.id === linkId);
    if (!link) return new HttpResponse(null, { status: 404 });
    const body = (await request.json().catch(() => ({}))) as {
      ask_ai_enabled?: boolean;
      ask_mode?: string;
      ask_ai_monthly_quota?: number;
      clear_ai_quota?: boolean;
    };
    if (body.ask_ai_enabled === true && !link.dealRoomId) {
      return HttpResponse.json(
        { code: "invalid_input", message: "ask_ai_enabled requires a deal-room link" },
        { status: 400 },
      );
    }
    if (typeof body.ask_ai_enabled === "boolean") {
      setLinkAskPolicyOverride(linkId, { askAiEnabled: body.ask_ai_enabled });
    }
    if (typeof body.ask_mode === "string") {
      setLinkAskPolicyOverride(linkId, { askMode: body.ask_mode });
    }
    return HttpResponse.json({
      data: {
        id: link.id,
        ask_mode: resolveLinkAskMode(link),
        ask_ai_enabled: resolveLinkAskAiEnabled(link),
        ask_ai_monthly_quota: null,
        ask_ai_monthly_used: 0,
        ask_ai_monthly_limit: 500,
        ask_ai_quota_exceeded: false,
        ask_ai_entitled: Boolean(link.dealRoomId),
        formal_entitled: Boolean(link.dealRoomId),
      },
    });
  }),

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
    if (doc.category === "agreement") {
      return HttpResponse.json(
        {
          code: "agreement_not_allowed_in_deal_room",
          message: "agreement documents cannot be added to a deal room",
        },
        { status: 400 },
      );
    }
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
    doc.category = "deal_room";
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
    let removedDocId: string | undefined;
    for (const fd of docs) {
      const idx = fd.documents.findIndex((d) => d.id === params.docId);
      if (idx !== -1) {
        removedDocId = fd.documents[idx]?.document_id;
        fd.documents.splice(idx, 1);
        break;
      }
    }
    room.documents = docs.filter((fd) => fd.documents.length > 0);
    if (removedDocId) {
      const stillInAnyRoom = mockDealRooms.some((r) =>
        (r.documents ?? []).some((fd) => fd.documents.some((d) => d.document_id === removedDocId)),
      );
      if (!stillInAnyRoom) {
        const libDoc = mockDocuments.find((d) => d.id === removedDocId);
        if (libDoc?.category === "deal_room") libDoc.category = "general";
      }
    }
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
  http.get("*/api/workspaces/:workspaceSlug/insights/overview", ({ params, request }) => {
    // Mirror backend: tierCounts from link heat; document heat = max linked heat (not views thresholds).
    const url = new URL(request.url);
    const fromParam = url.searchParams.get("from");
    const toParam = url.searchParams.get("to");
    let rangeDays = 7;
    let rangeFrom = "";
    let rangeTo = "";
    let rangeCustom = false;
    if (fromParam && toParam) {
      const fromMs = Date.parse(`${fromParam}T00:00:00Z`);
      const toMs = Date.parse(`${toParam}T00:00:00Z`);
      if (!Number.isNaN(fromMs) && !Number.isNaN(toMs) && toMs >= fromMs) {
        rangeDays = Math.floor((toMs - fromMs) / 86_400_000) + 1;
        if (rangeDays > 90) {
          return HttpResponse.json(
            { code: "invalid_input", message: "insights range exceeds maximum" },
            { status: 400 },
          );
        }
        rangeFrom = fromParam;
        rangeTo = toParam;
        rangeCustom = true;
      }
    }
    if (!rangeCustom) {
      const rawDays = Number(url.searchParams.get("days") ?? "7");
      rangeDays = rawDays === 30 || rawDays === 90 ? rawDays : 7;
      const end = new Date();
      end.setUTCHours(0, 0, 0, 0);
      const start = new Date(end);
      start.setUTCDate(start.getUTCDate() - (rangeDays - 1));
      rangeFrom = start.toISOString().slice(0, 10);
      rangeTo = end.toISOString().slice(0, 10);
    }
    const tierCounts = { hot: 0, warm: 0, cold: 0 } as Record<"hot" | "warm" | "cold", number>;
    for (const l of mockLinks) {
      const level = (l.heatLevel ?? "cold") as "hot" | "warm" | "cold";
      tierCounts[level] = (tierCounts[level] ?? 0) + 1;
    }
    const heatRank = { hot: 3, warm: 2, cold: 1 } as const;
    const topDocuments = mockDocuments
      .map((d) => {
        const linked = mockLinks.filter((l) => l.documentId === d.id);
        const views = linked.reduce((sum, l) => sum + l.accessCount, 0);
        const pages = mockPageAnalytics[d.id];
        let heatLevel: "hot" | "warm" | "cold" = "cold";
        let score = 0;
        let primaryLinkId: string | undefined;
        for (const l of linked) {
          const heat = computeMockLinkHeat(l, "founder", pages);
          if (heat.score > score || (heat.score === score && heatRank[heat.level] > heatRank[heatLevel])) {
            heatLevel = heat.level;
            score = heat.score;
            primaryLinkId = l.id;
          }
        }
        return { id: d.id, title: d.title, views, score, heatLevel, primaryLinkId };
      })
      .filter((d) => mockLinks.some((l) => l.documentId === d.id))
      .sort((a, b) => b.score - a.score || b.views - a.views)
      .slice(0, 5);
    const topLinks = [...mockLinks]
      .map((l) => {
        const pages = l.documentId ? mockPageAnalytics[l.documentId] : undefined;
        const heat = computeMockLinkHeat(l, "founder", pages);
        return { link: l, heat };
      })
      .sort(
        (a, b) =>
          heatRank[b.heat.level] - heatRank[a.heat.level] || b.heat.score - a.heat.score,
      )
      .slice(0, 5)
      .map(({ link: l, heat }) => ({
        id: l.id,
        title: l.documentTitle ?? "",
        documentId: l.documentId,
        shortUrl: l.shortUrl,
        views: l.accessCount,
        score: heat.score,
        heatLevel: heat.level,
      }));
    const seriesStart = new Date(`${rangeFrom}T00:00:00Z`);
    const dailyVisits = Array.from({ length: rangeDays }, (_, i) => {
      const d = new Date(seriesStart);
      d.setUTCDate(d.getUTCDate() + i);
      const isLast = i === rangeDays - 1;
      return {
        date: d.toISOString(),
        opens: isLast ? 1 : 0,
        uniqueVisitors: isLast ? 1 : 0,
      };
    });
    const periodOpens = dailyVisits.reduce((sum, d) => sum + d.opens, 0);
    const periodUniqueVisitors = dailyVisits.reduce((sum, d) => sum + d.uniqueVisitors, 0);
    return HttpResponse.json({
      tierEntity: "link",
      tierCounts,
      activeLinkCount: mockLinks.length,
      rangeDays,
      rangeFrom,
      rangeTo,
      rangeCustom,
      generatedAt: new Date().toISOString(),
      periodOpens,
      previousPeriodOpens: 0,
      periodUniqueVisitors,
      previousPeriodUniqueVisitors: 0,
      periodMedianDurationSeconds: periodOpens > 0 ? 42 : 0,
      previousPeriodMedianDurationSeconds: 0,
      periodAvgDurationSeconds: periodOpens > 0 ? 55 : 0,
      periodPageViewCount: periodOpens > 0 ? 3 : 0,
      periodSessionCount: periodOpens > 0 ? 2 : 0,
      periodMeasurableSessions: periodOpens > 0 ? 2 : 0,
      periodCompletedSessions: periodOpens > 0 ? 1 : 0,
      periodCompletionRate: periodOpens > 0 ? 0.5 : 0,
      previousPeriodSessionCount: 0,
      previousPeriodCompletedSessions: 0,
      previousPeriodCompletionRate: 0,
      openSignalCount: mockSignals.length,
      dailyVisits,
      topDocuments,
      topLinks,
      topContacts: workspaceContacts(params.workspaceSlug)
        .map((c) => ({
          id: c.id,
          email: c.email,
          score: c.score,
          heatLevel: c.heatLevel,
        }))
        .sort((a, b) => b.score - a.score)
        .slice(0, 5),
    });
  }),

  http.get("*/api/workspaces/:workspaceSlug/insights/key-page-settings", ({ request }) => {
    const circle = mockKeyPageSettings.defaultCircle;
    const baseRules = keyPageRulesForCircle(circle, keyPageLangFromRequest(request));
    const extras = mockKeyPageSettings.extraKeywords;
    const byCat = new Map(baseRules.map((r) => [r.category, [...r.keywords]]));
    for (const [cat, kws] of Object.entries(extras)) {
      const cur = byCat.get(cat) ?? [];
      for (const kw of kws) {
        if (!cur.some((x) => x.toLowerCase() === kw.toLowerCase())) cur.push(kw);
      }
      byCat.set(cat, cur);
    }
    const matchRules = [...byCat.entries()]
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([category, keywords]) => ({ category, keywords }));
    return HttpResponse.json({
      defaultCircle: circle,
      extraKeywords: extras,
      builtinRules: baseRules,
      matchRules,
      canEdit: true,
      updatedAt: new Date().toISOString(),
    });
  }),

  http.put("*/api/workspaces/:workspaceSlug/insights/key-page-settings", async ({ request }) => {
    const body = (await request.json()) as {
      defaultCircle?: string;
      extraKeywords?: Record<string, string[]>;
    };
    const circle =
      body.defaultCircle === "investor_ir" || body.defaultCircle === "sales"
        ? body.defaultCircle
        : "founder";
    mockKeyPageSettings = {
      defaultCircle: circle,
      extraKeywords: body.extraKeywords ?? {},
    };
    const baseRules = keyPageRulesForCircle(circle, keyPageLangFromRequest(request));
    const extras = mockKeyPageSettings.extraKeywords;
    const byCat = new Map(baseRules.map((r) => [r.category, [...r.keywords]]));
    for (const [cat, kws] of Object.entries(extras)) {
      const cur = byCat.get(cat) ?? [];
      for (const kw of kws) {
        if (!cur.some((x) => x.toLowerCase() === kw.toLowerCase())) cur.push(kw);
      }
      byCat.set(cat, cur);
    }
    const matchRules = [...byCat.entries()]
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([category, keywords]) => ({ category, keywords }));
    return HttpResponse.json({
      defaultCircle: circle,
      extraKeywords: extras,
      builtinRules: baseRules,
      matchRules,
      canEdit: true,
      updatedAt: new Date().toISOString(),
    });
  }),

  http.get("*/api/workspaces/:workspaceSlug/insights/key-pages", ({ request }) => {
    const url = new URL(request.url);
    const fromParam = url.searchParams.get("from");
    const toParam = url.searchParams.get("to");
    let rangeDays = 30;
    let rangeFrom = "";
    let rangeTo = "";
    let rangeCustom = false;
    if (fromParam && toParam) {
      const fromMs = Date.parse(`${fromParam}T00:00:00Z`);
      const toMs = Date.parse(`${toParam}T00:00:00Z`);
      if (!Number.isNaN(fromMs) && !Number.isNaN(toMs) && toMs >= fromMs) {
        rangeDays = Math.floor((toMs - fromMs) / 86_400_000) + 1;
        if (rangeDays > 90) {
          return HttpResponse.json(
            { code: "invalid_input", message: "insights range exceeds maximum" },
            { status: 400 },
          );
        }
        rangeFrom = fromParam;
        rangeTo = toParam;
        rangeCustom = true;
      }
    }
    if (!rangeCustom) {
      const rawDays = Number(url.searchParams.get("days") ?? "30");
      rangeDays = rawDays === 7 || rawDays === 90 ? rawDays : 30;
      const end = new Date();
      end.setUTCHours(0, 0, 0, 0);
      const start = new Date(end);
      start.setUTCDate(start.getUTCDate() - (rangeDays - 1));
      rangeFrom = start.toISOString().slice(0, 10);
      rangeTo = end.toISOString().slice(0, 10);
    }
    const circleRaw = url.searchParams.get("circle") ?? "founder";
    const circle =
      circleRaw === "investor_ir" || circleRaw === "sales" ? circleRaw : "founder";
    const baseRules = keyPageRulesForCircle(circle, keyPageLangFromRequest(request));
    const extras = mockKeyPageSettings.extraKeywords;
    const byCat = new Map(baseRules.map((r) => [r.category, [...r.keywords]]));
    for (const [cat, kws] of Object.entries(extras)) {
      const cur = byCat.get(cat) ?? [];
      for (const kw of kws) {
        if (!cur.some((x) => x.toLowerCase() === kw.toLowerCase())) cur.push(kw);
      }
      byCat.set(cat, cur);
    }
    const matchRules = [...byCat.entries()]
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([category, keywords]) => ({ category, keywords }));
    const limit = Math.min(100, Math.max(1, Number(url.searchParams.get("limit") ?? "25") || 25));
    const offset = Math.max(0, Number(url.searchParams.get("offset") ?? "0") || 0);
    const events = [
      {
        id: "kpv-mock-1",
        linkId: mockLinks[0]?.id,
        visitorId: "v-mock-1",
        visitorEmail: "buyer@example.com",
        documentId: mockDocuments[0]?.id,
        documentTitle: mockDocuments[0]?.title ?? "Pitch Deck",
        pageNumber: 4,
        pageTitle: "Financial Projections",
        category: "financials",
        durationSeconds: 42,
        createdAt: new Date().toISOString(),
        dealRoomId: "dr-mock-1",
        dealRoomName: "Series A Room",
      },
    ];
    const page = events.slice(offset, offset + limit + 1);
    const hasMore = page.length > limit;
    return HttpResponse.json({
      rangeDays,
      rangeFrom,
      rangeTo,
      rangeCustom,
      circle,
      generatedAt: new Date().toISOString(),
      totalViews: 1,
      engagedViews: 1,
      uniqueVisitors: 1,
      distinctPages: 1,
      matchRules,
      byCategory: [{ category: "financials", count: 1 }],
      pages: [
        {
          documentId: mockDocuments[0]?.id,
          documentTitle: mockDocuments[0]?.title ?? "Pitch Deck",
          pageNumber: 4,
          pageTitle: "Financial Projections",
          category: "financials",
          views: 1,
          uniqueVisitors: 1,
          avgDurationSeconds: 42,
          lastViewedAt: new Date().toISOString(),
        },
      ],
      events: hasMore ? page.slice(0, limit) : page,
      hasMore,
      limit,
      offset,
    });
  }),

  http.get("*/api/workspaces/:workspaceSlug/insights/access-audit", ({ request }) => {
    const url = new URL(request.url);
    const fromParam = url.searchParams.get("from");
    const toParam = url.searchParams.get("to");
    let rangeDays = 30;
    let rangeFrom = "";
    let rangeTo = "";
    let rangeCustom = false;
    if (fromParam && toParam) {
      const fromMs = Date.parse(`${fromParam}T00:00:00Z`);
      const toMs = Date.parse(`${toParam}T00:00:00Z`);
      if (!Number.isNaN(fromMs) && !Number.isNaN(toMs) && toMs >= fromMs) {
        rangeDays = Math.floor((toMs - fromMs) / 86_400_000) + 1;
        if (rangeDays > 90) {
          return HttpResponse.json(
            { code: "invalid_input", message: "insights range exceeds maximum" },
            { status: 400 },
          );
        }
        rangeFrom = fromParam;
        rangeTo = toParam;
        rangeCustom = true;
      }
    }
    if (!rangeCustom) {
      const rawDays = Number(url.searchParams.get("days") ?? "30");
      rangeDays = rawDays === 7 || rawDays === 90 ? rawDays : 30;
      const end = new Date();
      end.setUTCHours(0, 0, 0, 0);
      const start = new Date(end);
      start.setUTCDate(start.getUTCDate() - (rangeDays - 1));
      rangeFrom = start.toISOString().slice(0, 10);
      rangeTo = end.toISOString().slice(0, 10);
    }
    const eventTypeFilter = url.searchParams.get("eventType") ?? "";
    const dealRoomFilter = url.searchParams.get("dealRoomId") ?? "";
    const memberFilter = url.searchParams.get("memberId") ?? "";
    const folderFilter = url.searchParams.get("folderPath") ?? "";
    const limit = Math.min(100, Math.max(1, Number(url.searchParams.get("limit") ?? "25") || 25));
    const offset = Math.max(0, Number(url.searchParams.get("offset") ?? "0") || 0);
    const allEvents = [
      {
        id: "se-mock-1",
        linkId: mockLinks[0]?.id,
        eventType: "invalid_password",
        email: "buyer@example.com",
        visitorId: "v-mock-1",
        reason: "password mismatch",
        createdAt: new Date().toISOString(),
        documentTitle: mockDocuments[0]?.title ?? "Pitch Deck",
        dealRoomId: "dr-mock-1",
        dealRoomName: "Series A Room",
        memberId: "member-mock-1",
        memberEmail: "owner@example.com",
        folderPath: "Finance",
      },
      {
        id: "se-mock-2",
        linkId: mockLinks[1]?.id ?? mockLinks[0]?.id,
        eventType: "blocked_email",
        email: "blocked@competitor.com",
        reason: "email deny list",
        createdAt: new Date(Date.now() - 3600_000).toISOString(),
        documentTitle: mockDocuments[0]?.title ?? "Pitch Deck",
        dealRoomName: "",
        memberId: "member-mock-1",
        memberEmail: "owner@example.com",
        folderPath: "",
      },
    ];
    const filtered = allEvents.filter((e) => {
      if (eventTypeFilter && e.eventType !== eventTypeFilter) return false;
      if (dealRoomFilter && e.dealRoomId !== dealRoomFilter) return false;
      if (memberFilter && e.memberId !== memberFilter) return false;
      if (folderFilter && e.folderPath !== folderFilter) return false;
      return true;
    });
    const page = filtered.slice(offset, offset + limit + 1);
    const hasMore = page.length > limit;
    const events = hasMore ? page.slice(0, limit) : page;
    return HttpResponse.json({
      rangeDays,
      rangeFrom,
      rangeTo,
      rangeCustom,
      generatedAt: new Date().toISOString(),
      totalEvents: filtered.length,
      byType: [
        { eventType: "invalid_password", count: 1 },
        { eventType: "blocked_email", count: 1 },
      ].filter((b) => !eventTypeFilter || b.eventType === eventTypeFilter),
      byDealRoom: [
        { dealRoomId: "dr-mock-1", dealRoomName: "Series A Room", count: 1 },
        { dealRoomId: null, dealRoomName: "", count: 1, scope: "library" },
      ],
      byMember: [
        { memberId: "member-mock-1", memberEmail: "owner@example.com", count: 2 },
      ].filter((b) => !memberFilter || b.memberId === memberFilter),
      byFolder: [
        {
          folderPath: "Finance",
          dealRoomId: "dr-mock-1",
          dealRoomName: "Series A Room",
          count: 1,
        },
        {
          folderPath: "",
          dealRoomId: null,
          dealRoomName: "",
          count: 1,
          scope: "root",
        },
      ].filter((b) => !folderFilter || b.folderPath === folderFilter),
      events,
      hasMore,
      limit,
      offset,
    });
  }),

  http.get("*/api/workspaces/:workspaceSlug/insights/overview/export", ({ request }) => {
    const url = new URL(request.url);
    const rawDays = Number(url.searchParams.get("days") ?? "7");
    const rangeDays = rawDays === 30 || rawDays === 90 ? rawDays : 7;
    const lines = ["date,opens,unique_visitors"];
    for (let i = 0; i < rangeDays; i++) {
      const d = new Date();
      d.setUTCHours(0, 0, 0, 0);
      d.setUTCDate(d.getUTCDate() - (rangeDays - 1 - i));
      const isToday = i === rangeDays - 1;
      lines.push(`${d.toISOString().slice(0, 10)},${isToday ? 1 : 0},${isToday ? 1 : 0}`);
    }
    return new HttpResponse(`${lines.join("\n")}\n`, {
      status: 200,
      headers: {
        "Content-Type": "text/csv; charset=utf-8",
        "Content-Disposition": `attachment; filename="insights-daily-${rangeDays}d.csv"`,
      },
    });
  }),

  http.get("*/api/workspaces/:workspaceSlug/analytics/links/:linkId/score", ({ request, params }) => {
    const url = new URL(request.url);
    const circleRaw = url.searchParams.get("circle") ?? "founder";
    const circle = (
      circleRaw === "investor_ir" || circleRaw === "sales" ? circleRaw : "founder"
    ) as Circle;
    const link = mockLinks.find((l) => l.id === params.linkId);
    if (!link) {
      return HttpResponse.json(
        { code: "not_found", message: "link not found" },
        { status: 404 },
      );
    }
    const pages = link.documentId ? mockPageAnalytics[link.documentId] : undefined;
    const heat = computeMockLinkHeat(link, circle, pages);
    return HttpResponse.json({
      linkId: params.linkId,
      score: heat.score,
      level: heat.level,
      trend: heat.trend,
      breakdown: heat.breakdown,
      updatedAt: new Date().toISOString(),
    });
  }),

  http.get("*/api/workspaces/:workspaceSlug/insights/pages/:documentId", ({ request, params }) => {
    const url = new URL(request.url);
    const hasRange =
      url.searchParams.has("days") ||
      (url.searchParams.has("from") && url.searchParams.has("to"));
    return HttpResponse.json({
      data: mockPageAnalytics[params.documentId as string] || [],
      ...(hasRange
        ? {
            rangeDays: Number(url.searchParams.get("days") || 30),
            rangeFrom: url.searchParams.get("from") || undefined,
            rangeTo: url.searchParams.get("to") || undefined,
            rangeCustom: url.searchParams.has("from"),
          }
        : { lifetime: true }),
    });
  }),

  http.get("*/api/workspaces/:workspaceSlug/insights/documents/:documentId/visitors", ({ request, params }) => {
    const url = new URL(request.url);
    const hasRange =
      url.searchParams.has("days") ||
      (url.searchParams.has("from") && url.searchParams.has("to"));
    return HttpResponse.json({
      data: mockDocumentVisitors[params.documentId as string] || [],
      ...(hasRange
        ? {
            rangeDays: Number(url.searchParams.get("days") || 30),
            rangeFrom: url.searchParams.get("from") || undefined,
            rangeTo: url.searchParams.get("to") || undefined,
            rangeCustom: url.searchParams.has("from"),
          }
        : { lifetime: true }),
    });
  }),

  http.get("*/api/workspaces/:workspaceSlug/insights/documents/:documentId/sessions", ({ params }) => {
    const documentId = String(params.documentId ?? "");
    const visitors = mockDocumentVisitors[documentId] || [];
    const pageCount =
      mockDocuments.find((d) => d.id === documentId)?.pageCount ||
      mockPageAnalytics[documentId]?.length ||
      0;
    const sessions = visitors.map((v, idx) => {
      const maxPage = Math.min(pageCount || 1, Math.max(1, Math.round(v.pageViewCount / 2) || 1));
      const pages = Array.from({ length: maxPage }, (_, i) => ({
        pageNumber: i + 1,
        durationSeconds: Math.max(5, Math.round(v.avgDurationSeconds / Math.max(1, maxPage))),
      }));
      return {
        id: `sess-${documentId}-${idx}`,
        linkId: mockLinks.find((l) => l.documentId === documentId)?.id ?? `link-${idx}`,
        visitorId: v.visitorId,
        visitorEmail: v.visitorEmail,
        startedAt: v.lastSeenAt,
        lastActivityAt: v.lastSeenAt,
        maxPage,
        distinctPageCount: pages.length,
        totalDurationSeconds: Math.round(v.avgDurationSeconds * Math.max(1, v.pageViewCount)),
        completed: pageCount > 0 && maxPage >= pageCount,
        pages,
      };
    });
    return HttpResponse.json({
      documentId,
      pageCount,
      sessionModel: "reading_session",
      sessions,
    });
  }),

  http.get("*/api/workspaces/:workspaceSlug/insights/documents/:documentId/funnel", ({ params }) => {
    const documentId = String(params.documentId ?? "");
    const visitors = mockDocumentVisitors[documentId] || [];
    const pageAnalytics = mockPageAnalytics[documentId] || [];
    const pageCount =
      mockDocuments.find((d) => d.id === documentId)?.pageCount ||
      pageAnalytics.length ||
      0;
    const sessionCount = visitors.length;
    if (sessionCount === 0 || pageCount === 0) {
      return HttpResponse.json({
        documentId,
        pageCount,
        sessionCount: 0,
        completedSessions: 0,
        completionRate: 0,
        medianMaxPage: 0,
        avgPagesPerSession: 0,
        avgDurationSeconds: 0,
        biggestDropOffPage: 0,
        steps: [],
        sessionModel: "reading_session",
      });
    }
    // Deterministic mock depths from visitor pageViewCount.
    const depths = visitors.map((v) =>
      Math.min(pageCount, Math.max(1, Math.round(v.pageViewCount / 2) || 1)),
    );
    const completedSessions = depths.filter((d) => d >= pageCount).length;
    const steps = Array.from({ length: pageCount }, (_, i) => {
      const pageNumber = i + 1;
      const visitorsReached = depths.filter((d) => d >= pageNumber).length;
      const prev = i === 0 ? visitorsReached : depths.filter((d) => d >= i).length;
      return {
        pageNumber,
        visitorsReached,
        dropOffFromPrev: i === 0 || prev === 0 ? 0 : 1 - visitorsReached / prev,
      };
    });
    let biggestDropOffPage = 0;
    let biggestAbs = -1;
    for (let i = 1; i < steps.length; i++) {
      const abs = steps[i - 1]!.visitorsReached - steps[i]!.visitorsReached;
      if (abs > biggestAbs) {
        biggestAbs = abs;
        biggestDropOffPage = steps[i]!.pageNumber;
      }
    }
    const sorted = [...depths].sort((a, b) => a - b);
    const mid = Math.floor(sorted.length / 2);
    const medianMaxPage =
      sorted.length % 2 === 1
        ? sorted[mid]!
        : (sorted[mid - 1]! + sorted[mid]!) / 2;
    return HttpResponse.json({
      documentId,
      pageCount,
      sessionCount,
      completedSessions,
      completionRate: completedSessions / sessionCount,
      medianMaxPage,
      avgPagesPerSession:
        depths.reduce((s, d) => s + d, 0) / sessionCount,
      avgDurationSeconds:
        visitors.reduce((s, v) => s + v.avgDurationSeconds, 0) / sessionCount,
      biggestDropOffPage,
      steps,
      sessionModel: "reading_session",
    });
  }),

  http.get("*/api/workspaces/:workspaceSlug/insights/suggestions", () => {
    return HttpResponse.json({ data: mockSuggestions });
  }),

  http.post(
    "*/api/workspaces/:workspaceSlug/analytics/links/:linkId/suggestions/:id/dismiss",
    ({ params }) => {
      const id = String(params.id ?? "");
      const idx = mockSuggestions.findIndex((s) => s.id === id);
      if (idx < 0) {
        return HttpResponse.json(
          { code: "not_found", message: "Suggestion not found" },
          { status: 404 },
        );
      }
      mockSuggestions.splice(idx, 1);
      return new HttpResponse(null, { status: 204 });
    },
  ),

  http.post(
    "*/api/workspaces/:workspaceSlug/insights/suggestions/:id/snooze",
    async ({ params, request }) => {
      const id = String(params.id ?? "");
      const body = (await request.json().catch(() => ({}))) as { hours?: number };
      const hours = body.hours === 72 || body.hours === 168 ? body.hours : 24;
      const idx = mockSuggestions.findIndex((s) => s.id === id);
      if (idx < 0) {
        return HttpResponse.json(
          { code: "not_found", message: "Suggestion not found" },
          { status: 404 },
        );
      }
      mockSuggestions.splice(idx, 1);
      const until = new Date(Date.now() + hours * 3600_000).toISOString();
      return HttpResponse.json({ id, snoozed_until: until });
    },
  ),

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
    return HttpResponse.json(integrationsStatus);
  }),

  http.put("*/api/workspaces/:workspaceSlug/integrations/settings", async ({ request }) => {
    const body = (await request.json()) as Partial<typeof integrationsStatus>;
    integrationsStatus = { ...integrationsStatus, ...body };
    return HttpResponse.json(integrationsStatus);
  }),

  http.post("*/api/workspaces/:workspaceSlug/integrations/slack/connect", () => {
    return HttpResponse.json({ url: "https://slack.com/oauth/v2/authorize?client_id=mock" });
  }),

  http.post("*/api/workspaces/:workspaceSlug/integrations/slack/disconnect", () => {
    integrationsStatus.slack_connected = false;
    return HttpResponse.json({ code: "ok", message: "disconnected" });
  }),

  http.post("*/api/workspaces/:workspaceSlug/integrations/hubspot/connect", () => {
    return HttpResponse.json({ url: "https://app.hubspot.com/oauth/authorize?client_id=mock" });
  }),

  http.post("*/api/workspaces/:workspaceSlug/integrations/hubspot/disconnect", () => {
    integrationsStatus.hubspot_connected = false;
    return HttpResponse.json({ code: "ok", message: "disconnected" });
  }),

  http.get("*/api/workspaces/:workspaceSlug/integrations/webhook", () => {
    const { secret: _secret, ...publicView } = outboundWebhook;
    return HttpResponse.json({
      configured: publicView.configured,
      enabled: publicView.enabled,
      url: publicView.url || undefined,
      event_types: publicView.event_types,
      secret_hint: publicView.secret_hint || undefined,
      updated_at: publicView.updated_at || undefined,
    });
  }),

  http.put("*/api/workspaces/:workspaceSlug/integrations/webhook", async ({ request }) => {
    const body = (await request.json()) as {
      url?: string;
      enabled?: boolean;
      event_types?: string[];
      rotate_secret?: boolean;
    };
    const url = (body.url ?? "").trim();
    if (!url.startsWith("https://") && !url.startsWith("http://localhost") && !url.startsWith("http://127.0.0.1")) {
      return HttpResponse.json({ code: "invalid_input", message: "webhook url must use https" }, { status: 400 });
    }
    const rotate = Boolean(body.rotate_secret) || !outboundWebhook.configured;
    const secret = rotate
      ? `mocksecret${Math.random().toString(16).slice(2)}${Math.random().toString(16).slice(2)}`
      : outboundWebhook.secret;
    outboundWebhook = {
      configured: true,
      enabled: Boolean(body.enabled),
      url,
      event_types: body.event_types?.length ? body.event_types : ["key_page", "repeat_key_page"],
      secret_hint: `••••${secret.slice(-4)}`,
      secret,
      updated_at: new Date().toISOString(),
    };
    return HttpResponse.json({
      configured: true,
      enabled: outboundWebhook.enabled,
      url: outboundWebhook.url,
      event_types: outboundWebhook.event_types,
      secret_hint: outboundWebhook.secret_hint,
      secret: rotate ? secret : undefined,
      updated_at: outboundWebhook.updated_at,
    });
  }),

  http.delete("*/api/workspaces/:workspaceSlug/integrations/webhook", () => {
    outboundWebhook = {
      configured: false,
      enabled: false,
      url: "",
      event_types: ["key_page", "repeat_key_page"],
      secret_hint: "",
      secret: "",
      updated_at: "",
    };
    return HttpResponse.json({ code: "ok", message: "webhook deleted" });
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

  // Deal Radar — compiled feed (fixture-derived, same sources as /signals)
  http.get("*/api/workspaces/:workspaceSlug/radar", ({ params, request }) => {
    const slug = String(params.workspaceSlug || "acme-capital");
    const circle = new URL(request.url).searchParams.get("circle") || "founder";
    const feed = getMockRadarFeed(slug);
    return HttpResponse.json({
      ...feed,
      lens:
        circle === "investor_ir" || circle === "sales" ? circle : "founder",
    });
  }),

  http.get(
    "*/api/workspaces/:workspaceSlug/radar/items/:id/evidence",
    ({ params }) => {
      const slug = String(params.workspaceSlug || "acme-capital");
      const pack = getMockRadarEvidence(String(params.id), slug);
      if (!pack) return new HttpResponse(null, { status: 404 });
      return HttpResponse.json(pack);
    },
  ),

  http.patch(
    "*/api/workspaces/:workspaceSlug/radar/items/:id",
    async ({ params, request }) => {
      const body = (await request.json()) as {
        status?: string;
        snooze_hours?: number;
        outcome?: ActionItem["outcome"];
      };
      const action = mockActionItems.find((a) => a.id === params.id);
      if (!action) return new HttpResponse(null, { status: 404 });
      if (body?.status) action.status = body.status as ActionItem["status"];
      if (body?.status === "snoozed" && body.snooze_hours) {
        action.snoozedUntil = new Date(
          Date.now() + body.snooze_hours * 3600_000,
        ).toISOString();
      }
      if (body?.status === "done") {
        action.outcome = body.outcome || "acted";
      } else if (body?.status) {
        delete action.outcome;
      }
      return HttpResponse.json(action);
    },
  ),

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

    const doc = mockDocuments.find((d) => d.id === extended.documentId) ?? mockDocuments[0];
    const publicDocument = {
      id: doc.id,
      title: doc.title,
      pageCount: doc.pageCount,
      status: doc.status,
      sourceType: doc.fileType,
      fileSize: doc.fileSize,
    };
    const watermarkEnabled = Boolean(extended.watermarkEnabled);
    const visitorEmail = typeof body.email === "string" && body.email.trim()
      ? body.email.trim()
      : "Guest";
    return HttpResponse.json({
      link: {
        id: extended.id,
        name: extended.documentTitle,
        documentId: extended.documentId,
        permissionType: extended.permissionType ?? "public",
        downloadEnabled: true,
        watermarkEnabled,
        watermarkText: watermarkEnabled
          ? `${visitorEmail} | ${new Date().toISOString().slice(0, 19).replace("T", " ")} UTC | IP:mswmock1`
          : undefined,
        qaEnabled: Boolean(extended.qaEnabled),
        visitorAskUnified: Boolean(extended.qaEnabled && (extended.dealRoomId || extended.visitorAskUnified)),
        askMode: extended.askMode ?? "supervised",
        askAiEnabled: Boolean(extended.askAiEnabled),
        fileRequestsEnabled: Boolean(extended.fileRequestsEnabled),
        isBundle: Boolean(extended.isBundle),
        dealRoomId: extended.dealRoomId,
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

  http.get("*/api/v1/public/links/:token/ask/me", async ({ params }) => {
    await hydrateVisitorAskState();
    const token = params.token as string;
    return HttpResponse.json({ data: mockPublicAskTurns.get(token) ?? [] });
  }),

  http.get("*/api/v1/public/links/:token/ask/faq", async ({ params }) => {
    await hydrateVisitorAskState();
    const token = params.token as string;
    return HttpResponse.json({ data: mockPublicAskFAQsForToken(token) });
  }),

  http.get("*/api/v1/public/links/:token/ask/formal", async ({ params }) => {
    await hydrateVisitorAskState();
    const token = params.token as string;
    return HttpResponse.json({ data: mockPublicFormalAsk.get(token) ?? [] });
  }),

  http.post("*/api/v1/public/links/:token/ask", async ({ params, request }) => {
    await hydrateVisitorAskState();
    const token = params.token as string;
    const body = (await request.json().catch(() => ({}))) as {
      question?: string;
      escalate?: boolean;
    };
    const question = (body.question ?? "").trim();
    if (!question) {
      return HttpResponse.json({ code: "invalid_request", message: "question required" }, { status: 400 });
    }
    const lower = question.toLowerCase();
    if (lower.includes("__rate_limit__")) {
      return HttpResponse.json(
        { code: "rate_limit_exceeded", message: "too many Ask requests, please try again later" },
        { status: 429 },
      );
    }
    if (lower.includes("__limiter_down__")) {
      return HttpResponse.json(
        {
          code: "limiter_unavailable",
          message: "Ask is temporarily unavailable, please try again later",
        },
        { status: 503 },
      );
    }
    const sessionId = `sess_${token}`;
    const link = findMockLinkByPublicToken(token);
    const forceAI = lower.includes("__ai__");
    const forceHost = lower.includes("__host__");
    const askMode = resolveLinkAskMode(link);
    const isFormal = askMode === "formal" && !forceAI && !body.escalate;
    const aiEnabled = resolveLinkAskAiEnabled(link);
    const isAIAsk =
      forceAI || (!forceHost && aiEnabled && !isFormal && !body.escalate);
    const isSlowStream = lower.includes("__slow__");
    const cleanedQuestion =
      question
        .replace(/__ai__/gi, "")
        .replace(/__host__/gi, "")
        .replace(/__slow__/gi, "")
        .trim() || question;
    const turn: PublicAskTurn = isAIAsk
      ? {
          id: generateId("turn"),
          session_id: sessionId,
          question: cleanedQuestion,
          lane: "ai",
          status: "ai_streaming",
          route_reason: isSlowStream ? "slow_e2e" : "ai_lane_pending",
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        }
      : {
          id: generateId("turn"),
          session_id: sessionId,
          question: forceAI ? cleanedQuestion : question,
          lane: "host",
          status: "host_pending",
          route_reason: isFormal
            ? "policy_formal"
            : body.escalate
              ? "user_escalate"
              : forceAI
                ? "ai_lane_pending"
                : "unified_ask",
          formal_status: isFormal ? "pending_review" : undefined,
          formal_anonymize: isFormal ? true : undefined,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        };
    const list = mockPublicAskTurns.get(token) ?? [];
    list.push(turn);
    mockPublicAskTurns.set(token, list);
    await persistVisitorAskState();
    return HttpResponse.json({ data: turn }, { status: 201 });
  }),

  http.post("*/api/v1/public/links/:token/ask/:turnId/escalate", async ({ params }) => {
    await hydrateVisitorAskState();
    const token = params.token as string;
    const turnId = params.turnId as string;
    const list = mockPublicAskTurns.get(token) ?? [];
    const idx = list.findIndex((t) => t.id === turnId);
    if (idx < 0) {
      return HttpResponse.json({ code: "not_found", message: "ask turn not found" }, { status: 404 });
    }
    const turn = list[idx]!;
    if (turn.lane !== "ai" || (turn.status !== "ai_refused" && turn.status !== "ai_answered")) {
      return HttpResponse.json(
        { code: "ask_turn_not_escalatable", message: "ask turn cannot be escalated" },
        { status: 409 },
      );
    }
    if (turn.status === "host_escalated") {
      return HttpResponse.json({ data: turn });
    }
    const updated: PublicAskTurn = {
      ...turn,
      lane: "hybrid",
      status: "host_escalated",
      route_reason: "user_escalate",
      updated_at: new Date().toISOString(),
    };
    list[idx] = updated;
    mockPublicAskTurns.set(token, list);
    await persistVisitorAskState();
    return HttpResponse.json({ data: updated });
  }),

  http.get("*/api/v1/public/links/:token/ask/:turnId/stream", async ({ params }) => {
    await hydrateVisitorAskState();
    const token = params.token as string;
    const turnId = params.turnId as string;
    const turns = mockPublicAskTurns.get(token) ?? [];
    const turn = turns.find((row) => row.id === turnId);
    if (!turn || turn.lane !== "ai") {
      return HttpResponse.json({ code: "not_found", message: "ask turn not found" }, { status: 404 });
    }
    const answer =
      "Based on the authorized materials, revenue grew 12% year over year [1].";
    const hits = [
      {
        chunkId: "chunk_ai_1",
        documentId: "doc_1",
        text: "Revenue increased 12% YoY in FY2024.",
        score: 0.91,
        sourceName: "Financial Summary.pdf",
        pages: [3],
        viewerPage: 3,
      },
    ];
    const payload = {
      turn: {
        id: turn.id,
        question: turn.question,
        answer,
        refused: false,
        resultStatus: "answered",
        hits,
      },
      query: turn.question,
      answer,
      results: hits,
      refused: false,
      resultStatus: "answered",
    };
    turn.status = "ai_answered";
    turn.ai_payload = {
      answer,
      refused: false,
      resultStatus: "answered",
      hits,
    };
    turn.updated_at = new Date().toISOString();
    syncOwnerAskAITurnFromPublic(token, turn);
    await persistVisitorAskState();
    const tokenFrames = chunkMockAnswerTokens(answer, 36)
      .map((text) => `event: token\ndata: ${JSON.stringify({ text })}\n\n`)
      .join("");
    const sseHeaders = {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache",
    };
    if (turn.route_reason === "slow_e2e") {
      const stream = new ReadableStream({
        async start(controller) {
          const enc = new TextEncoder();
          const send = (chunk: string) => controller.enqueue(enc.encode(chunk));
          send(`event: phase\ndata: ${JSON.stringify({ phase: "retrieving" })}\n\n`);
          await new Promise((resolve) => setTimeout(resolve, 2500));
          send(`event: phase\ndata: ${JSON.stringify({ phase: "generating" })}\n\n`);
          send(`event: sources\ndata: ${JSON.stringify({ results: hits, grounded: true })}\n\n`);
          for (const frame of chunkMockAnswerTokens(answer, 36)) {
            send(`event: token\ndata: ${JSON.stringify({ text: frame })}\n\n`);
            await new Promise((resolve) => setTimeout(resolve, 400));
          }
          send(`event: done\ndata: ${JSON.stringify(payload)}\n\n`);
          controller.close();
        },
      });
      return new HttpResponse(stream, { status: 200, headers: sseHeaders });
    }
    const frames = [
      `event: phase\ndata: ${JSON.stringify({ phase: "retrieving" })}\n\n`,
      `event: phase\ndata: ${JSON.stringify({ phase: "generating" })}\n\n`,
      `event: sources\ndata: ${JSON.stringify({ results: hits, grounded: true })}\n\n`,
      tokenFrames,
      `event: done\ndata: ${JSON.stringify(payload)}\n\n`,
    ].join("");
    return new HttpResponse(frames, {
      status: 200,
      headers: sseHeaders,
    });
  }),

  http.get("*/api/v1/public/documents/:documentId/pages", ({ params }) => {
    const doc = mockDocuments.find((d) => d.id === params.documentId);
    if (!doc) return new HttpResponse(null, { status: 404 });
    if (doc.status === "archived") {
      return HttpResponse.json(
        { code: "access_denied", message: "document access denied" },
        { status: 403 },
      );
    }
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
    if (doc.status === "archived") {
      return HttpResponse.json(
        { code: "access_denied", message: "document access denied" },
        { status: 403 },
      );
    }
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
    if (doc.status === "archived") {
      return HttpResponse.json(
        { code: "access_denied", message: "document access denied" },
        { status: 403 },
      );
    }
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
