import type {
  AccessLog,
  AccessRule,
  ActionItem,
  Activity,
  BillingInfo,
  Contact,
  DealRoom,
  DealRoomAccessPolicy,
  DealRoomAccessRequest,
  DealRoomAnalytics,
  DealRoomKnowledgeCorpus,
  DealRoomKnowledgeQueryResult,
  DealRoomKnowledgeSessionDetail,
  DealRoomKnowledgeSessionList,
  DealRoomKnowledgeSessionQueryResult,
  DealRoomKnowledgeQASession,
  DealRoomKnowledgeTurnFeedback,
  DealRoomKnowledgeFeedbackKind,
  DealRoomKnowledgeFollowUpsResult,
  DealRoomKnowledgeMissionPack,
  DealRoomKnowledgeMissionProgress,
  DealRoomKnowledgeEvalCandidate,
  DealRoomKnowledgeEvalSeedExport,
  DealRoomKnowledgeDiligencePack,
  DealRoomKnowledgeSessionArchiveList,
  DealRoomKnowledgeSessionArchiveDetail,
  DealRoomKnowledgeOpsSummary,
  DealRoomDocumentItem,
  DealRoomFolder,
  DealRoomFolderDocs,
  DealRoomMember,
  DealRoomTemplate,
  Document,
  DocumentFilter,
  HeatAlert,
  HeatLevel,
  IntegrationStatus,
  OutboundWebhookConfig,
  Link,
  LinkAccessRequest,
  PendingLinkAccessRequest,
  LinkAnalytics,
  LinkAccessCodeContact,
  LinkRecentVisitor,
  PageAnalytics,
  PermissionConfig,
  RiskAlert,
  SecuritySettings,
  Signal,
  Suggestion,
  User,
  VisitorSummary,
  Workspace,
  WorkspaceInvitation,
  WorkspaceMember,
  WorkspaceSettings,
  PublicAskTurn,
  PublicAskFAQ,
  PublicFormalAsk,
  OwnerAskTurn,
  FileRequest,
  AskSecurityEvent,
} from "@/types";
import { openStream, request } from "@/lib/apiClient";
import {
  toBackendIntegrationStatus,
  toCreateDealRoomPayload,
  toCreateLinkPayload,
  toIntegrationStatus,
  toOutboundWebhookConfig,
  type BackendIntegrationStatus,
  type BackendOutboundWebhook,
  type UpdateLinkPayload,
} from "@/lib/apiAdapters";
import {
  clearCachedAccountEmail,
  setCachedAccountEmail,
} from "@/lib/authAccount";
import type { RadarEvidencePack, RadarFeed } from "@/lib/radarQueue";
import { consumeKnowledgeSSE } from "@/lib/knowledge/consumeKnowledgeSSE";
import type { KnowledgeStreamEvent } from "@/lib/knowledge/streamEvents";
import { useUIStore } from "@/stores/uiStore";

export interface RecentActivityItem {
  id: string;
  eventType: "visit" | "download" | "question" | "upload";
  actor: string;
  objectType: "room" | "document" | "question";
  objectName: string;
  objectId: string;
  createdAt: string;
}

export interface DashboardStats {
  hotCount: number;
  warmCount: number;
  coldCount: number;
  weeklyVisitors: number;
  pendingQuestions: number;
  recentDocuments: Document[];
  recentLinks: Link[];
  heatAlerts: HeatAlert[];
  riskAlerts: RiskAlert[];
  signals: Signal[];
  actionItems: ActionItem[];
  recentActivities: RecentActivityItem[];
}

export interface InsightsDailyVisit {
  date: string;
  opens: number;
  uniqueVisitors: number;
}

/** Supported Insights overview trend windows (matches API normalizeInsightsDays). */
export type InsightsRangeDays = 7 | 30 | 90;

/** Params for GET /insights/overview — preset days or custom UTC from/to. */
export type InsightsOverviewParams =
  | { days?: InsightsRangeDays; from?: undefined; to?: undefined }
  | { from: string; to: string; days?: undefined };

export interface InsightsOverview {
  /** Always "link" — tierCounts bucket share links via heat.Compute. */
  tierEntity?: "link";
  tierCounts: Record<HeatLevel, number>;
  activeLinkCount: number;
  /** Selected trend window length in days (preset or custom span). */
  rangeDays?: number;
  /** Inclusive UTC start date (YYYY-MM-DD). */
  rangeFrom?: string;
  /** Inclusive UTC end date (YYYY-MM-DD). */
  rangeTo?: string;
  /** True when the window was requested via from/to. */
  rangeCustom?: boolean;
  /** Server generation timestamp (UTC RFC3339). */
  generatedAt?: string;
  /** access_logs partition retention days (server config). */
  eventRetentionDays?: number;
  /** page_views partition retention days (server config). */
  pageViewRetentionDays?: number;
  /** Sum of link opens in the selected window. */
  periodOpens?: number;
  /** Sum of link opens in the prior equal-length window. */
  previousPeriodOpens?: number;
  /** Distinct visitors with ≥1 link_opened in the selected window. */
  periodUniqueVisitors?: number;
  /** Distinct visitors in the prior equal-length window. */
  previousPeriodUniqueVisitors?: number;
  /** Median page-view duration (seconds) in the selected window. */
  periodMedianDurationSeconds?: number;
  /** Median page-view duration in the prior equal-length window. */
  previousPeriodMedianDurationSeconds?: number;
  /** Average page-view duration (seconds) in the selected window. */
  periodAvgDurationSeconds?: number;
  /** Page-view count in the selected window. */
  periodPageViewCount?: number;
  /** Reading sessions with activity in the selected window. */
  periodSessionCount?: number;
  /** Sessions whose document has a known page_count (completion denominator). */
  periodMeasurableSessions?: number;
  /** Sessions that reached the last page (max_page ≥ page_count). */
  periodCompletedSessions?: number;
  /** completed / measurable in the selected window (0–1). */
  periodCompletionRate?: number;
  previousPeriodSessionCount?: number;
  previousPeriodCompletedSessions?: number;
  previousPeriodCompletionRate?: number;
  /** Open Deal Radar signals (honest action summary — not topContacts length). */
  openSignalCount?: number;
  /** Dense UTC day series of link_opened counts for rangeDays (server-aggregated). */
  dailyVisits: InsightsDailyVisit[];
  topDocuments: {
    id: string;
    title: string;
    views: number;
    score?: number;
    heatLevel: HeatLevel;
    /** Hottest share link on this document — opens heat breakdown. */
    primaryLinkId?: string;
  }[];
  topLinks: {
    id: string;
    title?: string;
    documentId?: string;
    shortUrl: string;
    views: number;
    score?: number;
    heatLevel: HeatLevel;
  }[];
  /** Lifetime heat contacts for Deal Radar feeds — not used as Insights Overview CTA count. */
  topContacts?: { id: string; email: string; score: number; heatLevel: HeatLevel }[];
}

/** GET /analytics/links/:linkId/score — heat.Compute breakdown. */
export interface LinkHeatScore {
  linkId: string;
  score: number;
  level: HeatLevel;
  trend: "rising" | "falling" | "stable";
  breakdown: Record<string, number>;
  updatedAt: string;
}

/** GET /insights/access-audit — permission / gate failure slice. */
export interface AccessAuditTypeCount {
  eventType: string;
  count: number;
}

export interface AccessAuditRoomCount {
  dealRoomId?: string | null;
  dealRoomName: string;
  count: number;
  /** Present when dealRoomId is null (library / non–deal-room links). */
  scope?: "library";
}

export interface AccessAuditMemberCount {
  memberId?: string | null;
  memberEmail: string;
  count: number;
  /** Present when memberId is null (orphan / deleted creator). */
  scope?: "unknown";
}

export interface AccessAuditFolderCount {
  folderPath: string;
  dealRoomId?: string | null;
  dealRoomName: string;
  count: number;
  /** Present when folderPath is empty (room root / no placement). */
  scope?: "root";
}

export interface AccessAuditEvent {
  id: string;
  linkId?: string;
  eventType: string;
  visitorId?: string;
  email?: string;
  reason?: string;
  createdAt: string;
  documentTitle: string;
  dealRoomId?: string;
  dealRoomName: string;
  memberId?: string;
  memberEmail?: string;
  folderPath?: string;
}

export interface AccessAudit {
  rangeDays: number;
  rangeFrom?: string;
  rangeTo?: string;
  rangeCustom?: boolean;
  generatedAt: string;
  totalEvents: number;
  byType: AccessAuditTypeCount[];
  byDealRoom: AccessAuditRoomCount[];
  byMember: AccessAuditMemberCount[];
  byFolder: AccessAuditFolderCount[];
  events: AccessAuditEvent[];
  hasMore: boolean;
  limit: number;
  offset: number;
}

export type AccessAuditParams = {
  days?: InsightsRangeDays;
  from?: string;
  to?: string;
  eventType?: string;
  dealRoomId?: string;
  memberId?: string;
  folderPath?: string;
  limit?: number;
  offset?: number;
};

/** Heat circle used for key-page keyword matching. */
export type KeyPageHeatCircle = "founder" | "investor_ir" | "sales";

/** GET /insights/key-pages — sensitive / key-page compliance report. */
export interface KeyPageComplianceCategoryCount {
  category: string;
  count: number;
}

export interface KeyPageCompliancePage {
  documentId: string;
  documentTitle: string;
  pageNumber: number;
  pageTitle: string;
  category: string;
  views: number;
  uniqueVisitors: number;
  avgDurationSeconds: number;
  lastViewedAt?: string;
}

export interface KeyPageComplianceEvent {
  id: string;
  linkId?: string;
  visitorId?: string;
  visitorEmail?: string;
  documentId?: string;
  documentTitle: string;
  pageNumber: number;
  pageTitle: string;
  category: string;
  durationSeconds: number;
  createdAt: string;
  dealRoomId?: string;
  dealRoomName: string;
}

export interface KeyPageComplianceMatchRule {
  category: string;
  keywords: string[];
}

export interface KeyPageCompliance {
  rangeDays: number;
  rangeFrom?: string;
  rangeTo?: string;
  rangeCustom?: boolean;
  circle: KeyPageHeatCircle | string;
  generatedAt: string;
  totalViews: number;
  engagedViews: number;
  uniqueVisitors: number;
  distinctPages: number;
  /** Same heat-circle title keywords the API used for matching (EN+ZH). */
  matchRules: KeyPageComplianceMatchRule[];
  byCategory: KeyPageComplianceCategoryCount[];
  pages: KeyPageCompliancePage[];
  events: KeyPageComplianceEvent[];
  hasMore: boolean;
  limit: number;
  offset: number;
}

export interface KeyPageSettings {
  defaultCircle: KeyPageHeatCircle | string;
  extraKeywords: Record<string, string[]>;
  /** Circle built-ins only (no workspace extras) — for editor disclosure. */
  builtinRules: KeyPageComplianceMatchRule[];
  /** Effective merged keywords (builtins + extras). */
  matchRules: KeyPageComplianceMatchRule[];
  canEdit: boolean;
  updatedAt?: string;
}

export type KeyPageSettingsUpdate = {
  defaultCircle: KeyPageHeatCircle | string;
  extraKeywords: Record<string, string[]>;
};

export type KeyPageComplianceParams = {
  days?: InsightsRangeDays;
  from?: string;
  to?: string;
  circle?: KeyPageHeatCircle;
  limit?: number;
  offset?: number;
};

/** GET /insights/documents/:id/funnel — visitor-session reach drop-off. */
export interface DocumentFunnelStep {
  pageNumber: number;
  visitorsReached: number;
  dropOffFromPrev: number;
}

export interface DocumentReadingFunnel {
  documentId: string;
  pageCount: number;
  sessionCount: number;
  completedSessions: number;
  completionRate: number;
  medianMaxPage: number;
  avgPagesPerSession: number;
  avgDurationSeconds: number;
  biggestDropOffPage: number;
  steps: DocumentFunnelStep[];
  /** Idle-gap session grain from reading_sessions ("reading_session"). */
  sessionModel?: "reading_session";
  rangeDays?: number;
  rangeFrom?: string;
  rangeTo?: string;
  rangeCustom?: boolean;
  /** True when no days/from/to was sent (lifetime aggregate). */
  lifetime?: boolean;
}

/** GET /insights/documents/:id/sessions — idle-gap reading session timeline. */
export interface DocumentReadingSessionPage {
  pageNumber: number;
  durationSeconds: number;
}

export interface DocumentReadingSession {
  id: string;
  linkId: string;
  visitorId: string;
  visitorEmail?: string;
  startedAt: string;
  lastActivityAt: string;
  endedAt?: string;
  maxPage: number;
  distinctPageCount: number;
  totalDurationSeconds: number;
  completed: boolean;
  pages: DocumentReadingSessionPage[];
}

export interface DocumentReadingSessions {
  documentId: string;
  pageCount: number;
  sessionModel: "reading_session";
  sessions: DocumentReadingSession[];
  rangeDays?: number;
  rangeFrom?: string;
  rangeTo?: string;
  rangeCustom?: boolean;
  /** True when no days/from/to was sent (lifetime aggregate). */
  lifetime?: boolean;
}

export interface SignalFeed {
  signals: Signal[];
  actions: ActionItem[];
}

export interface PublicLinkCredentials {
  email?: string;
  emailCode?: string;
  password?: string;
  ndaAgreed?: boolean;
  sessionToken?: string;
}

export interface CreateDealRoomLinkPayload {
  name?: string;
  require_email?: boolean;
  require_email_verification?: boolean;
  require_nda?: boolean;
  nda_document_id?: string;
  nda_template_id?: string;
  require_password?: boolean;
  password?: string;
  allowed_emails?: string[];
  blocked_emails?: string[];
  expires_at?: string;
  download_enabled?: boolean;
  watermark_enabled?: boolean;
  qa_enabled?: boolean;
  file_requests_enabled?: boolean;
  index_file_enabled?: boolean;
  screenshot_protection_enabled?: boolean;
  custom_domain?: string;
  tags?: string[];
  notify_on_access?: boolean;
  /** Folder allowlist when folder_scope_mode=allowlist. */
  folder_paths?: string[];
  /** "full" (whole room, default) or "allowlist". */
  folder_scope_mode?: "full" | "allowlist";
}

export interface UpsertDealRoomAccessPolicyPayload {
  require_email_verification_floor: boolean;
  require_nda_floor: boolean;
  blocked_emails: string[];
  /** Legacy aliases — same DB columns as floors; kept for older API builds. */
  require_email_verification?: boolean;
  require_nda?: boolean;
}

export interface SendMarketingBatchRequest {
  recipients: string[];
  subject: string;
  body?: string;
  headline?: string;
  cta_text?: string;
  cta_url?: string;
  preview_text?: string;
  template_variables?: Record<string, string>;
  track_opens?: boolean;
  track_clicks?: boolean;
}

export interface SendMarketingBatchResult {
  sent: number;
  failed: number;
  log_ids: string[];
  failed_recipients: { email: string; message: string }[];
}

function publicAccessHeaders(creds?: PublicLinkCredentials): Record<string, string> | undefined {
  if (!creds) return undefined;
  if (creds.sessionToken) {
    return { "X-Link-Session": creds.sessionToken };
  }
  if (!creds.email && !creds.emailCode && !creds.password && !creds.ndaAgreed) return undefined;
  const payload = {
    email: creds.email,
    email_code: creds.emailCode,
    password: creds.password,
    nda_agreed: creds.ndaAgreed,
  };
  return { "X-Link-Access": btoa(JSON.stringify(payload)) };
}

/** Normalize GET/PUT access-policy responses whether wrapped or unwrapped. */
function unwrapDealRoomAccessPolicy(
  res: DealRoomAccessPolicy | { data: DealRoomAccessPolicy } | null | undefined,
): DealRoomAccessPolicy {
  if (!res || typeof res !== "object") {
    return {
      dealRoomId: "",
      configured: false,
      blockedEmails: [],
    };
  }
  if ("dealRoomId" in res && typeof (res as DealRoomAccessPolicy).dealRoomId === "string") {
    return res as DealRoomAccessPolicy;
  }
  const nested = (res as { data?: DealRoomAccessPolicy }).data;
  if (nested && typeof nested === "object" && typeof nested.dealRoomId === "string") {
    return nested;
  }
  return {
    dealRoomId: "",
    configured: false,
    blockedEmails: [],
  };
}

function getWorkspaceSlug(): string {
  // Priority 1: from URL path (most reliable for page-level API calls)
  if (typeof window !== "undefined") {
    const match = window.location.pathname.match(/^\/([^/]+)/);
    if (match && match[1] && !match[1].startsWith("api") && !["login", "register", "viewer", "l", "r", "workspaces"].includes(match[1])) {
      return match[1];
    }
  }
  // Priority 2: from UI store (set after workspace selection)
  const slug = useUIStore.getState().currentWorkspace?.slug;
  if (slug) return slug;
  throw new Error("No workspace selected");
}

type KnowledgeSessionAskBody = {
  sessionId?: string;
  query: string;
  answer?: boolean;
  top_k?: number;
  /** Required idempotency key; server replays the same audited turn on retry. */
  clientRequestId: string;
};

async function streamDealRoomKnowledgeSession(
  roomId: string,
  body: KnowledgeSessionAskBody,
  opts: {
    signal?: AbortSignal;
    onEvent: (event: KnowledgeStreamEvent) => void;
  },
): Promise<DealRoomKnowledgeSessionQueryResult> {
  const response = await openStream(
    getWorkspaceSlug(),
    `/deal-rooms/${encodeURIComponent(roomId)}/knowledge/sessions/query/stream`,
    {
      method: "POST",
      body: JSON.stringify(body),
      signal: opts.signal,
      headers: { Accept: "text/event-stream" },
    },
  );

  const doneResult = await consumeKnowledgeSSE(response, {
    signal: opts.signal,
    onEvent: opts.onEvent,
    requireDone: true,
  });
  return doneResult!;
}

/** Coalesce concurrent deal-room list fetches per workspace+query (dashboard + list storms). */
const dealRoomsInflight = new Map<
  string,
  Promise<{
    data: DealRoom[];
    pagination?: {
      page: number;
      page_size: number;
      total: number;
      has_more: boolean;
    };
  }>
>();

export const api = {
  login: async (email: string, password: string) => {
    const res = await request<{ user: User; expires_in: number }>(
      undefined,
      "/auth/login",
      { method: "POST", body: JSON.stringify({ email, password }), skipAuth: true }
    );
    setCachedAccountEmail(res.user.email);
    return res.user;
  },
  register: async (email: string, password: string) => {
    const res = await request<{ user: User; expires_in: number }>(
      undefined,
      "/auth/register",
      { method: "POST", body: JSON.stringify({ email, password }), skipAuth: true }
    );
    setCachedAccountEmail(res.user.email);
    return res.user;
  },
  logout: async () => {
    try {
      await request<void>(undefined, "/auth/logout", {
        method: "POST",
      });
    } finally {
      clearCachedAccountEmail();
    }
  },
  /** Current authenticated account (owner viewer watermark uses login email). */
  getMe: async () => {
    const res = await request<{ user: User }>(undefined, "/auth/me");
    setCachedAccountEmail(res.user.email);
    return res.user;
  },
  refresh: async () => {
    await request<{ expires_in: number }>(
      undefined,
      "/auth/refresh",
      { method: "POST", skipAuth: true }
    );
  },

  verifyEmail: async (token: string) => {
    return request<{ code: string; message: string }>(undefined, `/auth/verify-email/${token}`, {
      skipAuth: true,
    });
  },

  getWorkspaces: () => request<{ data: Workspace[] }>(undefined, "/workspaces"),
  createWorkspace: (payload: { name: string; slug: string; brand_color?: string }) =>
    request<Workspace>(undefined, "/workspaces", {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  getDashboardStats: () =>
    request<DashboardStats>(getWorkspaceSlug(), "/dashboard/stats"),

  getDocuments: (
    filter?: DocumentFilter,
    category?: string,
    /** @deprecated Prefer explicit `category` (e.g. general | agreement). Legacy exclude flags remain for old clients. */
    opts?: { excludeDealRoom?: boolean; excludeAgreement?: boolean },
  ) => {
    const params = new URLSearchParams();
    if (filter && filter !== "all") params.set("filter", filter);
    if (category) params.set("category", category);
    if (opts?.excludeDealRoom) params.set("exclude_deal_room", "true");
    if (opts?.excludeAgreement) params.set("exclude_agreement", "true");
    const qs = params.toString();
    return request<{ data: Document[] }>(
      getWorkspaceSlug(),
      qs ? `/documents?${qs}` : "/documents"
    );
  },
  getDocumentById: (id: string) =>
    request<Document>(getWorkspaceSlug(), `/documents/${id}`),
  deleteDocument: (id: string) =>
    request<void>(getWorkspaceSlug(), `/documents/${id}`, { method: "DELETE" }),
  getDocumentDeleteImpact: (id: string) =>
    request<{ active_link_count: number; deal_room_count: number }>(
      getWorkspaceSlug(),
      `/documents/${id}/delete-impact`,
    ),
  archiveDocument: (id: string) =>
    request<Document>(getWorkspaceSlug(), `/documents/${id}/archive`, { method: "POST" }),
  unarchiveDocument: (id: string) =>
    request<Document>(getWorkspaceSlug(), `/documents/${id}/unarchive`, { method: "POST" }),
  updateDocumentCategory: (id: string, category: string) =>
    request<Document>(getWorkspaceSlug(), `/documents/${id}/category`, {
      method: "PATCH",
      body: JSON.stringify({ category }),
    }),

  getDocumentPages: async (id: string) => {
    const res = await request<{
      document_id: string;
      pages: { page_number: number; width: number; height: number }[];
      total: number;
    }>(getWorkspaceSlug(), `/documents/${id}/pages`);
    return {
      documentId: res.document_id,
      pages: res.pages.map((p) => ({
        pageNumber: p.page_number,
        width: p.width,
        height: p.height,
      })),
      total: res.total,
    };
  },
  getPageSignedUrl: (id: string, pageNumber: number, opts?: { signal?: AbortSignal }) =>
    request<{ page_number: number; image_url: string; expires_at: string; width: number; height: number }>(
      getWorkspaceSlug(),
      `/documents/${id}/pages/signed-url`,
      {
        method: "POST",
        body: JSON.stringify({ page_number: pageNumber }),
        signal: opts?.signal,
      }
    ),
  getDocumentDownloadUrl: (id: string) =>
    request<{ download_url: string; expires_at: string; filename: string; content_type: string }>(
      getWorkspaceSlug(),
      `/documents/${id}/download-url`
    ),

  accessPublicLink: (
    token: string,
    opts?: {
      email?: string;
      emailCode?: string;
      password?: string;
      ndaAgreed?: boolean;
      signerName?: string;
      sessionToken?: string;
      inviteToken?: string;
    }
  ) =>
    request<{
      link: {
        id: string;
        name?: string;
        permissionType: string;
        downloadEnabled: boolean;
        watermarkEnabled: boolean;
        watermarkText?: string;
        screenshotProtectionEnabled?: boolean;
        qaEnabled: boolean;
        visitorAskUnified?: boolean;
        fileRequestsEnabled: boolean;
        isBundle: boolean;
        dealRoomId?: string;
      };
      documents: { id: string; title: string; pageCount: number; sourceType: string; folderPath?: string }[];
      visitorId: string;
      requiresEmail: boolean;
      requiresEmailVerification: boolean;
      requiresPassword: boolean;
      requiresNda: boolean;
      sessionToken: string;
      ndaResponseId?: string;
      ndaCertificateId?: string;
      ndaTemplate?: {
        id: string;
        name: string;
        requireSignerName: boolean;
        sourceDocumentId: string;
        contentSha256?: string;
      };
    }>(undefined, `/v1/public/links/${token}`, {
      method: "POST",
      skipAuth: true,
      body: JSON.stringify({
        email: opts?.email,
        email_code: opts?.emailCode,
        password: opts?.password,
        nda_agreed: opts?.ndaAgreed ?? false,
        signer_name: opts?.signerName,
        invite_token: opts?.inviteToken,
      }),
      headers: opts?.sessionToken ? { "X-Link-Session": opts.sessionToken } : undefined,
    }),

  requestPublicLinkAccess: (
    token: string,
    opts: { email: string; reason: string; signerName?: string }
  ) =>
    request<{ id: string; email: string; status: string }>(undefined, `/v1/public/links/${token}/access-requests`, {
      method: "POST",
      skipAuth: true,
      body: JSON.stringify({
        email: opts.email,
        reason: opts.reason,
        signer_name: opts.signerName,
      }),
    }),

  checkPublicLinkEmail: (token: string, email: string) =>
    request<{ ok: boolean }>(undefined, `/v1/public/links/${token}/check-email`, {
      method: "POST",
      skipAuth: true,
      body: JSON.stringify({ email }),
    }),

  getPublicNDAPreview: (token: string, email?: string) => {
    const qs = email?.trim()
      ? `?email=${encodeURIComponent(email.trim())}`
      : "";
    return request<{
      ndaTemplate: {
        id: string;
        name: string;
        requireSignerName: boolean;
        sourceDocumentId: string;
        contentSha256?: string;
      };
      document: { id: string; title: string; pageCount: number; sourceType: string };
      previewImageUrl?: string;
      previewPageUrls?: string[];
      documentUrl?: string;
      previewUrl: string;
      expiresAt: string;
    }>(undefined, `/v1/public/links/${token}/nda${qs}`, {
      method: "GET",
      skipAuth: true,
    });
  },

  getPublicNDASignedDownloadPath: (token: string) =>
    `/api/v1/public/links/${token}/nda/signed`,

  listNDATemplates: (includeArchived = false) =>
    request<{ data: Array<{
      id: string;
      name: string;
      source_document_id: string;
      content_sha256: string;
      require_signer_name: boolean;
      status: string;
      response_count: number;
      link_count: number;
      created_at: string;
      updated_at: string;
    }> }>(
      getWorkspaceSlug(),
      `/nda/templates${includeArchived ? "?include_archived=true" : ""}`
    ),

  createNDATemplate: (documentId: string, name?: string) =>
    request<{ data: { id: string; name: string; source_document_id: string } }>(
      getWorkspaceSlug(),
      "/nda/templates",
      {
        method: "POST",
        body: JSON.stringify({ document_id: documentId, name }),
      }
    ),

  listNDATemplateResponses: (templateId: string) =>
    request<{ data: Array<{
      id: string;
      link_id: string;
      nda_template_id: string;
      email: string;
      signer_name: string;
      certificate_id: string;
      content_sha256: string;
      has_signed_file: boolean;
      signed_at: string;
      status: string;
    }> }>(getWorkspaceSlug(), `/nda/templates/${templateId}/responses`),

  downloadNDAResponse: (responseId: string) =>
    request<Blob>(getWorkspaceSlug(), `/nda/responses/${responseId}/download`, {
      method: "GET",
    }),

  // Public Visitor Ask (unified)
  createPublicAsk: (
    token: string,
    question: string,
    creds?: PublicLinkCredentials,
    opts?: { escalate?: boolean },
  ) =>
    request<{ data: PublicAskTurn }>(undefined, `/v1/public/links/${token}/ask`, {
      method: "POST",
      skipAuth: true,
      headers: publicAccessHeaders(creds),
      body: JSON.stringify({
        question,
        ...(opts?.escalate ? { escalate: true } : {}),
      }),
    }),
  listPublicAskTurns: (token: string, creds?: PublicLinkCredentials) =>
    request<{ data: PublicAskTurn[] }>(undefined, `/v1/public/links/${token}/ask/me`, {
      method: "GET",
      skipAuth: true,
      headers: publicAccessHeaders(creds),
    }),
  listPublicAskFAQs: (token: string, creds?: PublicLinkCredentials) =>
    request<{ data: PublicAskFAQ[] }>(undefined, `/v1/public/links/${token}/ask/faq`, {
      method: "GET",
      skipAuth: true,
      headers: publicAccessHeaders(creds),
    }),
  listPublicFormalAsk: (token: string, creds?: PublicLinkCredentials) =>
    request<{ data: PublicFormalAsk[] }>(undefined, `/v1/public/links/${token}/ask/formal`, {
      method: "GET",
      skipAuth: true,
      headers: publicAccessHeaders(creds),
    }),
  escalatePublicAskTurn: (token: string, turnId: string, creds?: PublicLinkCredentials) =>
    request<{ data: PublicAskTurn }>(
      undefined,
      `/v1/public/links/${token}/ask/${encodeURIComponent(turnId)}/escalate`,
      {
        method: "POST",
        skipAuth: true,
        headers: publicAccessHeaders(creds),
        body: JSON.stringify({}),
      },
    ),
  streamPublicAskTurn: (
    token: string,
    turnId: string,
    opts: {
      creds?: PublicLinkCredentials;
      signal?: AbortSignal;
      onEvent: (event: KnowledgeStreamEvent) => void;
    },
  ) =>
    openStream(undefined, `/v1/public/links/${token}/ask/${encodeURIComponent(turnId)}/stream`, {
      method: "GET",
      skipAuth: true,
      headers: publicAccessHeaders(opts.creds),
      signal: opts.signal,
    }).then((response) =>
      consumeKnowledgeSSE(response, {
        signal: opts.signal,
        onEvent: opts.onEvent,
        requireDone: true,
      }),
    ),

  // Public File Requests
  createPublicFileRequest: (token: string, message: string, creds?: PublicLinkCredentials) =>
    request<{ data: FileRequest }>(undefined, `/v1/public/links/${token}/file-requests`, {
      method: "POST",
      skipAuth: true,
      headers: publicAccessHeaders(creds),
      body: JSON.stringify({ message }),
    }),
  listPublicFileRequests: (token: string, creds?: PublicLinkCredentials) =>
    request<{ data: FileRequest[] }>(undefined, `/v1/public/links/${token}/file-requests/me`, {
      method: "GET",
      skipAuth: true,
      headers: publicAccessHeaders(creds),
    }),

  sendEmailVerificationCode: (token: string, email: string) =>
    request<void>(undefined, `/v1/public/links/${token}/send-email-code`, {
      method: "POST",
      skipAuth: true,
      body: JSON.stringify({ email }),
    }),

  getPublicDocumentPages: (documentId: string, token: string, creds?: PublicLinkCredentials, signal?: AbortSignal) =>
    request<{ documentId: string; pages: { pageNumber: number; width: number; height: number }[]; total: number }>(
      undefined,
      `/v1/public/documents/${documentId}/pages?token=${encodeURIComponent(token)}`,
      { skipAuth: true, headers: publicAccessHeaders(creds), signal }
    ).then((res) => ({
      documentId: res.documentId,
      pages: res.pages.map((p) => ({ pageNumber: p.pageNumber, width: p.width, height: p.height })),
      total: res.total,
    })),

  getPublicPageSignedUrl: (documentId: string, token: string, pageNumber: number, creds?: PublicLinkCredentials, signal?: AbortSignal) =>
    request<{ pageNumber: number; imageUrl: string; expiresAt: string; width: number; height: number }>(
      undefined,
      `/v1/public/documents/${documentId}/pages/signed-url?token=${encodeURIComponent(token)}&page_number=${pageNumber}`,
      { method: "GET", skipAuth: true, headers: publicAccessHeaders(creds), signal }
    ).then((res) => ({
      page_number: res.pageNumber,
      image_url: res.imageUrl,
      expires_at: res.expiresAt,
      width: res.width,
      height: res.height,
    })),

  getPublicDocumentDownloadUrl: (documentId: string, token: string, creds?: PublicLinkCredentials) =>
    request<{ downloadUrl: string; expiresAt: string; filename: string; contentType: string }>(
      undefined,
      `/v1/public/documents/${documentId}/download-url?token=${encodeURIComponent(token)}`,
      { skipAuth: true, headers: publicAccessHeaders(creds) }
    ).then((res) => ({
      download_url: res.downloadUrl,
      expires_at: res.expiresAt,
      filename: res.filename,
      content_type: res.contentType,
    })),

  recordPublicEvent: (
    payload: {
      event_type: string;
      public_token: string;
      visitor_id?: string;
      email?: string;
      document_id?: string;
      page_number?: number;
      duration_seconds?: number;
      scroll_depth?: number;
      reason?: string;
    },
    creds?: PublicLinkCredentials
  ) =>
    request<void>(undefined, "/v1/public/events", {
      method: "POST",
      body: JSON.stringify({
        ...payload,
        email: creds?.email ?? payload.email,
        password: creds?.password,
        nda_agreed: creds?.ndaAgreed,
      }),
      skipAuth: true,
      headers: publicAccessHeaders(creds),
    }),

  recordViewerEvent: (payload: {
    documentId: string;
    eventType: "page_viewed" | "download_attempted";
    pageNumber?: number;
    durationSeconds?: number;
    scrollDepth?: number;
  }) =>
    request<void>(getWorkspaceSlug(), "/events", {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  uploadDocument: (
    file: File,
    category?: string,
    opts?: { replace?: boolean },
  ) => {
    const formData = new FormData();
    formData.append("file", file);
    if (category) formData.append("category", category);
    if (opts?.replace) formData.append("replace", "true");
    return request<Document>(getWorkspaceSlug(), "/documents", {
      method: "POST",
      body: formData,
    });
  },

  createLink: (documentIds: string[], config: PermissionConfig) =>
    request<Link>(getWorkspaceSlug(), "/links", {
      method: "POST",
      body: JSON.stringify(toCreateLinkPayload(documentIds, config)),
    }),

  updateLinkFull: (id: string, payload: UpdateLinkPayload) =>
    request<Link>(getWorkspaceSlug(), `/links/${id}`, {
      method: "PUT",
      body: JSON.stringify(payload),
    }),

  getLinks: () =>
    request<{ data: Link[] }>(getWorkspaceSlug(), "/links"),
  getLinkById: (id: string) =>
    request<Link>(getWorkspaceSlug(), `/links/${id}`),
  getLinksByDocumentId: (documentId: string) =>
    request<{ data: Link[] }>(
      getWorkspaceSlug(),
      `/links?documentId=${documentId}`
    ),
  updateLink: (id: string, patch: Partial<Link>) =>
    request<Link>(getWorkspaceSlug(), `/links/${id}`, {
      method: "PATCH",
      body: JSON.stringify(patch),
    }),
  deleteLink: (id: string) =>
    request<void>(getWorkspaceSlug(), `/links/${id}`, {
      method: "DELETE",
    }),

  getAccessLogs: (
    linkId: string,
    params: { limit?: number; offset?: number } = {},
  ) => {
    const search = new URLSearchParams();
    if (params.limit != null) search.set("limit", String(params.limit));
    if (params.offset != null) search.set("offset", String(params.offset));
    const qs = search.toString();
    return request<{ data: AccessLog[]; has_more?: boolean }>(
      getWorkspaceSlug(),
      `/links/${linkId}/access-logs${qs ? `?${qs}` : ""}`,
    );
  },
  getLinkAnalytics: (linkId: string) =>
    request<{ data: LinkAnalytics }>(getWorkspaceSlug(), `/links/${linkId}/analytics`),

  listLinkRecentVisitors: (
    linkId: string,
    params: { limit?: number; offset?: number } = {},
  ) => {
    const search = new URLSearchParams();
    if (params.limit != null) search.set("limit", String(params.limit));
    if (params.offset != null) search.set("offset", String(params.offset));
    const qs = search.toString();
    return request<{ data: LinkRecentVisitor[]; has_more?: boolean }>(
      getWorkspaceSlug(),
      `/links/${linkId}/analytics/visitors${qs ? `?${qs}` : ""}`,
    );
  },

  listLinkAccessCodeContacts: (
    linkId: string,
    params: { limit?: number; offset?: number } = {},
  ) => {
    const search = new URLSearchParams();
    if (params.limit != null) search.set("limit", String(params.limit));
    if (params.offset != null) search.set("offset", String(params.offset));
    const qs = search.toString();
    return request<{ data: LinkAccessCodeContact[]; has_more?: boolean }>(
      getWorkspaceSlug(),
      `/links/${linkId}/analytics/access-code-contacts${qs ? `?${qs}` : ""}`,
    );
  },

  resendLinkAccessCode: (linkId: string, email: string, force = false) =>
    request<void>(getWorkspaceSlug(), `/links/${linkId}/access-codes/resend`, {
      method: "POST",
      body: JSON.stringify({ email, force }),
    }),

  resendFailedLinkAccessCodes: (linkId: string) =>
    request<{
      data: {
        attempted: number;
        sent: number;
        failed: number;
        skipped: number;
        errors?: string[];
      };
    }>(getWorkspaceSlug(), `/links/${linkId}/access-codes/resend-failed`, {
      method: "POST",
    }),

  // Deal-room access policy (Room Security source of truth).
  getDealRoomAccessPolicy: async (roomId: string) => {
    const res = await request<DealRoomAccessPolicy | { data: DealRoomAccessPolicy }>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/access-policy`,
    );
    return { data: unwrapDealRoomAccessPolicy(res) };
  },
  upsertDealRoomAccessPolicy: async (
    roomId: string,
    payload: UpsertDealRoomAccessPolicyPayload,
  ) => {
    const res = await request<DealRoomAccessPolicy | { data: DealRoomAccessPolicy }>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/access-policy`,
      { method: "PUT", body: JSON.stringify(payload) },
    );
    return { data: unwrapDealRoomAccessPolicy(res) };
  },

  // Deal-room share links.
  createDealRoomLink: (roomId: string, payload: CreateDealRoomLinkPayload) =>
    request<Link>(getWorkspaceSlug(), `/deal-rooms/${roomId}/links`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  getDealRoomLinks: (
    roomId: string,
    opts?: {
      page?: number;
      page_size?: number;
      /** created_at_desc (default) | created_at_asc */
      sort?: "created_at_desc" | "created_at_asc";
      q?: string;
    },
  ) => {
    const params = new URLSearchParams();
    if (opts?.page != null) params.set("page", String(opts.page));
    if (opts?.page_size != null) params.set("page_size", String(opts.page_size));
    if (opts?.sort) params.set("sort", opts.sort);
    if (opts?.q) params.set("q", opts.q);
    const qs = params.toString();
    return request<{
      data: Link[];
      pagination?: {
        page: number;
        page_size: number;
        total: number;
        has_more: boolean;
      };
    }>(getWorkspaceSlug(), `/deal-rooms/${roomId}/links${qs ? `?${qs}` : ""}`);
  },

  // Link access rules.
  getLinkAccessRules: (linkId: string) =>
    request<{ data: AccessRule[] }>(getWorkspaceSlug(), `/links/${linkId}/access-rules`),
  setLinkAccessRules: (linkId: string, rules: AccessRule[]) =>
    request<void>(getWorkspaceSlug(), `/links/${linkId}/access-rules`, {
      method: "POST",
      body: JSON.stringify({ rules }),
    }),

  // Link access requests (visitor authorization applications).
  // scope defaults to document — never fetch an unscoped inbox (PII boundary).
  getPendingLinkAccessRequests: (opts?: {
    scope?: "document" | "deal_room";
    dealRoomId?: string;
  }) => {
    const params = new URLSearchParams();
    const scope = opts?.scope ?? "document";
    params.set("scope", scope);
    if (scope === "deal_room") {
      const roomId = opts?.dealRoomId?.trim();
      if (!roomId) {
        return Promise.reject(new Error("dealRoomId is required when scope=deal_room"));
      }
      params.set("deal_room_id", roomId);
    }
    return request<{ data: PendingLinkAccessRequest[] }>(
      getWorkspaceSlug(),
      `/links/pending-access-requests?${params.toString()}`,
    );
  },
  getLinkAccessRequests: (linkId: string) =>
    request<{ data: LinkAccessRequest[] }>(getWorkspaceSlug(), `/links/${linkId}/access-requests`),
  approveLinkAccessRequest: (linkId: string, requestId: string) =>
    request<{
      data: LinkAccessRequest;
      /** Soft warning when approval committed but verification email failed. */
      warning?: { code: string; message: string };
    }>(
      getWorkspaceSlug(),
      `/links/${linkId}/access-requests/${requestId}/approve`,
      { method: "POST" }
    ),
  rejectLinkAccessRequest: (linkId: string, requestId: string) =>
    request<{ data: LinkAccessRequest }>(
      getWorkspaceSlug(),
      `/links/${linkId}/access-requests/${requestId}/reject`,
      { method: "POST" }
    ),

  // Owner Ask inbox (unified turns)
  listLinkAsk: (linkId: string, params: { lane?: string; status?: string } = {}) => {
    const search = new URLSearchParams();
    if (params.lane) search.set("lane", params.lane);
    if (params.status) search.set("status", params.status);
    const qs = search.toString();
    return request<{ data: OwnerAskTurn[] }>(
      getWorkspaceSlug(),
      `/links/${linkId}/ask${qs ? `?${qs}` : ""}`,
    );
  },
  listLinkAskPinnedFAQ: (linkId: string) =>
    request<{ data: OwnerAskTurn[] }>(getWorkspaceSlug(), `/links/${linkId}/ask/faq`),
  reorderLinkAskFAQ: (linkId: string, turnIds: string[]) =>
    request<{ data: OwnerAskTurn[] }>(getWorkspaceSlug(), `/links/${linkId}/ask/faq/order`, {
      method: "PATCH",
      body: JSON.stringify({ turn_ids: turnIds }),
    }),
  listRoomAsk: (roomId: string, params: { linkId?: string; lane?: string; status?: string } = {}) => {
    const search = new URLSearchParams();
    if (params.linkId) search.set("link_id", params.linkId);
    if (params.lane) search.set("lane", params.lane);
    if (params.status) search.set("status", params.status);
    const qs = search.toString();
    return request<{ data: OwnerAskTurn[] }>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/ask${qs ? `?${qs}` : ""}`,
    );
  },
  listRoomAskPinnedFAQ: (roomId: string, params: { linkId?: string } = {}) => {
    const search = new URLSearchParams();
    if (params.linkId) search.set("link_id", params.linkId);
    const qs = search.toString();
    return request<{ data: OwnerAskTurn[] }>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/ask/faq${qs ? `?${qs}` : ""}`,
    );
  },
  answerAskTurn: (linkId: string, turnId: string, answer: string) =>
    request<{ data: OwnerAskTurn }>(getWorkspaceSlug(), `/links/${linkId}/ask/${turnId}/host-answer`, {
      method: "PATCH",
      body: JSON.stringify({ answer }),
    }),
  publishFormalAskTurn: (
    linkId: string,
    turnId: string,
    body: { answer: string; publishAt?: string; anonymize?: boolean },
  ) =>
    request<{ data: OwnerAskTurn }>(
      getWorkspaceSlug(),
      `/links/${linkId}/ask/${turnId}/formal-publish`,
      {
        method: "PATCH",
        body: JSON.stringify({
          answer: body.answer,
          publish_at: body.publishAt,
          anonymize: body.anonymize,
        }),
      },
    ),
  pinAskTurnFAQ: (linkId: string, turnId: string) =>
    request<{ data: OwnerAskTurn }>(getWorkspaceSlug(), `/links/${linkId}/ask/${turnId}/pin-faq`, {
      method: "POST",
    }),
  unpinAskTurnFAQ: (linkId: string, turnId: string) =>
    request<{ data: OwnerAskTurn }>(getWorkspaceSlug(), `/links/${linkId}/ask/${turnId}/unpin-faq`, {
      method: "POST",
    }),
  updateLinkAskPolicy: (
    linkId: string,
    patch: {
      askAiEnabled?: boolean;
      askMode?: string;
      askAiMonthlyQuota?: number;
      clearAiQuota?: boolean;
    },
  ) =>
    request<{
      data: {
        id: string;
        ask_mode: string;
        ask_ai_enabled: boolean;
        ask_ai_monthly_quota: number | null;
        ask_ai_monthly_used?: number;
        ask_ai_monthly_limit?: number;
        ask_ai_quota_exceeded?: boolean;
        ask_ai_entitled?: boolean;
        formal_entitled?: boolean;
      };
    }>(getWorkspaceSlug(), `/links/${linkId}/ask-policy`, {
      method: "PATCH",
      body: JSON.stringify({
        ask_ai_enabled: patch.askAiEnabled,
        ask_mode: patch.askMode,
        ask_ai_monthly_quota: patch.askAiMonthlyQuota,
        clear_ai_quota: patch.clearAiQuota,
      }),
    }).then((res) => ({
      data: {
        id: res.data.id,
        askMode: res.data.ask_mode,
        askAiEnabled: res.data.ask_ai_enabled,
        askAiMonthlyQuota: res.data.ask_ai_monthly_quota,
        askAiMonthlyUsed: res.data.ask_ai_monthly_used,
        askAiMonthlyLimit: res.data.ask_ai_monthly_limit,
        askAiQuotaExceeded: res.data.ask_ai_quota_exceeded,
        askAiEntitled: res.data.ask_ai_entitled,
        formalEntitled: res.data.formal_entitled,
      },
    })),

  getLinkAskPolicy: (linkId: string) =>
    request<{
      data: {
        id: string;
        ask_mode: string;
        ask_ai_enabled: boolean;
        ask_ai_monthly_quota: number | null;
        ask_ai_monthly_used: number;
        ask_ai_monthly_limit: number;
        ask_ai_quota_exceeded: boolean;
        ask_ai_entitled?: boolean;
        formal_entitled?: boolean;
      };
    }>(getWorkspaceSlug(), `/links/${linkId}/ask-policy`).then((res) => ({
      data: {
        id: res.data.id,
        askMode: res.data.ask_mode,
        askAiEnabled: res.data.ask_ai_enabled,
        askAiMonthlyQuota: res.data.ask_ai_monthly_quota,
        askAiMonthlyUsed: res.data.ask_ai_monthly_used,
        askAiMonthlyLimit: res.data.ask_ai_monthly_limit,
        askAiQuotaExceeded: res.data.ask_ai_quota_exceeded,
        askAiEntitled: res.data.ask_ai_entitled,
        formalEntitled: res.data.formal_entitled,
      },
    })),

  // File Requests
  listLinkFileRequests: (linkId: string) =>
    request<{ data: FileRequest[] }>(getWorkspaceSlug(), `/links/${linkId}/file-requests`),
  updateFileRequestStatus: (linkId: string, requestId: string, status: string) =>
    request<{ data: FileRequest }>(getWorkspaceSlug(), `/links/${linkId}/file-requests/${requestId}/status`, {
      method: "PATCH",
      body: JSON.stringify({ status }),
    }),

  // Visitor Ask high-risk security events (link + room)
  listLinkAskSecurityEvents: (
    linkId: string,
    params: {
      limit?: number;
      offset?: number;
      eventType?: string;
      since?: string;
      until?: string;
    } = {},
  ) => {
    const search = new URLSearchParams();
    if (params.limit != null) search.set("limit", String(params.limit));
    if (params.offset != null) search.set("offset", String(params.offset));
    if (params.eventType) search.set("event_type", params.eventType);
    if (params.since) search.set("since", params.since);
    if (params.until) search.set("until", params.until);
    const qs = search.toString();
    return request<{ data: AskSecurityEvent[]; has_more: boolean }>(
      getWorkspaceSlug(),
      `/links/${linkId}/ask-security-events${qs ? `?${qs}` : ""}`,
    );
  },
  listRoomAskSecurityEvents: (
    roomId: string,
    params: {
      linkId?: string;
      limit?: number;
      offset?: number;
      eventType?: string;
      since?: string;
      until?: string;
    } = {},
  ) => {
    const search = new URLSearchParams();
    if (params.linkId) search.set("link_id", params.linkId);
    if (params.limit != null) search.set("limit", String(params.limit));
    if (params.offset != null) search.set("offset", String(params.offset));
    if (params.eventType) search.set("event_type", params.eventType);
    if (params.since) search.set("since", params.since);
    if (params.until) search.set("until", params.until);
    const qs = search.toString();
    return request<{ data: AskSecurityEvent[]; has_more: boolean }>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/ask-security-events${qs ? `?${qs}` : ""}`,
    );
  },

  /** Prefer passing route `workspaceSlug` so requests never follow a stale window.location. */
  getContacts: (workspaceSlug?: string) =>
    request<{ data: Contact[] }>(workspaceSlug ?? getWorkspaceSlug(), "/contacts"),
  createContact: (payload: { email: string; name?: string }, workspaceSlug?: string) =>
    request<Contact>(workspaceSlug ?? getWorkspaceSlug(), "/contacts", {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  getContactById: (id: string, workspaceSlug?: string) =>
    request<Contact>(workspaceSlug ?? getWorkspaceSlug(), `/contacts/${id}`),
  getActivitiesByContactId: (contactId: string, workspaceSlug?: string) =>
    request<{ data: Activity[] }>(
      workspaceSlug ?? getWorkspaceSlug(),
      `/contacts/${contactId}/activities`,
    ),

  getDealRooms: (opts?: { page?: number; page_size?: number; q?: string }) => {
    const slug = getWorkspaceSlug();
    const params = new URLSearchParams();
    if (opts?.page != null) params.set("page", String(opts.page));
    if (opts?.page_size != null) params.set("page_size", String(opts.page_size));
    const q = opts?.q?.trim();
    if (q) params.set("q", q);
    const qs = params.toString();
    const path = qs ? `/deal-rooms?${qs}` : "/deal-rooms";
    const inflightKey = `${slug}:${qs || "all"}`;
    const existing = dealRoomsInflight.get(inflightKey);
    if (existing) {
      return existing;
    }
    const pending = request<{
      data: DealRoom[];
      pagination?: {
        page: number;
        page_size: number;
        total: number;
        has_more: boolean;
      };
    }>(slug, path).finally(() => {
      if (dealRoomsInflight.get(inflightKey) === pending) {
        dealRoomsInflight.delete(inflightKey);
      }
    });
    dealRoomsInflight.set(inflightKey, pending);
    return pending;
  },
  getDealRoomById: (id: string) =>
    request<DealRoom>(getWorkspaceSlug(), `/deal-rooms/${id}`),
  getDealRoomAnalytics: (roomId: string) =>
    request<DealRoomAnalytics>(getWorkspaceSlug(), `/deal-rooms/${roomId}/analytics`),
  getDealRoomKnowledge: (roomId: string) =>
    request<DealRoomKnowledgeCorpus>(getWorkspaceSlug(), `/deal-rooms/${roomId}/knowledge`),
  syncDealRoomKnowledge: (roomId: string) =>
    request<{ status: string }>(getWorkspaceSlug(), `/deal-rooms/${roomId}/knowledge/sync`, {
      method: "POST",
    }),
  queryDealRoomKnowledge: (
    roomId: string,
    body: { query: string; answer?: boolean; top_k?: number },
  ) =>
    request<DealRoomKnowledgeQueryResult>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/knowledge/query`,
      { method: "POST", body: JSON.stringify(body) },
    ),
  getActiveDealRoomKnowledgeSession: (roomId: string) =>
    request<DealRoomKnowledgeSessionDetail>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/knowledge/sessions/active`,
    ),
  listDealRoomKnowledgeSessions: (
    roomId: string,
    params?: { limit?: number; cursor?: string },
  ) => {
    const qs = new URLSearchParams();
    if (params?.limit != null) qs.set("limit", String(params.limit));
    if (params?.cursor) qs.set("cursor", params.cursor);
    const suffix = qs.size > 0 ? `?${qs.toString()}` : "";
    return request<DealRoomKnowledgeSessionList>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/knowledge/sessions${suffix}`,
    );
  },
  getDealRoomKnowledgeSession: (roomId: string, sessionId: string) =>
    request<DealRoomKnowledgeSessionDetail>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/knowledge/sessions/${sessionId}`,
    ),
  createDealRoomKnowledgeSession: (roomId: string, body?: { title?: string }) =>
    request<DealRoomKnowledgeQASession>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/knowledge/sessions`,
      { method: "POST", body: JSON.stringify(body ?? {}) },
    ),
  queryDealRoomKnowledgeSession: (roomId: string, body: KnowledgeSessionAskBody) =>
    request<DealRoomKnowledgeSessionQueryResult>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/knowledge/sessions/query`,
      { method: "POST", body: JSON.stringify(body) },
    ),
  /**
   * Coarse SSE query (phase → sources → done). Prefer this for the Knowledge desk;
   * JSON `queryDealRoomKnowledgeSession` remains for non-streaming clients.
   */
  streamDealRoomKnowledgeSession: (
    roomId: string,
    body: KnowledgeSessionAskBody,
    opts: {
      signal?: AbortSignal;
      onEvent: (event: KnowledgeStreamEvent) => void;
    },
  ) => streamDealRoomKnowledgeSession(roomId, body, opts),
  closeDealRoomKnowledgeSession: (roomId: string, sessionId: string) =>
    request<DealRoomKnowledgeQASession>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/knowledge/sessions/${sessionId}/close`,
      { method: "POST" },
    ),
  upsertDealRoomKnowledgeTurnFeedback: (
    roomId: string,
    turnId: string,
    body: { kind: DealRoomKnowledgeFeedbackKind; note?: string },
  ) =>
    request<DealRoomKnowledgeTurnFeedback>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/knowledge/turns/${turnId}/feedback`,
      { method: "PUT", body: JSON.stringify(body) },
    ),
  /** Evidence-grounded follow-ups for a turn (async; falls back to templates server-side). */
  suggestDealRoomKnowledgeFollowUps: (
    roomId: string,
    turnId: string,
    opts?: { signal?: AbortSignal },
  ) =>
    request<DealRoomKnowledgeFollowUpsResult>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/knowledge/turns/${turnId}/follow-ups`,
      { method: "POST", signal: opts?.signal },
    ),
  listDealRoomKnowledgeMissions: (roomId: string) =>
    request<{ items: DealRoomKnowledgeMissionPack[] }>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/knowledge/missions`,
    ),
  getDealRoomKnowledgeMission: (roomId: string) =>
    request<DealRoomKnowledgeMissionPack>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/knowledge/mission`,
    ),
  setDealRoomKnowledgeMission: (roomId: string, body: { packId: string }) =>
    request<DealRoomKnowledgeMissionPack>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/knowledge/mission`,
      { method: "PUT", body: JSON.stringify(body) },
    ),
  /** Checklist coverage of the room mission pack vs optional session (ceiling Phase N). */
  getDealRoomKnowledgeMissionProgress: (
    roomId: string,
    params?: { sessionId?: string },
  ) => {
    const qs = new URLSearchParams();
    if (params?.sessionId) qs.set("sessionId", params.sessionId);
    const suffix = qs.size > 0 ? `?${qs.toString()}` : "";
    return request<DealRoomKnowledgeMissionProgress>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/knowledge/mission/progress${suffix}`,
    );
  },
  /** Diligence audit JSON pack for a live session (ceiling Phase H). */
  exportDealRoomKnowledgeSession: (roomId: string, sessionId: string) =>
    request<DealRoomKnowledgeDiligencePack>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/knowledge/sessions/${sessionId}/export`,
    ),
  listDealRoomKnowledgeArchives: (roomId: string, params?: { limit?: number }) => {
    const qs = new URLSearchParams();
    if (params?.limit != null) qs.set("limit", String(params.limit));
    const suffix = qs.size > 0 ? `?${qs.toString()}` : "";
    return request<DealRoomKnowledgeSessionArchiveList>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/knowledge/archives${suffix}`,
    );
  },
  getDealRoomKnowledgeArchive: (roomId: string, archiveId: string) =>
    request<DealRoomKnowledgeSessionArchiveDetail>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/knowledge/archives/${archiveId}`,
    ),
  listDealRoomKnowledgeEvalCandidates: (
    roomId: string,
    params?: { kind?: string; status?: string; limit?: number },
  ) => {
    const qs = new URLSearchParams();
    if (params?.kind) qs.set("kind", params.kind);
    if (params?.status) qs.set("status", params.status);
    if (params?.limit != null) qs.set("limit", String(params.limit));
    const suffix = qs.size > 0 ? `?${qs.toString()}` : "";
    return request<{ items: DealRoomKnowledgeEvalCandidate[] }>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/knowledge/eval/candidates${suffix}`,
    );
  },
  reviewDealRoomKnowledgeEvalCandidate: (
    roomId: string,
    candidateId: string,
    body: { reviewStatus: "accepted" | "rejected"; expect?: string },
  ) =>
    request<DealRoomKnowledgeEvalCandidate>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/knowledge/eval/candidates/${candidateId}`,
      { method: "PATCH", body: JSON.stringify(body) },
    ),
  exportDealRoomKnowledgeEvalCandidates: (roomId: string, params?: { limit?: number }) => {
    const qs = new URLSearchParams();
    if (params?.limit != null) qs.set("limit", String(params.limit));
    const suffix = qs.size > 0 ? `?${qs.toString()}` : "";
    return request<DealRoomKnowledgeEvalSeedExport>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/knowledge/eval/candidates/export${suffix}`,
    );
  },
  getDealRoomKnowledgeOps: (roomId: string, params?: { windowHours?: number }) => {
    const qs = new URLSearchParams();
    if (params?.windowHours != null) qs.set("windowHours", String(params.windowHours));
    const suffix = qs.size > 0 ? `?${qs.toString()}` : "";
    return request<DealRoomKnowledgeOpsSummary>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/knowledge/ops${suffix}`,
    );
  },
  /** Fire-and-forget product funnel signal (204). Errors are ignored by callers. */
  recordDealRoomKnowledgeDeskEvent: (
    roomId: string,
    body:
      | { type: "cite_open"; turnOutcome?: "grounded" | "refused" | "unknown" }
      | { type: "followups_upgrade_failed" },
  ) =>
    request<void>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/knowledge/events`,
      { method: "POST", body: JSON.stringify(body) },
    ),
  createDealRoom: (payload: {
    name: string;
    slug: string;
    description?: string;
    template?: string;
    ndaEnabled?: boolean;
    requiresApproval?: boolean;
  }) =>
    request<DealRoom>(getWorkspaceSlug(), "/deal-rooms", {
      method: "POST",
      body: JSON.stringify(toCreateDealRoomPayload(payload)),
    }),

  // Deal room folders
  getDealRoomFolders: (roomId: string) =>
    request<{ data: DealRoomFolder[] }>(getWorkspaceSlug(), `/deal-rooms/${roomId}/folders`),
  createDealRoomFolder: (roomId: string, payload: { name: string; parent_path?: string }) =>
    request<{ data: DealRoomFolder[] }>(getWorkspaceSlug(), `/deal-rooms/${roomId}/folders`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  renameDealRoomFolder: (roomId: string, path: string, payload: { name: string }) =>
    request<{ data: DealRoomFolder[] }>(getWorkspaceSlug(), `/deal-rooms/${roomId}/folders/${encodeURIComponent(path)}`, {
      method: "PATCH",
      body: JSON.stringify(payload),
    }),
  deleteDealRoomFolder: (roomId: string, path: string) =>
    request<{ data: DealRoomFolder[] }>(getWorkspaceSlug(), `/deal-rooms/${roomId}/folders/${encodeURIComponent(path)}`, {
      method: "DELETE",
    }),

  lockDealRoomResources: (
    roomId: string,
    payload: { folder_paths?: string[]; document_ids?: string[] },
  ) =>
    request<void>(getWorkspaceSlug(), `/deal-rooms/${roomId}/resources/lock`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  unlockDealRoomResources: (
    roomId: string,
    payload: { folder_paths?: string[]; document_ids?: string[] },
  ) =>
    request<void>(getWorkspaceSlug(), `/deal-rooms/${roomId}/resources/unlock`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  // Deal room documents
  getDealRoomDocuments: (roomId: string) =>
    request<{ data: DealRoomFolderDocs[] }>(getWorkspaceSlug(), `/deal-rooms/${roomId}/documents`),
  addDealRoomDocument: (roomId: string, payload: { document_id: string; folder_path?: string; sort_order?: number }) =>
    request<DealRoomDocumentItem>(getWorkspaceSlug(), `/deal-rooms/${roomId}/documents`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  updateDealRoomDocument: (roomId: string, docId: string, payload: { folder_path?: string; sort_order?: number }) =>
    request<DealRoomDocumentItem>(getWorkspaceSlug(), `/deal-rooms/${roomId}/documents/${docId}`, {
      method: "PATCH",
      body: JSON.stringify(payload),
    }),
  removeDealRoomDocument: (roomId: string, docId: string) =>
    request<void>(getWorkspaceSlug(), `/deal-rooms/${roomId}/documents/${docId}`, {
      method: "DELETE",
    }),

  // Deal room members
  getDealRoomMembers: (roomId: string) =>
    request<{ data: DealRoomMember[] }>(getWorkspaceSlug(), `/deal-rooms/${roomId}/members`),
  inviteDealRoomMember: (roomId: string, payload: { email: string; role: DealRoomMember["role"] }) =>
    request<{ data: DealRoomMember }>(getWorkspaceSlug(), `/deal-rooms/${roomId}/members`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  removeDealRoomMember: (roomId: string, memberId: string) =>
    request<void>(getWorkspaceSlug(), `/deal-rooms/${roomId}/members/${memberId}`, {
      method: "DELETE",
    }),

  // Deal room access requests
  getDealRoomAccessRequests: (roomId: string) =>
    request<{ data: DealRoomAccessRequest[] }>(getWorkspaceSlug(), `/deal-rooms/${roomId}/access-requests`),

  // Deal room folder permissions
  setDealRoomFolderPermission: (
    roomId: string,
    payload: { email: string; folder_path: string; permission: DealRoomFolderDocs["permission"] }
  ) =>
    request<{ data: { id: string; email: string; folder_path: string; permission: string } }>(
      getWorkspaceSlug(),
      `/deal-rooms/${roomId}/folder-permissions`,
      { method: "POST", body: JSON.stringify(payload) }
    ),
  approveDealRoomAccessRequest: (roomId: string, requestId: string) =>
    request<DealRoomAccessRequest>(getWorkspaceSlug(), `/deal-rooms/${roomId}/access-requests/${requestId}/approve`, {
      method: "POST",
    }),
  rejectDealRoomAccessRequest: (roomId: string, requestId: string) =>
    request<DealRoomAccessRequest>(getWorkspaceSlug(), `/deal-rooms/${roomId}/access-requests/${requestId}/reject`, {
      method: "POST",
    }),

  getInsightsOverview: (params: InsightsOverviewParams | InsightsRangeDays = 7) => {
    const q = typeof params === "number" ? { days: params } : params;
    const qs = new URLSearchParams();
    if (q.from && q.to) {
      qs.set("from", q.from);
      qs.set("to", q.to);
    } else {
      qs.set("days", String(q.days ?? 7));
    }
    return request<InsightsOverview>(
      getWorkspaceSlug(),
      `/insights/overview?${qs.toString()}`,
    );
  },
  getAccessAudit: (params: AccessAuditParams = {}) => {
    const qs = new URLSearchParams();
    if (params.from && params.to) {
      qs.set("from", params.from);
      qs.set("to", params.to);
    } else {
      qs.set("days", String(params.days ?? 7));
    }
    if (params.eventType) qs.set("eventType", params.eventType);
    if (params.dealRoomId) qs.set("dealRoomId", params.dealRoomId);
    if (params.memberId) qs.set("memberId", params.memberId);
    if (params.folderPath) qs.set("folderPath", params.folderPath);
    if (params.limit != null) qs.set("limit", String(params.limit));
    if (params.offset != null) qs.set("offset", String(params.offset));
    return request<AccessAudit>(
      getWorkspaceSlug(),
      `/insights/access-audit?${qs.toString()}`,
    );
  },
  getKeyPageCompliance: (params: KeyPageComplianceParams = {}) => {
    const qs = new URLSearchParams();
    if (params.from && params.to) {
      qs.set("from", params.from);
      qs.set("to", params.to);
    } else {
      qs.set("days", String(params.days ?? 30));
    }
    qs.set("circle", params.circle ?? "founder");
    if (params.limit != null) qs.set("limit", String(params.limit));
    if (params.offset != null) qs.set("offset", String(params.offset));
    return request<KeyPageCompliance>(
      getWorkspaceSlug(),
      `/insights/key-pages?${qs.toString()}`,
    );
  },
  getKeyPageSettings: () =>
    request<KeyPageSettings>(getWorkspaceSlug(), "/insights/key-page-settings"),
  saveKeyPageSettings: (body: KeyPageSettingsUpdate) =>
    request<KeyPageSettings>(getWorkspaceSlug(), "/insights/key-page-settings", {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  getPageAnalytics: (documentId: string, params?: InsightsOverviewParams) => {
    const qs = new URLSearchParams();
    if (params?.from && params?.to) {
      qs.set("from", params.from);
      qs.set("to", params.to);
    } else if (params?.days != null) {
      qs.set("days", String(params.days));
    }
    const suffix = qs.toString() ? `?${qs.toString()}` : "";
    return request<{
      data: PageAnalytics[];
      rangeDays?: number;
      rangeFrom?: string;
      rangeTo?: string;
      rangeCustom?: boolean;
      lifetime?: boolean;
    }>(getWorkspaceSlug(), `/insights/pages/${documentId}${suffix}`);
  },
  getDocumentVisitors: (documentId: string, params?: InsightsOverviewParams) => {
    const qs = new URLSearchParams();
    if (params?.from && params?.to) {
      qs.set("from", params.from);
      qs.set("to", params.to);
    } else if (params?.days != null) {
      qs.set("days", String(params.days));
    }
    const suffix = qs.toString() ? `?${qs.toString()}` : "";
    return request<{
      data: VisitorSummary[];
      rangeDays?: number;
      rangeFrom?: string;
      rangeTo?: string;
      rangeCustom?: boolean;
      lifetime?: boolean;
    }>(getWorkspaceSlug(), `/insights/documents/${documentId}/visitors${suffix}`);
  },
  getDocumentReadingFunnel: (documentId: string, params?: InsightsOverviewParams) => {
    const qs = new URLSearchParams();
    if (params?.from && params?.to) {
      qs.set("from", params.from);
      qs.set("to", params.to);
    } else if (params?.days != null) {
      qs.set("days", String(params.days));
    }
    const suffix = qs.toString() ? `?${qs.toString()}` : "";
    return request<DocumentReadingFunnel>(
      getWorkspaceSlug(),
      `/insights/documents/${documentId}/funnel${suffix}`,
    );
  },
  getDocumentReadingSessions: (
    documentId: string,
    limit = 40,
    params?: InsightsOverviewParams,
  ) => {
    const qs = new URLSearchParams();
    qs.set("limit", String(limit));
    if (params?.from && params?.to) {
      qs.set("from", params.from);
      qs.set("to", params.to);
    } else if (params?.days != null) {
      qs.set("days", String(params.days));
    }
    return request<DocumentReadingSessions>(
      getWorkspaceSlug(),
      `/insights/documents/${documentId}/sessions?${qs.toString()}`,
    );
  },
  getLinkHeatScore: (linkId: string, circle: "founder" | "investor_ir" | "sales" = "founder") =>
    request<LinkHeatScore>(
      getWorkspaceSlug(),
      `/analytics/links/${linkId}/score?circle=${encodeURIComponent(circle)}`,
    ),
  getSuggestions: () =>
    request<{ data: Suggestion[] }>(getWorkspaceSlug(), "/insights/suggestions"),
  /** Dismiss a workspace suggestion (link-scoped backend route). */
  dismissSuggestion: (linkId: string, suggestionId: string) =>
    request<void>(
      getWorkspaceSlug(),
      `/analytics/links/${linkId}/suggestions/${suggestionId}/dismiss`,
      { method: "POST" },
    ),
  /** Snooze a workspace suggestion (1d / 3d / 7d). Hides until snoozed_until; mirrors radar action snooze. */
  snoozeSuggestion: (suggestionId: string, hours: 24 | 72 | 168 = 24) =>
    request<{ id: string; snoozed_until?: string }>(
      getWorkspaceSlug(),
      `/insights/suggestions/${suggestionId}/snooze`,
      { method: "POST", body: JSON.stringify({ hours }) },
    ),

  getWorkspaceMembers: () =>
    request<{ data: WorkspaceMember[] }>(getWorkspaceSlug(), "/members"),
  inviteWorkspaceMember: (email: string, role: WorkspaceMember["role"]) =>
    request<{ data: WorkspaceInvitation }>(getWorkspaceSlug(), "/invitations", {
      method: "POST",
      body: JSON.stringify({ email, role }),
    }),

  sendMarketingBatch: (payload: SendMarketingBatchRequest, workspaceSlug?: string) =>
    request<{ data: SendMarketingBatchResult }>(
      workspaceSlug ?? getWorkspaceSlug(),
      "/marketing/send",
      {
        method: "POST",
        body: JSON.stringify(payload),
      },
    ),

  getWorkspaceSettings: () => request<WorkspaceSettings>(getWorkspaceSlug(), "/settings"),
  updateWorkspaceSettings: (settings: WorkspaceSettings) =>
    request<WorkspaceSettings>(getWorkspaceSlug(), "/settings", {
      method: "PUT",
      body: JSON.stringify(settings),
    }),

  uploadWorkspaceLogo: (file: File) => {
    const formData = new FormData();
    formData.append("file", file);
    return request<{ data: { logoUrl: string } }>(
      getWorkspaceSlug(),
      "/logo",
      {
        method: "POST",
        body: formData,
      }
    );
  },

  getBillingInfo: () => request<BillingInfo>(getWorkspaceSlug(), "/billing"),

  getIntegrations: async () => {
    const backend = await request<BackendIntegrationStatus>(
      getWorkspaceSlug(),
      "/integrations/settings",
    );
    return toIntegrationStatus(backend);
  },
  updateIntegrations: async (status: IntegrationStatus) => {
    const backend = await request<BackendIntegrationStatus>(
      getWorkspaceSlug(),
      "/integrations/settings",
      {
        method: "PUT",
        body: JSON.stringify(toBackendIntegrationStatus(status)),
      },
    );
    return toIntegrationStatus(backend);
  },

  connectSlack: () =>
    request<{ url: string }>(getWorkspaceSlug(), "/integrations/slack/connect", {
      method: "POST",
    }),
  disconnectSlack: () =>
    request<{ code: string; message: string }>(getWorkspaceSlug(), "/integrations/slack/disconnect", {
      method: "POST",
    }),
  connectHubSpot: () =>
    request<{ url: string }>(getWorkspaceSlug(), "/integrations/hubspot/connect", {
      method: "POST",
    }),
  disconnectHubSpot: () =>
    request<{ code: string; message: string }>(getWorkspaceSlug(), "/integrations/hubspot/disconnect", {
      method: "POST",
    }),

  getOutboundWebhook: async (): Promise<OutboundWebhookConfig> => {
    const backend = await request<BackendOutboundWebhook>(
      getWorkspaceSlug(),
      "/integrations/webhook",
    );
    return toOutboundWebhookConfig(backend);
  },
  saveOutboundWebhook: async (input: {
    url: string;
    enabled: boolean;
    eventTypes?: string[];
    rotateSecret?: boolean;
  }): Promise<OutboundWebhookConfig> => {
    const backend = await request<BackendOutboundWebhook>(
      getWorkspaceSlug(),
      "/integrations/webhook",
      {
        method: "PUT",
        body: JSON.stringify({
          url: input.url,
          enabled: input.enabled,
          event_types: input.eventTypes,
          rotate_secret: input.rotateSecret ?? false,
        }),
      },
    );
    return toOutboundWebhookConfig(backend);
  },
  deleteOutboundWebhook: () =>
    request<{ code: string; message: string }>(getWorkspaceSlug(), "/integrations/webhook", {
      method: "DELETE",
    }),

  getSecuritySettings: () => request<SecuritySettings>(getWorkspaceSlug(), "/security"),
  updateSecuritySettings: (settings: SecuritySettings) =>
    request<SecuritySettings>(getWorkspaceSlug(), "/security", {
      method: "PUT",
      body: JSON.stringify(settings),
    }),

  getSignals: () => request<SignalFeed>(getWorkspaceSlug(), "/signals"),
  getSignalById: (id: string) =>
    request<Signal>(getWorkspaceSlug(), `/signals/${id}`),
  /** Compiled Deal Radar feed (server-side productize + coalesce + rank). */
  getRadar: (opts?: { circle?: "founder" | "investor_ir" | "sales" }) => {
    const params = new URLSearchParams();
    if (opts?.circle && opts.circle !== "founder") {
      params.set("circle", opts.circle);
    }
    const q = params.toString();
    return request<RadarFeed>(
      getWorkspaceSlug(),
      q ? `/radar?${q}` : "/radar",
    );
  },
  getRadarEvidence: (itemId: string) =>
    request<RadarEvidencePack>(
      getWorkspaceSlug(),
      `/radar/items/${itemId}/evidence`,
    ),
  updateRadarItem: (
    id: string,
    status: ActionItem["status"],
    snoozeHours?: 24 | 72 | 168,
    outcome?: ActionItem["outcome"],
  ) =>
    request<ActionItem>(getWorkspaceSlug(), `/radar/items/${id}`, {
      method: "PATCH",
      body: JSON.stringify({
        status,
        ...(status === "snoozed" && snoozeHours
          ? { snooze_hours: snoozeHours }
          : {}),
        ...(status === "done" && outcome ? { outcome } : {}),
      }),
    }),
  updateActionStatus: (
    id: string,
    status: ActionItem["status"],
    snoozeHours?: 24 | 72 | 168,
    outcome?: ActionItem["outcome"],
  ) =>
    request<ActionItem>(getWorkspaceSlug(), `/signals/actions/${id}`, {
      method: "PATCH",
      body: JSON.stringify({
        status,
        ...(status === "snoozed" && snoozeHours
          ? { snooze_hours: snoozeHours }
          : {}),
        ...(status === "done" && outcome ? { outcome } : {}),
      }),
    }),

  getDealRoomTemplates: () =>
    request<{ data: DealRoomTemplate[] }>(getWorkspaceSlug(), "/deal-room-templates"),

  exportVisitorData: (email: string) =>
    request<{ data: Record<string, unknown> }>(
      getWorkspaceSlug(),
      `/compliance/data?visitor_email=${encodeURIComponent(email)}`
    ).then((res) => res.data),

  anonymizeVisitorData: (email: string) =>
    request<{ data: Record<string, number> }>(
      getWorkspaceSlug(),
      "/compliance/data",
      { method: "POST", body: JSON.stringify({ visitor_email: email }) }
    ).then((res) => res.data),

  deleteVisitorData: (email: string) =>
    request<{ data: Record<string, number> }>(
      getWorkspaceSlug(),
      `/compliance/data?visitor_email=${encodeURIComponent(email)}`,
      { method: "DELETE" }
    ).then((res) => res.data),
};
