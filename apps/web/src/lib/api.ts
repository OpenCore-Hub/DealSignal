import type {
  AccessLog,
  ActionItem,
  Activity,
  BillingInfo,
  Contact,
  DealRoom,
  DealRoomTemplate,
  Document,
  Evidence,
  HeatAlert,
  HeatLevel,
  IntegrationStatus,
  Link,
  PageAnalytics,
  PermissionConfig,
  RiskAlert,
  SecuritySettings,
  Signal,
  Suggestion,
  Workspace,
  WorkspaceMember,
  WorkspaceSettings,
} from "@/types";
import { request } from "@/lib/apiClient";
import {
  toCreateDealRoomPayload,
  toCreateLinkPayload,
} from "@/lib/apiAdapters";
import { useUIStore } from "@/stores/uiStore";
import i18next from "i18next";

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

function getWorkspaceSlug(): string {
  const slug = useUIStore.getState().currentWorkspace?.slug;
  if (!slug) {
    throw new Error("No workspace selected");
  }
  return slug;
}

export const api = {
  getWorkspaces: () => request<{ data: Workspace[] }>(undefined, "/workspaces"),
  createWorkspace: (payload: { name: string; slug: string; brand_color?: string }) =>
    request<Workspace>(undefined, "/workspaces", {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  getDashboardStats: () =>
    request<DashboardStats>(getWorkspaceSlug(), "/dashboard/stats"),

  getDocuments: () =>
    request<{ data: Document[] }>(getWorkspaceSlug(), "/documents"),
  getDocumentById: (id: string) =>
    request<Document>(getWorkspaceSlug(), `/documents/${id}`),
  deleteDocument: (id: string) =>
    request<void>(getWorkspaceSlug(), `/documents/${id}`, { method: "DELETE" }),

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

  accessPublicLink: (token: string, opts?: { email?: string; password?: string; ndaAgreed?: boolean }) => {
    const params = new URLSearchParams();
    if (opts?.email) params.set("email", opts.email);
    if (opts?.password) params.set("password", opts.password);
    if (opts?.ndaAgreed) params.set("nda_agreed", "true");
    const query = params.toString();
    return request<{
      link: { id: string; name?: string; documentId: string; permissionType: string; downloadEnabled: boolean; watermarkEnabled: boolean };
      document: { id: string; title: string; pageCount: number; status: string; sourceType: string; fileSize: number };
      visitorId: string;
      requiresEmail: boolean;
      requiresPassword: boolean;
      requiresNda: boolean;
    }>(undefined, `/v1/public/links/${token}${query ? `?${query}` : ""}`, {
      skipAuth: true,
    });
  },

  getPublicDocumentPages: (documentId: string, token: string) =>
    request<{ documentId: string; pages: { pageNumber: number; width: number; height: number }[]; total: number }>(
      undefined,
      `/v1/public/documents/${documentId}/pages?token=${encodeURIComponent(token)}`,
      { skipAuth: true }
    ).then((res) => ({
      documentId: res.documentId,
      pages: res.pages.map((p) => ({ pageNumber: p.pageNumber, width: p.width, height: p.height })),
      total: res.total,
    })),

  getPublicPageSignedUrl: (documentId: string, token: string, pageNumber: number) =>
    request<{ pageNumber: number; imageUrl: string; expiresAt: string; width: number; height: number }>(
      undefined,
      `/v1/public/documents/${documentId}/pages/signed-url?token=${encodeURIComponent(token)}&page_number=${pageNumber}`,
      { method: "POST", skipAuth: true }
    ).then((res) => ({
      page_number: res.pageNumber,
      image_url: res.imageUrl,
      expires_at: res.expiresAt,
      width: res.width,
      height: res.height,
    })),

  getPublicDocumentDownloadUrl: (documentId: string, token: string) =>
    request<{ downloadUrl: string; expiresAt: string; filename: string; contentType: string }>(
      undefined,
      `/v1/public/documents/${documentId}/download-url?token=${encodeURIComponent(token)}`,
      { skipAuth: true }
    ).then((res) => ({
      download_url: res.downloadUrl,
      expires_at: res.expiresAt,
      filename: res.filename,
      content_type: res.contentType,
    })),

  recordPublicEvent: (payload: {
    event_type: string;
    public_token: string;
    visitor_id?: string;
    email?: string;
    page_number?: number;
    duration_seconds?: number;
    scroll_depth?: number;
  }) =>
    request<void>(undefined, "/v1/public/events", {
      method: "POST",
      body: JSON.stringify(payload),
      skipAuth: true,
    }),

  uploadDocument: (file: File) => {
    const formData = new FormData();
    formData.append("file", file);
    return request<Document>(getWorkspaceSlug(), "/documents", {
      method: "POST",
      body: formData,
    });
  },

  createLink: (documentId: string, config: PermissionConfig) =>
    request<Link>(getWorkspaceSlug(), "/links", {
      method: "POST",
      body: JSON.stringify(toCreateLinkPayload(documentId, config)),
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

  getAccessLogs: (linkId: string) =>
    request<{ data: AccessLog[] }>(
      getWorkspaceSlug(),
      `/links/${linkId}/access-logs`
    ),

  getContacts: () =>
    request<{ data: Contact[] }>(getWorkspaceSlug(), "/contacts"),
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

  getInsightsOverview: () =>
    request<InsightsOverview>(getWorkspaceSlug(), "/insights/overview"),
  getPageAnalytics: (documentId: string) =>
    request<{ data: PageAnalytics[] }>(
      getWorkspaceSlug(),
      `/insights/pages/${documentId}`
    ),
  getSuggestions: () =>
    request<{ data: Suggestion[] }>(getWorkspaceSlug(), "/insights/suggestions"),

  assistantChat: (payload: {
    query: string;
    document_id?: string;
    session_id?: string;
    history?: { role: "user" | "assistant"; content: string }[];
  }) =>
    request<{
      session_id: string;
      answer: string;
      evidence?: Evidence[];
      follow_up_questions?: string[];
    }>(getWorkspaceSlug(), "/assistant/chat", {
      method: "POST",
      body: JSON.stringify(payload),
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
    request<{ data: WorkspaceMember[] }>(getWorkspaceSlug(), "/workspace/members"),
  inviteWorkspaceMember: (email: string, role: WorkspaceMember["role"]) =>
    request<{ data: WorkspaceMember }>(getWorkspaceSlug(), "/workspace/members", {
      method: "POST",
      body: JSON.stringify({ email, role }),
    }),

  getWorkspaceSettings: () =>
    request<{ data: WorkspaceSettings }>(getWorkspaceSlug(), "/workspace/settings"),
  updateWorkspaceSettings: (settings: WorkspaceSettings) =>
    request<{ data: WorkspaceSettings }>(getWorkspaceSlug(), "/workspace/settings", {
      method: "PUT",
      body: JSON.stringify(settings),
    }),

  uploadWorkspaceLogo: (file: File) => {
    const formData = new FormData();
    formData.append("file", file);
    return request<{ data: { logoUrl: string } }>(
      getWorkspaceSlug(),
      "/workspace/logo",
      {
        method: "POST",
        body: formData,
      }
    );
  },

  getBillingInfo: () =>
    request<{ data: BillingInfo }>(getWorkspaceSlug(), "/workspace/billing"),

  getIntegrations: () =>
    request<{ data: IntegrationStatus }>(getWorkspaceSlug(), "/workspace/integrations"),
  updateIntegrations: (status: IntegrationStatus) =>
    request<{ data: IntegrationStatus }>(getWorkspaceSlug(), "/workspace/integrations", {
      method: "PUT",
      body: JSON.stringify(status),
    }),

  getSecuritySettings: () =>
    request<{ data: SecuritySettings }>(getWorkspaceSlug(), "/workspace/security"),
  updateSecuritySettings: (settings: SecuritySettings) =>
    request<{ data: SecuritySettings }>(getWorkspaceSlug(), "/workspace/security", {
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
};

export function heatLabel(level: HeatLevel): string {
  return i18next.t(`common:heat.${level}`);
}
