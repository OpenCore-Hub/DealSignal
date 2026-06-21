import type {
  AccessLog,
  ActionItem,
  Activity,
  BillingInfo,
  Contact,
  DealRoom,
  DealRoomTemplate,
  Document,
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

export interface RequestOptions extends RequestInit {
  token?: string;
  idempotencyKey?: string;
}

async function request<T>(path: string, options?: RequestOptions): Promise<T> {
  const headers = new Headers(options?.headers);
  const body = options?.body;

  // 仅在非 FormData 请求体时自动设置 JSON Content-Type，避免破坏 multipart 上传
  if (!(body instanceof FormData)) {
    if (!headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }
  }

  // 预留认证与幂等扩展点
  if (options?.token) {
    headers.set("Authorization", `Bearer ${options.token}`);
  }
  if (options?.idempotencyKey) {
    headers.set("Idempotency-Key", options.idempotencyKey);
  }

  const response = await fetch(`/api${path}`, {
    ...options,
    headers,
  });
  if (!response.ok) {
    throw new Error(`API error: ${response.status} ${response.statusText}`);
  }
  return response.json() as Promise<T>;
}

export const api = {
  getWorkspaces: () => request<{ data: Workspace[] }>("/workspaces"),

  getDashboardStats: () => request<DashboardStats>("/dashboard/stats"),

  getDocuments: () => request<{ data: Document[] }>("/documents"),
  getDocumentById: (id: string) => request<Document>(`/documents/${id}`),
  deleteDocument: (id: string) => request<void>(`/documents/${id}`, { method: "DELETE" }),

  uploadDocument: (file: File) => {
    const formData = new FormData();
    formData.append("file", file);
    return request<Document>("/documents", { method: "POST", body: formData });
  },

  createLink: (documentId: string, config: PermissionConfig) =>
    request<Link>("/links", {
      method: "POST",
      body: JSON.stringify({ documentId, config }),
    }),

  getLinks: () => request<{ data: Link[] }>("/links"),
  getLinkById: (id: string) => request<Link>(`/links/${id}`),
  getLinksByDocumentId: (documentId: string) =>
    request<{ data: Link[] }>(`/links?documentId=${documentId}`),
  updateLink: (id: string, patch: Partial<Link>) =>
    request<Link>(`/links/${id}`, { method: "PATCH", body: JSON.stringify(patch) }),

  getAccessLogs: (linkId: string) =>
    request<{ data: AccessLog[] }>(`/links/${linkId}/access-logs`),

  getContacts: () => request<{ data: Contact[] }>("/contacts"),
  getContactById: (id: string) => request<Contact>(`/contacts/${id}`),
  getActivitiesByContactId: (contactId: string) =>
    request<{ data: Activity[] }>(`/contacts/${contactId}/activities`),

  getDealRooms: () => request<{ data: DealRoom[] }>("/deal-rooms"),
  getDealRoomById: (id: string) => request<DealRoom>(`/deal-rooms/${id}`),
  createDealRoom: (payload: {
    name: string;
    description: string;
    templateId: string;
    ndaEnabled: boolean;
  }) =>
    request<DealRoom>("/deal-rooms", {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  getInsightsOverview: () => request<InsightsOverview>("/insights/overview"),
  getPageAnalytics: (documentId: string) =>
    request<{ data: PageAnalytics[] }>(`/insights/pages/${documentId}`),
  getSuggestions: () => request<{ data: Suggestion[] }>("/insights/suggestions"),

  getWorkspaceMembers: () => request<{ data: WorkspaceMember[] }>("/workspace/members"),
  inviteWorkspaceMember: (email: string, role: WorkspaceMember["role"]) =>
    request<{ data: WorkspaceMember }>("/workspace/members", {
      method: "POST",
      body: JSON.stringify({ email, role }),
    }),

  getWorkspaceSettings: () => request<{ data: WorkspaceSettings }>("/workspace/settings"),
  updateWorkspaceSettings: (settings: WorkspaceSettings) =>
    request<{ data: WorkspaceSettings }>("/workspace/settings", {
      method: "PUT",
      body: JSON.stringify(settings),
    }),

  uploadWorkspaceLogo: (file: File) => {
    const formData = new FormData();
    formData.append("file", file);
    return request<{ data: { logoUrl: string } }>("/workspace/logo", {
      method: "POST",
      body: formData,
    });
  },

  getBillingInfo: () => request<{ data: BillingInfo }>("/workspace/billing"),

  getIntegrations: () => request<{ data: IntegrationStatus }>("/workspace/integrations"),
  updateIntegrations: (status: IntegrationStatus) =>
    request<{ data: IntegrationStatus }>("/workspace/integrations", {
      method: "PUT",
      body: JSON.stringify(status),
    }),

  getSecuritySettings: () => request<{ data: SecuritySettings }>("/workspace/security"),
  updateSecuritySettings: (settings: SecuritySettings) =>
    request<{ data: SecuritySettings }>("/workspace/security", {
      method: "PUT",
      body: JSON.stringify(settings),
    }),

  getSignals: () => request<SignalFeed>("/signals"),
  getSignalById: (id: string) => request<Signal>(`/signals/${id}`),

  getDealRoomTemplates: () => request<{ data: DealRoomTemplate[] }>("/deal-room-templates"),
};

// 工具函数请直接从 @/lib/formatters 或 @/lib/calculations 导入
// api.ts 仅保留网络层与业务方法，避免职责耦合

export function heatLabel(level: HeatLevel): string {
  const labels: Record<HeatLevel, string> = {
    hot: "高热度",
    warm: "中热度",
    cold: "低热度",
  };
  return labels[level];
}
