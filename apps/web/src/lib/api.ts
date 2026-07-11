import type {
  AccessLog,
  AccessRule,
  ActionItem,
  Activity,
  BillingInfo,
  Contact,
  DealRoom,
  DealRoomAccessRequest,
  DealRoomDocumentItem,
  DealRoomFolder,
  DealRoomFolderDocs,
  DealRoomMember,
  DealRoomTemplate,
  Document,
  DocumentFilter,
  Evidence,
  HeatAlert,
  HeatLevel,
  IntegrationStatus,
  Link,
  LinkAccessRequest,
  LinkInvitation,
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
  VisitorQuestion,
  FileRequest,
} from "@/types";
import { request } from "@/lib/apiClient";
import {
  toBackendIntegrationStatus,
  toCreateDealRoomPayload,
  toCreateLinkPayload,
  toIntegrationStatus,
  type BackendIntegrationStatus,
  type UpdateLinkPayload,
} from "@/lib/apiAdapters";
import { useUIStore } from "@/stores/uiStore";

export interface DashboardStats {
  hotCount: number;
  warmCount: number;
  coldCount: number;
  recentDocuments: Document[];
  recentLinks: Link[];
  heatAlerts: HeatAlert[];
  riskAlerts: RiskAlert[];
  signals: Signal[];
  actionItems: ActionItem[];
}

export interface InsightsOverview {
  tierCounts: Record<HeatLevel, number>;
  topDocuments: { id: string; title: string; views: number; heatLevel: HeatLevel }[];
  topLinks: { id: string; shortUrl: string; views: number; heatLevel: HeatLevel }[];
  topContacts: { id: string; email: string; score: number; heatLevel: HeatLevel }[];
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

export interface PublicDealRoomView {
  room: {
    id: string;
    name: string;
    description: string;
    ndaEnabled: boolean;
    requiresApproval: boolean;
  };
  member: {
    id: string;
    email: string;
    role: DealRoomMember["role"];
    ndaStatus: DealRoomMember["nda_status"];
    status: DealRoomMember["status"];
  } | null;
  folders: DealRoomFolder[];
  documents: DealRoomFolderDocs[];
}

export interface CreateDealRoomLinkPayload {
  name?: string;
  require_email?: boolean;
  require_email_verification?: boolean;
  require_nda?: boolean;
  require_password?: boolean;
  password?: string;
  expires_at?: string;
  download_enabled?: boolean;
  watermark_enabled?: boolean;
  ai_copilot_enabled?: boolean;
  custom_domain?: string;
  tags?: string[];
  notify_on_access?: boolean;
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

function setTokens(accessToken: string, refreshToken: string) {
  localStorage.setItem("access_token", accessToken);
  localStorage.setItem("refresh_token", refreshToken);
}

function clearTokens() {
  localStorage.removeItem("access_token");
  localStorage.removeItem("refresh_token");
}

export const api = {
  login: async (email: string, password: string) => {
    const res = await request<{ user: User; access_token: string; refresh_token: string; expires_in: number }>(
      undefined,
      "/auth/login",
      { method: "POST", body: JSON.stringify({ email, password }), skipAuth: true }
    );
    setTokens(res.access_token, res.refresh_token);
    return res.user;
  },
  register: async (email: string, password: string) => {
    const res = await request<{ user: User; access_token: string; refresh_token: string; expires_in: number }>(
      undefined,
      "/auth/register",
      { method: "POST", body: JSON.stringify({ email, password }), skipAuth: true }
    );
    setTokens(res.access_token, res.refresh_token);
    return res.user;
  },
  logout: async () => {
    const refreshToken = localStorage.getItem("refresh_token");
    try {
      await request<void>(undefined, "/auth/logout", {
        method: "POST",
        body: JSON.stringify({ refresh_token: refreshToken }),
      });
    } finally {
      clearTokens();
    }
  },
  refresh: async () => {
    const refreshToken = localStorage.getItem("refresh_token");
    if (!refreshToken) throw new Error("No refresh token");
    const res = await request<{ access_token: string; refresh_token: string; expires_in: number }>(
      undefined,
      "/auth/refresh",
      { method: "POST", body: JSON.stringify({ refresh_token: refreshToken }), skipAuth: true }
    );
    setTokens(res.access_token, res.refresh_token);
    return res.access_token;
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

  getDocuments: (filter?: DocumentFilter, category?: string) => {
    const params = new URLSearchParams();
    if (filter && filter !== "all") params.set("filter", filter);
    if (category) params.set("category", category);
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
  getPageSignedUrl: (id: string, pageNumber: number) =>
    request<{ page_number: number; image_url: string; expires_at: string; width: number; height: number }>(
      getWorkspaceSlug(),
      `/documents/${id}/pages/signed-url`,
      { method: "POST", body: JSON.stringify({ page_number: pageNumber }) }
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
      sessionToken?: string;
      inviteToken?: string;
    }
  ) =>
    request<{
      link: { id: string; name?: string; permissionType: string; downloadEnabled: boolean; watermarkEnabled: boolean; aiCopilotEnabled: boolean; qaEnabled: boolean; fileRequestsEnabled: boolean; isBundle: boolean; dealRoomId?: string };
      documents: { id: string; title: string; pageCount: number; sourceType: string }[];
      visitorId: string;
      requiresEmail: boolean;
      requiresEmailVerification: boolean;
      requiresPassword: boolean;
      requiresNda: boolean;
      sessionToken: string;
    }>(undefined, `/v1/public/links/${token}`, {
      method: "POST",
      skipAuth: true,
      body: JSON.stringify({
        email: opts?.email,
        email_code: opts?.emailCode,
        password: opts?.password,
        nda_agreed: opts?.ndaAgreed ?? false,
        invite_token: opts?.inviteToken,
      }),
      headers: opts?.sessionToken ? { "X-Link-Session": opts.sessionToken } : undefined,
    }),

  // Public Visitor Q&A
  createPublicQuestion: (token: string, question: string, creds?: PublicLinkCredentials) =>
    request<{ data: VisitorQuestion }>(undefined, `/v1/public/links/${token}/questions`, {
      method: "POST",
      skipAuth: true,
      headers: publicAccessHeaders(creds),
      body: JSON.stringify({ question }),
    }),
  listPublicQuestions: (token: string, creds?: PublicLinkCredentials) =>
    request<{ data: VisitorQuestion[] }>(undefined, `/v1/public/links/${token}/questions/me`, {
      method: "GET",
      skipAuth: true,
      headers: publicAccessHeaders(creds),
    }),

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

  // Public access requests
  createLinkAccessRequest: (token: string, payload: { email: string; reason?: string }) =>
    request<{ data: LinkAccessRequest }>(undefined, `/v1/public/links/${token}/access-requests`, {
      method: "POST",
      skipAuth: true,
      body: JSON.stringify(payload),
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
      page_number?: number;
      duration_seconds?: number;
      scroll_depth?: number;
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

  uploadDocument: (file: File, category?: string) => {
    const formData = new FormData();
    formData.append("file", file);
    if (category) formData.append("category", category);
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

  getAccessLogs: (linkId: string) =>
    request<{ data: AccessLog[] }>(
      getWorkspaceSlug(),
      `/links/${linkId}/access-logs`
    ),

  // Deal-room share links.
  createDealRoomLink: (roomId: string, payload: CreateDealRoomLinkPayload) =>
    request<Link>(getWorkspaceSlug(), `/deal-rooms/${roomId}/links`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  getDealRoomLinks: (roomId: string) =>
    request<{ data: Link[] }>(getWorkspaceSlug(), `/deal-rooms/${roomId}/links`),

  // Link access rules.
  getLinkAccessRules: (linkId: string) =>
    request<{ data: AccessRule[] }>(getWorkspaceSlug(), `/links/${linkId}/access-rules`),
  setLinkAccessRules: (linkId: string, rules: AccessRule[]) =>
    request<void>(getWorkspaceSlug(), `/links/${linkId}/access-rules`, {
      method: "POST",
      body: JSON.stringify({ rules }),
    }),

  // Link invitations.
  getLinkInvitations: (linkId: string) =>
    request<{ data: LinkInvitation[] }>(getWorkspaceSlug(), `/links/${linkId}/invitations`),
  inviteLinkViewers: (linkId: string, emails: string[]) =>
    request<{ data: LinkInvitation[] }>(getWorkspaceSlug(), `/links/${linkId}/invitations`, {
      method: "POST",
      body: JSON.stringify({ emails }),
    }),
  revokeLinkInvitation: (
    linkId: string,
    invitationId: string,
    removeFromAllowList = true
  ) =>
    request<void>(
      getWorkspaceSlug(),
      `/links/${linkId}/invitations/${invitationId}/revoke`,
      {
        method: "POST",
        body: JSON.stringify({ removeFromAllowList }),
      }
    ),

  // Visitor Q&A
  listLinkQuestions: (linkId: string) =>
    request<{ data: VisitorQuestion[] }>(getWorkspaceSlug(), `/links/${linkId}/questions`),
  answerQuestion: (linkId: string, questionId: string, answer: string) =>
    request<{ data: VisitorQuestion }>(getWorkspaceSlug(), `/links/${linkId}/questions/${questionId}/answer`, {
      method: "PATCH",
      body: JSON.stringify({ answer }),
    }),

  // File Requests
  listLinkFileRequests: (linkId: string) =>
    request<{ data: FileRequest[] }>(getWorkspaceSlug(), `/links/${linkId}/file-requests`),
  updateFileRequestStatus: (linkId: string, requestId: string, status: string) =>
    request<{ data: FileRequest }>(getWorkspaceSlug(), `/links/${linkId}/file-requests/${requestId}/status`, {
      method: "PATCH",
      body: JSON.stringify({ status }),
    }),

  getContacts: () =>
    request<{ data: Contact[] }>(getWorkspaceSlug(), "/contacts"),
  createContact: (payload: { email: string; name?: string }) =>
    request<Contact>(getWorkspaceSlug(), "/contacts", {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  getContactById: (id: string) =>
    request<Contact>(getWorkspaceSlug(), `/contacts/${id}`),
  getActivitiesByContactId: (contactId: string) =>
    request<{ data: Activity[] }>(
      getWorkspaceSlug(),
      `/contacts/${contactId}/activities`
    ),

  getDealRooms: () =>
    request<{ data: DealRoom[] }>(getWorkspaceSlug(), "/deal-rooms"),
  getDealRoomById: (id: string) =>
    request<DealRoom>(getWorkspaceSlug(), `/deal-rooms/${id}`),
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

  // Public deal room
  getPublicDealRoom: (slug: string, email?: string) =>
    request<PublicDealRoomView>(undefined, `/v1/public/deal-rooms/${slug}${email ? `?email=${encodeURIComponent(email)}` : ""}`, {
      skipAuth: true,
    }),
  requestDealRoomAccess: (slug: string, payload: { email: string; reason?: string }) =>
    request<{ request_id: string }>(undefined, `/v1/public/deal-rooms/${slug}/access-requests`, {
      method: "POST",
      body: JSON.stringify(payload),
      skipAuth: true,
    }),
  signDealRoomNDA: (slug: string, payload: { email: string }) =>
    request<void>(undefined, `/v1/public/deal-rooms/${slug}/nda`, {
      method: "POST",
      body: JSON.stringify(payload),
      skipAuth: true,
    }),

  getInsightsOverview: () =>
    request<InsightsOverview>(getWorkspaceSlug(), "/insights/overview"),
  getPageAnalytics: (documentId: string) =>
    request<{ data: PageAnalytics[] }>(
      getWorkspaceSlug(),
      `/insights/pages/${documentId}`
    ),
  getDocumentVisitors: (documentId: string) =>
    request<{ data: VisitorSummary[] }>(
      getWorkspaceSlug(),
      `/insights/documents/${documentId}/visitors`
    ),
  getSuggestions: () =>
    request<{ data: Suggestion[] }>(getWorkspaceSlug(), "/insights/suggestions"),

  assistantChat: (payload: {
    message: string;
    session_id?: string;
  }) =>
    request<{
      session_id: string;
      answer: string;
      evidence?: Evidence[];
    }>(getWorkspaceSlug(), "/assistant/chat", {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  publicAssistantChat: (payload: { message: string; session_id?: string }, sessionToken: string) =>
    request<{
      session_id: string;
      answer: string;
      evidence?: Evidence[];
    }>(undefined, "/v1/public/assistant/chat", {
      method: "POST",
      body: JSON.stringify(payload),
      skipAuth: true,
      headers: { "X-Link-Session": sessionToken },
    }),

  searchDocument: (payload: {
    query: string;
    document_id?: string;
    mode?: "exact" | "fulltext" | "vector" | "hybrid";
    top_k?: number;
  }) =>
    request<{
      document_id?: string;
      query: string;
      results: Evidence[];
    }>(getWorkspaceSlug(), "/search", {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  getWorkspaceMembers: () =>
    request<{ data: WorkspaceMember[] }>(getWorkspaceSlug(), "/members"),
  inviteWorkspaceMember: (email: string, role: WorkspaceMember["role"]) =>
    request<{ data: WorkspaceInvitation }>(getWorkspaceSlug(), "/invitations", {
      method: "POST",
      body: JSON.stringify({ email, role }),
    }),

  sendMarketingBatch: (payload: SendMarketingBatchRequest) =>
    request<{ data: SendMarketingBatchResult }>(getWorkspaceSlug(), "/marketing/send", {
      method: "POST",
      body: JSON.stringify(payload),
    }),

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

  getSecuritySettings: () => request<SecuritySettings>(getWorkspaceSlug(), "/security"),
  updateSecuritySettings: (settings: SecuritySettings) =>
    request<SecuritySettings>(getWorkspaceSlug(), "/security", {
      method: "PUT",
      body: JSON.stringify(settings),
    }),

  getSignals: () => request<SignalFeed>(getWorkspaceSlug(), "/signals"),
  getSignalById: (id: string) =>
    request<Signal>(getWorkspaceSlug(), `/signals/${id}`),
  updateActionStatus: (id: string, status: ActionItem["status"]) =>
    request<ActionItem>(getWorkspaceSlug(), `/signals/actions/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ status }),
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

