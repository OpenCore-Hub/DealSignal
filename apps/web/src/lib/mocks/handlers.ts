import { http, HttpResponse } from "msw";
import type {
  ActionItem,
  AskDocsAuditDetail,
  AskDocsAuditEntry,
  Contact,
  DealRoom,
  DealRoomDocumentItem,
  DealRoomFolder,
  DealRoomFolderDocs,
  DealRoomKnowledgeBase,
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

function resetMockState() {
  mockUsers.clear();
  mockPublicQuestions.clear();
  mockOwnerQuestions.clear();
  mockAskDocsBurst.clear();
  mockAskDocsAuditByLink.clear();
  mockAskDocsAuditDetails.clear();
  mockKnowledgeBases.clear();
  mockDDCoverageRuns.clear();
  mockDDCoverageRunsById.clear();
  mockDDCoverageSnapshots.clear();
  mockDDCoveragePacks.clear();
  mockDDCrossChecks.clear();
  mockDDPortfolioViews.clear();
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
/** Per-link Ask Docs burst counters for rate-limit e2e. */
const mockAskDocsBurst = new Map<string, number>();
/** Per-link Ask Docs audit list rows (Owner analytics). */
const mockAskDocsAuditByLink = new Map<string, AskDocsAuditEntry[]>();
/** Keyed by `${linkId}:${sessionId}` for Ask Docs audit detail. */
const mockAskDocsAuditDetails = new Map<string, AskDocsAuditDetail>();
/** Per-room knowledge base state for owner KB e2e. */
const mockKnowledgeBases = new Map<string, DealRoomKnowledgeBase>();
/** DD Coverage runs keyed by `${roomId}:${linkId|room}`. */
type MockDDCoverageRun = {
  id: string;
  pack_id: string;
  pack_version: string;
  scope: "room" | "link";
  link_id?: string;
  status: "queued" | "running" | "succeeded" | "failed";
  triggered_by: string;
  created_at: string;
  started_at?: string;
  finished_at?: string;
};
const mockDDCoverageRuns = new Map<string, MockDDCoverageRun>();
const mockDDCoverageRunsById = new Map<string, { key: string; run: MockDDCoverageRun }>();
const mockDDCoverageSnapshots = new Map<string, Record<string, unknown>>();
const mockDDCoveragePacks = new Map<
  string,
  {
    pack_id: string;
    pack_version: string;
    base_pack_id: string;
    forked: boolean;
    fork_revision?: number;
    items: Array<{
      id: string;
      label_en: string;
      label_zh: string;
      query_en: string;
      query_zh: string;
      value_type?: string;
    }>;
  }
>();

const builtinDDPack = {
  pack_id: "financing_dd_v1",
  pack_version: "1",
  base_pack_id: "financing_dd_v1",
  forked: false,
  items: [
    {
      id: "cap_table",
      label_en: "Cap table",
      label_zh: "股权结构表 / Cap table",
      query_en: "capitalization table cap table share ownership",
      query_zh: "股权结构表 股本结构 cap table 持股",
    },
    {
      id: "option_pool",
      label_en: "Option / ESOP pool",
      label_zh: "期权池 / ESOP",
      query_en: "option pool ESOP equity incentive plan percentage",
      query_zh: "期权池 ESOP 员工激励 池比例",
      value_type: "percent",
    },
  ],
};

const builtinMAPack = {
  pack_id: "ma_redflag_v1",
  pack_version: "1",
  base_pack_id: "ma_redflag_v1",
  forked: false,
  items: [
    {
      id: "indemnity_cap",
      label_en: "Indemnity / liability cap",
      label_zh: "赔偿上限 / 责任上限",
      query_en: "indemnity cap liability basket survival",
      query_zh: "赔偿上限 责任上限 一揽子 存续期",
    },
  ],
};

const mockDDCrossChecks = new Map<string, Record<string, unknown>>();

const mockDDPortfolioViews = new Map<
  string,
  {
    id: string;
    name: string;
    pack_id: string;
    room_ids: string[];
    created_by: string;
    created_at: string;
    updated_at: string;
  }
>();

type AskDocsChatPayload = {
  session_id: string;
  answer: string;
  evidence: Array<{
    chunk_id: string;
    document_id: string;
    quote: string;
    page_number: number;
    boxes: Array<{ x: number; y: number; w: number; h: number }>;
    score: number;
  }>;
  result_status: string;
  suggest_ask_host: boolean;
};

/** MSW fixtures mirroring Intent-first P0 DocIntent routing (no doc_intent in chat API). */
function mockAskDocsIntentFirstResponse(
  message: string,
  sessionId: string,
  link: Link,
): AskDocsChatPayload | null {
  const trimmed = message.trim();
  const lower = trimmed.toLowerCase();
  const docId = link.documentId ?? "doc_1";
  const evidence = [
    {
      chunk_id: "chk_ask_docs_intent_001",
      document_id: docId,
      quote: "Revenue grew 3x year over year.",
      page_number: 1,
      boxes: [{ x: 0.12, y: 0.34, w: 0.45, h: 0.06 }],
      score: 0.92,
    },
  ];

  const refuse =
    lower.includes("__intent_refuse__") ||
    trimmed.includes("市场惯例") ||
    lower.includes("investment advice") ||
    trimmed.includes("投资建议");
  if (refuse) {
    return {
      session_id: sessionId,
      answer:
        "This question is outside what the authorized materials can support (for example market practice or external legal advice), so I will not invent an answer. You can ask the host instead.",
      evidence: [],
      result_status: "out_of_corpus",
      suggest_ask_host: Boolean(link.qaEnabled),
    };
  }

  // Locate paste with no strong literal → degrade to topic (≤3 clues); audit fallback is server-side only.
  if (lower.includes("__intent_locate_fallback__")) {
    const topicHits = [
      evidence[0],
      {
        chunk_id: "chk_ask_docs_intent_002",
        document_id: docId,
        quote: "Gross margin remained stable across quarters.",
        page_number: 2,
        boxes: [{ x: 0.1, y: 0.2, w: 0.4, h: 0.05 }],
        score: 0.81,
      },
      {
        chunk_id: "chk_ask_docs_intent_003",
        document_id: docId,
        quote: "Operating expenses tracked to plan.",
        page_number: 3,
        boxes: [{ x: 0.1, y: 0.25, w: 0.4, h: 0.05 }],
        score: 0.74,
      },
    ];
    return {
      session_id: sessionId,
      answer: `Found these related excerpts in the authorized materials:\n\n1. Document ${docId.slice(0, 8)} · page 1\n"${topicHits[0].quote}"\n\n2. Document ${docId.slice(0, 8)} · page 2\n"${topicHits[1].quote}"\n\n3. Document ${docId.slice(0, 8)} · page 3\n"${topicHits[2].quote}"`,
      evidence: topicHits,
      result_status: "success",
      suggest_ask_host: false,
    };
  }

  const locate =
    lower.includes("__intent_locate__") ||
    /第\s*\d+\s*条/.test(trimmed) ||
    trimmed.length >= 80;
  if (locate && !lower.includes("__intent_topic__")) {
    return {
      session_id: sessionId,
      answer: `Located the following excerpt:\n\n1. Document ${docId.slice(0, 8)} · page 1\n"${evidence[0].quote}"`,
      evidence: evidence.slice(0, 1),
      result_status: "success",
      suggest_ask_host: false,
    };
  }

  const list =
    lower.includes("__intent_list__") ||
    trimmed.includes("有哪些") ||
    lower.startsWith("what are") ||
    lower.includes("list ");
  if (list) {
    return {
      session_id: sessionId,
      answer: "From the authorized materials:\n1. Revenue growth\n2. Gross margin\n3. Operating expenses",
      evidence,
      result_status: "success",
      suggest_ask_host: false,
    };
  }

  // P1b absence slot (qa + existence): MSW mirrors not_found_in_scope vs hit.
  const absenceQuestion =
    trimmed.includes("有没有") ||
    trimmed.includes("是否有") ||
    trimmed.includes("是否存在") ||
    trimmed.includes("存不存在") ||
    /^is there\b/i.test(trimmed) ||
    /^are there\b/i.test(trimmed) ||
    lower.includes("__absence_not_found__");
  if (absenceQuestion) {
    if (lower.includes("__absence_hit__")) {
      const ncEvidence = [
        {
          ...evidence[0],
          chunk_id: "chk_ask_docs_absence_001",
          quote: "乙方不得从事与甲方相竞争的业务（竞业限制）。",
        },
      ];
      return {
        session_id: sessionId,
        answer: "Based on the authorized materials, a non-compete / 竞业限制 clause is present.",
        evidence: ncEvidence,
        result_status: "success",
        suggest_ask_host: false,
      };
    }
    return {
      session_id: sessionId,
      answer:
        "I could not find that clause or topic in the authorized materials. You can ask the host instead.",
      evidence: [],
      result_status: "not_found_in_scope",
      suggest_ask_host: Boolean(link.qaEnabled),
    };
  }

  const qa =
    lower.includes("__intent_qa__") ||
    trimmed.includes("是否") ||
    lower.startsWith("whether") ||
    lower.includes("can i transfer");
  if (qa) {
    return {
      session_id: sessionId,
      answer:
        "Based on the authorized materials, transfer requires prior written consent of the other party.",
      evidence,
      result_status: "success",
      suggest_ask_host: false,
    };
  }

  const topic =
    lower.includes("__intent_topic__") ||
    trimmed === "财务数据" ||
    trimmed === "financials" ||
    trimmed === "财务";
  if (topic) {
    return {
      session_id: sessionId,
      answer: `Found these related excerpts in the authorized materials:\n\n1. Document ${docId.slice(0, 8)} · page 1\n"${evidence[0].quote}"`,
      evidence,
      result_status: "success",
      suggest_ask_host: false,
    };
  }

  return null;
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

function kbAllowsAskDocs(roomId: string | undefined): boolean {
  if (!roomId) return true;
  const kb = mockKnowledgeBases.get(roomId);
  return !!kb && (kb.status === "ready" || kb.status === "stale");
}

/** Hard gate when enabling Ask Docs without ready/stale room KB (US#11). */
function knowledgeBaseRequiredResponse(roomId: string | undefined, aiCopilotEnabled: boolean) {
  if (!aiCopilotEnabled || !roomId) return null;
  if (kbAllowsAskDocs(roomId)) return null;
  return HttpResponse.json(
    {
      code: "knowledge_base_required",
      message: "create or rebuild the room knowledge base before enabling Ask Docs",
    },
    { status: 409 },
  );
}

/**
 * Soft coverage warning when Ask Docs is on and authorized docs/folders are
 * outside the KB checkbox selection (US#12). Empty KB selection ⇒ all auth gaps.
 */
function askDocsCoverageWarnings(
  roomId: string | undefined,
  aiCopilotEnabled: boolean,
  documentIds: string[],
  folderPaths: string[],
): Array<{
  code: string;
  message: string;
  missing_folder_paths?: string[];
  missing_document_ids?: string[];
}> | undefined {
  if (!aiCopilotEnabled || !roomId) return undefined;
  const kb = mockKnowledgeBases.get(roomId);
  if (!kb || (kb.status !== "ready" && kb.status !== "stale")) return undefined;

  const kbFolders = new Set((kb.folder_paths ?? []).map((p) => p.replace(/\/+$/, "") || "/"));
  const kbDocs = new Set(kb.document_ids ?? []);
  const missingFolders: string[] = [];
  const missingDocs: string[] = [];

  for (const path of folderPaths) {
    const normalized = path.replace(/\/+$/, "") || "/";
    const covered = [...kbFolders].some(
      (scope) => normalized === scope || normalized.startsWith(`${scope}/`),
    );
    if (!covered) missingFolders.push(path);
  }
  for (const id of documentIds) {
    if (kbDocs.has(id)) continue;
    // Covered via folder selection only when we know the doc's folder — MSW uses
    // empty folderPaths on many links, so treat uncovered doc ids as gaps unless
    // the KB selected that doc explicitly.
    const coveredByFolder =
      folderPaths.length > 0 &&
      missingFolders.length === 0 &&
      kbFolders.size > 0;
    if (!coveredByFolder) missingDocs.push(id);
  }

  // Empty KB selection: every authorized doc is outside the selection.
  if (kbDocs.size === 0 && kbFolders.size === 0 && documentIds.length > 0) {
    return [
      {
        code: "ask_docs_scope_not_in_kb",
        message:
          "Some authorized folders or documents are outside the knowledge base selection; Ask Docs will only use the intersection.",
        missing_document_ids: documentIds,
      },
    ];
  }

  if (missingFolders.length === 0 && missingDocs.length === 0) return undefined;
  return [
    {
      code: "ask_docs_scope_not_in_kb",
      message:
        "Some authorized folders or documents are outside the knowledge base selection; Ask Docs will only use the intersection.",
      missing_folder_paths: missingFolders.length > 0 ? missingFolders : undefined,
      missing_document_ids: missingDocs.length > 0 ? missingDocs : undefined,
    },
  ];
}

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

  // Test-only reset endpoint used by E2E suites to isolate cases.
  http.post("*/__e2e/reset", () => {
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
    const filter = new URL(request.url).searchParams.get("filter");
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
      case "archived":
        docs = mockDocuments.filter((d) => d.status === "archived");
        break;
      default:
        docs = mockDocuments;
    }
    return HttpResponse.json({ data: docs });
  }),

  http.get("*/api/workspaces/:workspaceSlug/documents/:id", ({ params }) => {
    const doc = mockDocuments.find((d) => d.id === params.id);
    if (!doc) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(doc);
  }),

  http.delete("*/api/workspaces/:workspaceSlug/documents/:id", ({ params }) => {
    const index = mockDocuments.findIndex((d) => d.id === params.id);
    if (index === -1) return new HttpResponse(null, { status: 404 });
    mockDocuments.splice(index, 1);
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
    const title = file?.name ?? "uploaded.pdf";
    const ext = title.split(".").pop()?.toLowerCase() ?? "pdf";
    const fileType = (["pdf", "docx", "pptx", "xlsx"] as const).includes(ext as never) ? (ext as import("@/types").Document["fileType"]) : "pdf";
    const newDoc = {
      id: generateId("doc"),
      title,
      sourceType: fileType,
      fileName: title,
      fileType,
      fileSize: file?.size ?? 1_000_000,
      pageCount: 10,
      status: "ready" as const,
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
      ai_copilot_enabled?: boolean;
      ask_docs_dd_chips_enabled?: boolean;
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
    if (payload.allowed_emails) link.allowedEmails = payload.allowed_emails;
    if (payload.expires_at) link.expiresAt = payload.expires_at;
    if (typeof payload.max_access_count === "number") link.maxAccessCount = payload.max_access_count;
    if (typeof payload.download_enabled === "boolean") link.downloadEnabled = payload.download_enabled;
    if (typeof payload.watermark_enabled === "boolean") link.watermarkEnabled = payload.watermark_enabled;
    if (typeof payload.ai_copilot_enabled === "boolean") link.aiCopilotEnabled = payload.ai_copilot_enabled;
    if (typeof payload.ask_docs_dd_chips_enabled === "boolean") {
      link.askDocsDDChipsEnabled = payload.ask_docs_dd_chips_enabled;
    }
    if (typeof payload.qa_enabled === "boolean") link.qaEnabled = payload.qa_enabled;
    if (payload.contact_ids) link.contactIds = payload.contact_ids;

    if (!link.aiCopilotEnabled) {
      link.askDocsDDChipsEnabled = false;
    }

    const gate = knowledgeBaseRequiredResponse(link.dealRoomId, !!link.aiCopilotEnabled);
    if (gate) return gate;

    const warnings = askDocsCoverageWarnings(
      link.dealRoomId,
      !!link.aiCopilotEnabled,
      link.documentIds ?? (link.documentId ? [link.documentId] : []),
      link.folderPaths ?? [],
    );
    if (warnings?.length) {
      return HttpResponse.json({ ...link, warnings });
    }
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

  http.get("*/api/workspaces/:workspaceSlug/links/:id/access-requests", ({ params }) => {
    const linkId = params.id as string;
    const data = mockLinkAccessRequests.filter((r) => r.link_id === linkId);
    return HttpResponse.json({ data });
  }),

  http.get("*/api/workspaces/:workspaceSlug/links/:id/access-rules", ({ params }) => {
    const link = mockLinks.find((l) => l.id === params.id);
    if (!link) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({ data: [] as { ruleType: "email"; value: string; action: "allow" | "block" }[] });
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
      document_id: string;
      name?: string;
      permission_type?: string;
      require_email?: boolean;
      require_email_verification?: boolean;
      require_password?: boolean;
      require_nda?: boolean;
      allowed_emails?: string[];
      password?: string;
      expires_at?: string;
      max_access_count?: number;
      download_enabled?: boolean;
      watermark_enabled?: boolean;
    };
    const doc = mockDocuments.find((d) => d.id === body.document_id);

    const requirePassword = body.require_password || body.permission_type === "password" || !!body.password;
    const requireNDA = body.require_nda || body.permission_type === "nda";
    const hasWhitelist = body.allowed_emails && body.allowed_emails.length > 0;
    const requireEmailVerification =
      body.require_email_verification ||
      body.permission_type === "email_required" ||
      body.permission_type === "whitelist" ||
      hasWhitelist ||
      requireNDA ||
      false;

    let permissionType: "public" | "email" | "password" | "nda" = "public";
    if (requirePassword) permissionType = "password";
    else if (requireNDA) permissionType = "nda";
    // Modern email verification uses permission_type "public" + require_email_verification.
    // Only the legacy "email_required" permission type maps to "email".
    else if (body.permission_type === "email_required" || body.require_email) permissionType = "email";

    const newLink = {
      id: generateId("link"),
      documentId: body.document_id,
      documentTitle: doc?.title ?? "Untitled",
      shortUrl: `https://invest.acme.capital/d/${generateId("sh")}`,
      accessCount: 0,
      heatLevel: "cold" as const,
      createdAt: new Date().toISOString(),
      expiresAt: body.expires_at,
      isActive: true,
      avgDurationSeconds: 0,
      permissionType,
      _requireEmailVerification: requireEmailVerification,
      _requirePassword: requirePassword,
      _requireNDA: requireNDA,
      _password: body.password,
      _allowedEmails: body.allowed_emails ?? [],
    } as Link & {
      _requireEmailVerification?: boolean;
      _requirePassword?: boolean;
      _requireNDA?: boolean;
      _password?: string;
      _allowedEmails?: string[];
    };
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

  http.get("*/api/workspaces/:workspaceSlug/deal-rooms/:id/links", ({ params }) => {
    const roomId = params.id as string;
    if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
    const data = mockLinks.filter((l) => l.dealRoomId === roomId);
    return HttpResponse.json({ data });
  }),

  http.post("*/api/workspaces/:workspaceSlug/deal-rooms/:id/links", async ({ params, request }) => {
    const roomId = params.id as string;
    if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
    const body = (await request.json()) as {
      name?: string;
      ai_copilot_enabled?: boolean;
      ask_docs_dd_chips_enabled?: boolean;
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
    const askDocs = !!body.ai_copilot_enabled;
    const gate = knowledgeBaseRequiredResponse(roomId, askDocs);
    if (gate) return gate;

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
      aiCopilotEnabled: askDocs,
      askDocsDDChipsEnabled: askDocs ? !!body.ask_docs_dd_chips_enabled : false,
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
    const warnings = askDocsCoverageWarnings(
      roomId,
      askDocs,
      documentIds,
      body.folder_paths ?? [],
    );
    if (warnings?.length) {
      return HttpResponse.json({ ...newLink, warnings }, { status: 201 });
    }
    return HttpResponse.json(newLink, { status: 201 });
  }),

  // Ask Docs audit (owner analytics) — empty ledger by default
  http.get("*/api/workspaces/:workspaceSlug/links/:id/ask-docs-audit", ({ params }) => {
    const link = mockLinks.find((l) => l.id === params.id);
    if (!link) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({ data: mockAskDocsAuditByLink.get(params.id as string) ?? [] });
  }),

  http.get(
    "*/api/workspaces/:workspaceSlug/links/:id/ask-docs-audit/:sessionId",
    ({ params }) => {
      const linkId = params.id as string;
      const sessionId = params.sessionId as string;
      const link = mockLinks.find((l) => l.id === linkId);
      if (!link) return new HttpResponse(null, { status: 404 });
      const detail = mockAskDocsAuditDetails.get(`${linkId}:${sessionId}`);
      if (!detail) return new HttpResponse(null, { status: 404 });
      return HttpResponse.json(detail);
    },
  ),

  http.get(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/ask-docs-audit",
    ({ params }) => {
      const roomId = params.roomId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      return HttpResponse.json({ data: [] });
    },
  ),

  // Deal-room DD Coverage (P2) — in-memory snapshot/run for Owner diligence UI
  http.post(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/dd-coverage/scans",
    async ({ params, request }) => {
      const roomId = params.roomId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      const body = (await request.json().catch(() => ({}))) as {
        pack_id?: string;
        link_id?: string;
        lang?: string;
      };
      const packId = body.pack_id || "financing_dd_v1";
      const key = `${roomId}:${packId}:${body.link_id ?? "room"}`;
      const existing = mockDDCoverageRuns.get(key);
      if (existing && (existing.status === "queued" || existing.status === "running")) {
        return HttpResponse.json(
          { code: "scan_in_progress", message: "a scan is already queued or running for this scope" },
          { status: 409 },
        );
      }
      const runId = `dd-run-${crypto.randomUUID()}`;
      const now = new Date().toISOString();
      const run = {
        id: runId,
        pack_id: packId,
        pack_version: "1",
        scope: body.link_id ? ("link" as const) : ("room" as const),
        link_id: body.link_id,
        status: "queued" as const,
        triggered_by: "user-1",
        created_at: now,
      };
      mockDDCoverageRuns.set(key, run);
      mockDDCoverageRunsById.set(runId, { key, run });
      // Complete asynchronously on next get — mark succeeded with sample rows.
      queueMicrotask(() => {
        const entry = mockDDCoverageRunsById.get(runId);
        if (!entry) return;
        const finished = {
          ...entry.run,
          status: "succeeded" as const,
          started_at: now,
          finished_at: new Date().toISOString(),
        };
        mockDDCoverageRuns.set(entry.key, finished);
        mockDDCoverageRunsById.set(runId, { key: entry.key, run: finished });
        mockDDCoverageSnapshots.set(entry.key, {
          id: `dd-snap-${runId}`,
          pack_id: finished.pack_id,
          pack_version: finished.pack_version,
          scope: finished.scope,
          link_id: finished.link_id,
          run_id: finished.id,
          kb_generation: 1,
          stale: false,
          coverage_rows: [
            {
              item_id: "cap_table",
              label: "Cap table",
              status: "supported",
              clues: [
                {
                  chunk_id: "chunk-cap-1",
                  document_id: "doc-1",
                  page_number: 2,
                  quote: "Fully diluted capitalization table as of Series A.",
                  score: 0.91,
                  boxes: [],
                },
              ],
            },
            {
              item_id: "option_pool",
              label: "Option / ESOP pool",
              status: "supported",
              value_type: "percent",
              extracted_value: "10%",
              clues: [
                {
                  chunk_id: "chunk-opt-1",
                  document_id: "doc-1",
                  page_number: 5,
                  quote: "ESOP pool remains at 10% post-money.",
                  score: 0.87,
                  boxes: [],
                },
              ],
            },
          ],
          created_at: now,
          updated_at: new Date().toISOString(),
        });
      });
      return HttpResponse.json({ job_id: runId, run }, { status: 202 });
    },
  ),
  http.get(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/dd-coverage/scans/:runId",
    ({ params }) => {
      const roomId = params.roomId as string;
      const runId = params.runId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      const entry = mockDDCoverageRunsById.get(runId);
      if (!entry) return new HttpResponse(null, { status: 404 });
      return HttpResponse.json(entry.run);
    },
  ),
  http.get(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/dd-coverage/snapshot",
    ({ params, request }) => {
      const roomId = params.roomId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      const url = new URL(request.url);
      const linkId = url.searchParams.get("link_id") ?? undefined;
      const packId = url.searchParams.get("pack_id") || "financing_dd_v1";
      const key = `${roomId}:${packId}:${linkId ?? "room"}`;
      const snap = mockDDCoverageSnapshots.get(key);
      if (!snap) {
        return HttpResponse.json({ code: "not_found", message: "not found" }, { status: 404 });
      }
      return HttpResponse.json(snap);
    },
  ),

  http.get(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/dd-coverage/packs",
    ({ params }) => {
      const roomId = params.roomId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      const financing = mockDDCoveragePacks.get(roomId) ?? builtinDDPack;
      return HttpResponse.json({ data: [financing, builtinMAPack] });
    },
  ),

  http.get(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/dd-coverage/pack",
    ({ params, request }) => {
      const roomId = params.roomId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      const packId = new URL(request.url).searchParams.get("pack_id") || "financing_dd_v1";
      if (packId === "ma_redflag_v1") {
        return HttpResponse.json(builtinMAPack);
      }
      return HttpResponse.json(mockDDCoveragePacks.get(roomId) ?? builtinDDPack);
    },
  ),
  http.put(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/dd-coverage/pack",
    async ({ params, request }) => {
      const roomId = params.roomId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      const body = (await request.json()) as { items?: typeof builtinDDPack.items };
      const prev = mockDDCoveragePacks.get(roomId);
      const revision = (prev?.fork_revision ?? 0) + 1;
      const pack = {
        pack_id: "financing_dd_v1",
        pack_version: `1.f${revision}`,
        base_pack_id: "financing_dd_v1",
        forked: true,
        fork_revision: revision,
        items: body.items ?? [],
      };
      mockDDCoveragePacks.set(roomId, pack);
      for (const [key, snap] of mockDDCoverageSnapshots) {
        if (key.startsWith(`${roomId}:`)) {
          mockDDCoverageSnapshots.set(key, { ...snap, stale: true });
        }
      }
      return HttpResponse.json(pack);
    },
  ),
  http.post(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/dd-coverage/pack/reset",
    ({ params }) => {
      const roomId = params.roomId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      mockDDCoveragePacks.delete(roomId);
      for (const [key, snap] of mockDDCoverageSnapshots) {
        if (key.startsWith(`${roomId}:`)) {
          mockDDCoverageSnapshots.set(key, { ...snap, stale: true });
        }
      }
      return HttpResponse.json(builtinDDPack);
    },
  ),

  http.post(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/dd-coverage/cross-checks",
    async ({ params, request }) => {
      const roomId = params.roomId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      const body = (await request.json()) as {
        pack_id?: string;
        document_a_id?: string;
        document_b_id?: string;
      };
      if (!body.document_a_id || !body.document_b_id || body.document_a_id === body.document_b_id) {
        return HttpResponse.json(
          { code: "invalid_input", message: "document ids must differ" },
          { status: 400 },
        );
      }
      const packId = body.pack_id || "financing_dd_v1";
      const now = new Date().toISOString();
      const view = {
        id: `dd-cross-${crypto.randomUUID()}`,
        pack_id: packId,
        pack_version: "1",
        document_a_id: body.document_a_id,
        document_b_id: body.document_b_id,
        triggered_by: "user-1",
        claims: [
          {
            item_id: packId === "ma_redflag_v1" ? "indemnity_cap" : "cap_table",
            label: packId === "ma_redflag_v1" ? "Indemnity / liability cap" : "Cap table",
            status: "conflict",
            clues_a: [
              {
                chunk_id: "xa1",
                document_id: body.document_a_id,
                page_number: 1,
                quote: "Document A says one thing.",
                score: 0.9,
                boxes: [],
              },
            ],
            clues_b: [
              {
                chunk_id: "xb1",
                document_id: body.document_b_id,
                page_number: 2,
                quote: "Document B says the opposite.",
                score: 0.88,
                boxes: [],
              },
            ],
          },
        ],
        created_at: now,
      };
      mockDDCrossChecks.set(`${roomId}:${packId}`, view);
      return HttpResponse.json(view);
    },
  ),
  http.get(
    "*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/dd-coverage/cross-checks/latest",
    ({ params, request }) => {
      const roomId = params.roomId as string;
      if (!findRoom(roomId)) return new HttpResponse(null, { status: 404 });
      const packId = new URL(request.url).searchParams.get("pack_id") || "financing_dd_v1";
      const view = mockDDCrossChecks.get(`${roomId}:${packId}`);
      if (!view) {
        return HttpResponse.json({ code: "not_found", message: "not found" }, { status: 404 });
      }
      return HttpResponse.json(view);
    },
  ),

  http.get("*/api/workspaces/:workspaceSlug/dd-portfolio/views", () => {
    const data = Array.from(mockDDPortfolioViews.values()).map((v) => ({
      id: v.id,
      name: v.name,
      pack_id: v.pack_id,
      room_count: v.room_ids.length,
      created_by: v.created_by,
      created_at: v.created_at,
      updated_at: v.updated_at,
    }));
    return HttpResponse.json({ data });
  }),
  http.post("*/api/workspaces/:workspaceSlug/dd-portfolio/views", async ({ request }) => {
    const body = (await request.json()) as {
      name?: string;
      pack_id?: string;
      room_ids?: string[];
    };
    if (!body.name?.trim() || !body.room_ids?.length) {
      return HttpResponse.json(
        { code: "invalid_input", message: "name and room_ids required" },
        { status: 400 },
      );
    }
    const now = new Date().toISOString();
    const id = `pf-${crypto.randomUUID()}`;
    const row = {
      id,
      name: body.name.trim(),
      pack_id: body.pack_id || "financing_dd_v1",
      room_ids: body.room_ids,
      created_by: "user-1",
      created_at: now,
      updated_at: now,
    };
    mockDDPortfolioViews.set(id, row);
    return HttpResponse.json(
      {
        id: row.id,
        name: row.name,
        pack_id: row.pack_id,
        created_by: row.created_by,
        created_at: row.created_at,
        updated_at: row.updated_at,
        rooms: row.room_ids.map((roomId, idx) => ({
          deal_room_id: roomId,
          deal_room_name: findRoom(roomId)?.name || roomId,
          has_snapshot: idx === 0,
          stale: false,
          supported: idx === 0 ? 1 : 0,
          absent: idx === 0 ? 2 : 0,
          insufficient: idx === 0 ? 1 : 0,
          total: idx === 0 ? 4 : 0,
          top_absent:
            idx === 0
              ? [
                  { item_id: "option_pool", label: "Option pool" },
                  { item_id: "nda", label: "NDA" },
                ]
              : [],
        })),
      },
      { status: 201 },
    );
  }),
  http.get("*/api/workspaces/:workspaceSlug/dd-portfolio/views/:viewId", ({ params }) => {
    const viewId = params.viewId as string;
    const row = mockDDPortfolioViews.get(viewId);
    if (!row) {
      return HttpResponse.json({ code: "not_found", message: "not found" }, { status: 404 });
    }
    return HttpResponse.json({
      id: row.id,
      name: row.name,
      pack_id: row.pack_id,
      created_by: row.created_by,
      created_at: row.created_at,
      updated_at: row.updated_at,
      rooms: row.room_ids.map((roomId, idx) => ({
        deal_room_id: roomId,
        deal_room_name: findRoom(roomId)?.name || roomId,
        has_snapshot: idx === 0,
        stale: false,
        supported: idx === 0 ? 1 : 0,
        absent: idx === 0 ? 2 : 0,
        insufficient: idx === 0 ? 1 : 0,
        total: idx === 0 ? 4 : 0,
        top_absent:
          idx === 0
            ? [
                { item_id: "option_pool", label: "Option pool" },
                { item_id: "nda", label: "NDA" },
              ]
            : [],
      })),
    });
  }),
  http.put("*/api/workspaces/:workspaceSlug/dd-portfolio/views/:viewId", async ({ params, request }) => {
    const viewId = params.viewId as string;
    const row = mockDDPortfolioViews.get(viewId);
    if (!row) {
      return HttpResponse.json({ code: "not_found", message: "not found" }, { status: 404 });
    }
    const body = (await request.json()) as {
      name?: string;
      pack_id?: string;
      room_ids?: string[];
    };
    if (body.name !== undefined) row.name = body.name.trim();
    if (body.pack_id) row.pack_id = body.pack_id;
    if (body.room_ids) row.room_ids = body.room_ids;
    row.updated_at = new Date().toISOString();
    mockDDPortfolioViews.set(viewId, row);
    return HttpResponse.json({
      id: row.id,
      name: row.name,
      pack_id: row.pack_id,
      created_by: row.created_by,
      created_at: row.created_at,
      updated_at: row.updated_at,
      rooms: row.room_ids.map((roomId) => ({
        deal_room_id: roomId,
        deal_room_name: findRoom(roomId)?.name || roomId,
        has_snapshot: false,
        supported: 0,
        absent: 0,
        insufficient: 0,
        total: 0,
        top_absent: [],
      })),
    });
  }),
  http.delete("*/api/workspaces/:workspaceSlug/dd-portfolio/views/:viewId", ({ params }) => {
    const viewId = params.viewId as string;
    if (!mockDDPortfolioViews.has(viewId)) {
      return HttpResponse.json({ code: "not_found", message: "not found" }, { status: 404 });
    }
    mockDDPortfolioViews.delete(viewId);
    return new HttpResponse(null, { status: 204 });
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

  // Deal room documents
  http.get("*/api/workspaces/:workspaceSlug/deal-rooms/:id/documents", ({ params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({ data: getRoomFolderDocs(room) });
  }),

  // Deal-room knowledge base (Ask Docs corpus)
  http.get("*/api/workspaces/:workspaceSlug/deal-rooms/:id/knowledge-base", ({ params }) => {
    const roomId = params.id as string;
    const room = findRoom(roomId);
    if (!room) return new HttpResponse(null, { status: 404 });
    const existing = mockKnowledgeBases.get(roomId);
    if (existing) return HttpResponse.json(existing);
    return HttpResponse.json({
      room_id: roomId,
      status: "none",
      folder_paths: [],
      document_ids: [],
      active_document_ids: [],
      embedded_count: 0,
      folder_count: 0,
    } satisfies DealRoomKnowledgeBase);
  }),

  http.post("*/api/workspaces/:workspaceSlug/deal-rooms/:id/knowledge-base", async ({ params, request }) => {
    const roomId = params.id as string;
    const room = findRoom(roomId);
    if (!room) return new HttpResponse(null, { status: 404 });
    const body = (await request.json().catch(() => ({}))) as {
      folder_paths?: string[];
      document_ids?: string[];
    };
    const documentIds = body.document_ids ?? [];
    if (documentIds.some((id) => id.includes("no_chunks") || id === "__no_chunks__")) {
      return HttpResponse.json(
        {
          code: "no_searchable_chunks",
          message:
            "selected documents have no searchable text chunks; re-ingest documents that have preview pages but no extracted text before building the knowledge base",
        },
        { status: 400 },
      );
    }
    const kb: DealRoomKnowledgeBase = {
      room_id: roomId,
      status: "ready",
      folder_paths: body.folder_paths ?? [],
      document_ids: documentIds,
      active_document_ids: documentIds,
      active_generation: 1,
      embedded_count: documentIds.length,
      folder_count: (body.folder_paths ?? []).length,
    };
    mockKnowledgeBases.set(roomId, kb);
    return HttpResponse.json(kb, { status: 201 });
  }),

  http.post("*/api/workspaces/:workspaceSlug/deal-rooms/:id/knowledge-base/rebuild", async ({ params, request }) => {
    const roomId = params.id as string;
    const room = findRoom(roomId);
    if (!room) return new HttpResponse(null, { status: 404 });
    const existing = mockKnowledgeBases.get(roomId);
    if (!existing || existing.status === "none") {
      return HttpResponse.json(
        { code: "knowledge_base_not_found", message: "knowledge base not found" },
        { status: 404 },
      );
    }
    if (existing.status === "building") {
      return HttpResponse.json(
        { code: "knowledge_base_building", message: "knowledge base is building" },
        { status: 409 },
      );
    }
    const body = (await request.json().catch(() => ({}))) as {
      folder_paths?: string[];
      document_ids?: string[];
    };
    const documentIds = body.document_ids ?? existing.document_ids;
    const folderPaths = body.folder_paths ?? existing.folder_paths;
    if (documentIds.some((id) => id.includes("no_chunks") || id === "__no_chunks__")) {
      return HttpResponse.json(
        {
          code: "no_searchable_chunks",
          message:
            "selected documents have no searchable text chunks; re-ingest documents that have preview pages but no extracted text before building the knowledge base",
        },
        { status: 400 },
      );
    }
    const kb: DealRoomKnowledgeBase = {
      ...existing,
      status: "ready",
      folder_paths: folderPaths,
      document_ids: documentIds,
      active_document_ids: documentIds,
      active_generation: (existing.active_generation ?? 1) + 1,
      building_document_ids: [],
      building_generation: undefined,
      embedded_count: documentIds.length,
      folder_count: folderPaths.length,
      error_message: undefined,
    };
    mockKnowledgeBases.set(roomId, kb);
    return HttpResponse.json(kb);
  }),

  // Visitor Ask high-risk security events (owner analytics)
  http.get("*/api/workspaces/:workspaceSlug/links/:id/ask-security-events", ({ params }) => {
    const linkId = params.id as string;
    const link = mockLinks.find((l) => l.id === linkId);
    if (!link) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({
      data: [
        {
          id: `ask-sec-${linkId}-1`,
          link_id: linkId,
          event_type: "rate_limit_exceeded",
          visitor_id: "visitor-ask-1",
          email: "visitor@example.com",
          reason: "ask_docs",
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
      ],
    });
  }),

  http.get("*/api/workspaces/:workspaceSlug/deal-rooms/:roomId/ask-security-events", ({ params, request }) => {
    const roomId = params.roomId as string;
    const room = findRoom(roomId);
    if (!room) return new HttpResponse(null, { status: 404 });
    const url = new URL(request.url);
    const filterLinkId = url.searchParams.get("link_id");
    const roomLinks = mockLinks.filter((l) => l.dealRoomId === roomId);
    const source = roomLinks.length > 0
      ? roomLinks
      : [{ id: `${roomId}-synthetic-link` } as { id: string }];
    const events = source
      .filter((l) => !filterLinkId || l.id === filterLinkId)
      .flatMap((l, idx) => [
        {
          id: `ask-sec-room-${l.id}-rate`,
          link_id: l.id,
          event_type: "rate_limit_exceeded",
          visitor_id: `visitor-rate-${idx}`,
          email: `rate${idx}@example.com`,
          reason: "ask_docs",
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
      ]);
    return HttpResponse.json({ data: events });
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

  // Assistant (owner Ask Docs)
  http.post("*/api/workspaces/:workspaceSlug/assistant/chat", async ({ request }) => {
    const body = (await request.json()) as {
      message?: string;
      query?: string;
      document_id?: string;
      session_id?: string;
    };
    const message = (body.message ?? body.query ?? "").trim();
    const sessionId = body.session_id || generateId("sess");

    if (
      message.includes("投资建议") ||
      message.toLowerCase().includes("investment advice") ||
      message.toLowerCase().includes("__intent_refuse__") ||
      message === "财务数据" ||
      message.toLowerCase().includes("__intent_topic__") ||
      message.toLowerCase().includes("__intent_locate__") ||
      message.toLowerCase().includes("__intent_list__") ||
      message.toLowerCase().includes("__intent_qa__") ||
      message.includes("有哪些") ||
      message.includes("是否")
    ) {
      const ownerLink = {
        documentId: body.document_id ?? "doc_1",
        qaEnabled: false,
      } as Link;
      const intentFixture = mockAskDocsIntentFirstResponse(message, sessionId, ownerLink);
      if (intentFixture) {
        // Owner refuse: same status, no Host CTA (G12).
        if (intentFixture.result_status === "out_of_corpus") {
          intentFixture.suggest_ask_host = false;
          intentFixture.answer =
            "This question is outside what the authorized materials can support (for example market practice or external legal advice), so I will not invent an answer. Please add materials or use human judgment.";
        }
        return HttpResponse.json(intentFixture);
      }
    }

    return HttpResponse.json({
      session_id: sessionId,
      answer: `Based on the document, here's an answer to "${message}".`,
      evidence: body.document_id
        ? [
            {
              chunk_id: "chk_demo_001",
              quote: "Revenue grew 3x year over year.",
              page_number: 1,
              boxes: [{ x: 0.12, y: 0.34, w: 0.45, h: 0.06 }],
              score: 0.92,
            },
          ]
        : [],
      follow_up_questions: ["Can you explain the growth drivers?", "What are the risks?"],
      result_status: "success",
    });
  }),

  // Search
  http.post("*/api/workspaces/:workspaceSlug/search", async ({ request }) => {
    const body = (await request.json()) as { query: string; document_id?: string };
    return HttpResponse.json({
      document_id: body.document_id,
      query: body.query,
      results: body.document_id
        ? [
            {
              chunk_id: "chk_demo_001",
              quote: "Revenue grew 3x year over year.",
              page_number: 1,
              boxes: [{ x: 0.12, y: 0.34, w: 0.45, h: 0.06 }],
              score: 0.92,
            },
          ]
        : [],
    });
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

  // Public viewer
  http.post("*/api/v1/public/links/:token", async ({ params, request }) => {
    const body = (await request.json().catch(() => ({}))) as {
      email?: string;
      email_code?: string;
      password?: string;
      nda_agreed?: boolean;
    };
    const token = params.token as string;
    const link = mockLinks.find((l) => l.shortUrl.endsWith(token)) ?? mockLinks[0];
    const extended = link as Link & {
      _requireEmailVerification?: boolean;
      _requirePassword?: boolean;
      _requireNDA?: boolean;
      _password?: string;
      _allowedEmails?: string[];
    };

    // The mock permissionType "email" corresponds to the legacy "email_required" type,
    // where the visitor must supply both email and code. Modern email verification uses
    // permissionType "public" + _requireEmailVerification and is code-only.
    const isLegacyEmailRequired = extended.permissionType === "email";
    const requiresEmailVerification =
      extended._requireEmailVerification || isLegacyEmailRequired || extended.permissionType === "nda";
    const requiresPassword = extended._requirePassword || extended.permissionType === "password";
    const requiresNda = extended._requireNDA || extended.permissionType === "nda";
    const hasWhitelist = extended._allowedEmails && extended._allowedEmails.length > 0;
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
      const allowed = (extended._allowedEmails ?? []).some(
        (entry) => entry.trim().toLowerCase() === body.email!.toLowerCase()
      );
      if (!allowed) {
        return HttpResponse.json(
          { code: "whitelist_denied", message: "email not in whitelist", requiresEmail, requiresEmailVerification, requiresPassword, requiresNda },
          { status: 403 }
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
        aiCopilotEnabled: Boolean(link.aiCopilotEnabled),
        askDocsDDChipsEnabled: Boolean(link.askDocsDDChipsEnabled),
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

  http.get("*/api/v1/public/links/:token/assistant/dd-chips", ({ params }) => {
    const token = params.token as string;
    const link = mockLinks.find((l) => l.shortUrl.endsWith(token));
    if (!link) {
      return HttpResponse.json({ code: "not_found", message: "link not found" }, { status: 404 });
    }
    if (!link.aiCopilotEnabled || !link.askDocsDDChipsEnabled) {
      return HttpResponse.json(
        { code: "not_found", message: "suggested checklist chips are not available" },
        { status: 404 },
      );
    }
    return HttpResponse.json({
      data: [
        { item_id: "financing_dd_v1.cap_table", label: "Cap table" },
        { item_id: "financing_dd_v1.financials", label: "Financial statements" },
      ],
    });
  }),

  // Public Ask Docs (Visitor Ask / Ask Docs channel)
  http.post("*/api/v1/public/links/:token/assistant/chat", async ({ params, request }) => {
    const token = params.token as string;
    const link = mockLinks.find((l) => l.shortUrl.endsWith(token));
    if (!link) {
      return HttpResponse.json({ code: "not_found", message: "link not found" }, { status: 404 });
    }
    if (!link.aiCopilotEnabled) {
      return HttpResponse.json(
        { code: "ai_copilot_disabled", message: "Ask Docs is disabled for this link" },
        { status: 403 },
      );
    }

    const burst = (mockAskDocsBurst.get(token) ?? 0) + 1;
    mockAskDocsBurst.set(token, burst);
    // Keep burst low so smoke tests stay under limit; dedicated rate-limit test uses __rate_limit__.
    if (burst > 20) {
      return HttpResponse.json(
        { code: "rate_limit_exceeded", message: "too many Ask Docs requests, please try again later" },
        { status: 429 },
      );
    }

    const body = (await request.json().catch(() => ({}))) as {
      message?: string;
      session_id?: string;
      checklist_item_id?: string;
    };
    const message = (body.message ?? "").trim();
    if (!message) {
      return HttpResponse.json({ code: "invalid_request", message: "message required" }, { status: 400 });
    }
    if (body.checklist_item_id && !link.askDocsDDChipsEnabled) {
      return HttpResponse.json(
        { code: "invalid_checklist_item", message: "checklist item is not enabled for this link" },
        { status: 400 },
      );
    }

    const sessionId = body.session_id?.trim() || generateId("sess");
    const lower = message.toLowerCase();
    if (lower.includes("__rate_limit__")) {
      return HttpResponse.json(
        { code: "rate_limit_exceeded", message: "too many Ask Docs requests, please try again later" },
        { status: 429 },
      );
    }
    if (lower.includes("__limiter_down__")) {
      return HttpResponse.json(
        {
          code: "limiter_unavailable",
          message: "Ask Docs is temporarily unavailable, please try again later",
        },
        { status: 503 },
      );
    }
    if (lower.includes("__no_evidence__") || lower.includes("no evidence")) {
      return HttpResponse.json({
        session_id: sessionId,
        answer:
          "I couldn't find supporting material in the documents you can access for this link. You can ask the host instead.",
        evidence: [],
        result_status: "no_evidence",
        suggest_ask_host: Boolean(link.qaEnabled),
      });
    }

    // Intent-first P0 acceptance fixtures (MSW mirrors backend DocIntent routing).
    const intentFixture = mockAskDocsIntentFirstResponse(message, sessionId, link);
    if (intentFixture) {
      return HttpResponse.json(intentFixture);
    }

    return HttpResponse.json({
      session_id: sessionId,
      answer: `Based on authorized materials, here is an answer to "${message}".`,
      evidence: [
        {
          chunk_id: "chk_ask_docs_001",
          document_id: link.documentId ?? "doc_1",
          quote: "Revenue grew 3x year over year.",
          page_number: 1,
          boxes: [{ x: 0.12, y: 0.34, w: 0.45, h: 0.06 }],
          score: 0.92,
        },
      ],
      result_status: "success",
      suggest_ask_host: false,
    });
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
