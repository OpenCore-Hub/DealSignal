import type {
  AccessLog,
  ActionItem,
  Activity,
  Contact,
  DealRoom,
  DealRoomTemplate,
  Document,
  HeatAlert,
  HeatLevel,
  Link,
  PageAnalytics,
  PermissionConfig,
  RiskAlert,
  Signal,
  Suggestion,
  WorkspaceMember,
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

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`/api${path}`, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });
  if (!response.ok) {
    throw new Error(`API error: ${response.status} ${response.statusText}`);
  }
  return response.json() as Promise<T>;
}

export const api = {
  getDashboardStats: () => request<DashboardStats>("/dashboard/stats"),

  getDocuments: () => request<{ data: Document[] }>("/documents"),
  getDocumentById: (id: string) => request<Document>(`/documents/${id}`),

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

  getSignals: () => request<SignalFeed>("/signals"),
  getSignalById: (id: string) => request<Signal>(`/signals/${id}`),

  getDealRoomTemplates: () => request<{ data: DealRoomTemplate[] }>("/deal-room-templates"),
};

export function formatFileSize(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / k ** i).toFixed(1))} ${sizes[i]}`;
}

export function formatDate(date: string): string {
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(date));
}

export function formatDuration(seconds: number): string {
  if (!seconds || seconds <= 0) return "-";
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  if (m === 0) return `${s}s`;
  return `${m}m ${s}s`;
}

export function formatRelativeTime(date?: string): string {
  if (!date) return "-";
  const now = new Date();
  const then = new Date(date);
  const diff = Math.floor((now.getTime() - then.getTime()) / 1000);
  if (diff < 60) return "刚刚";
  if (diff < 3600) return `${Math.floor(diff / 60)} 分钟前`;
  if (diff < 86400) return `${Math.floor(diff / 3600)} 小时前`;
  if (diff < 604800) return `${Math.floor(diff / 86400)} 天前`;
  return formatDate(date);
}

export function heatLabel(level: HeatLevel): string {
  const labels: Record<HeatLevel, string> = {
    hot: "高热度",
    warm: "中热度",
    cold: "低热度",
  };
  return labels[level];
}

export function getInitials(name: string): string {
  return name
    .split(" ")
    .map((n) => n[0])
    .join("")
    .toUpperCase()
    .slice(0, 2);
}

export function calculateUniqueVisitors(logs: { visitorEmail: string }[]): number {
  return new Set(logs.map((l) => l.visitorEmail)).size;
}

export function calculateHeatDistribution(contacts: { heatLevel: HeatLevel }[]): Record<HeatLevel, number> {
  return contacts.reduce(
    (acc, c) => {
      acc[c.heatLevel] = (acc[c.heatLevel] ?? 0) + 1;
      return acc;
    },
    { hot: 0, warm: 0, cold: 0 } as Record<HeatLevel, number>
  );
}

export function isOverdue(dueAt: string): boolean {
  return new Date(dueAt) < new Date();
}

export function daysOverdue(dueAt: string): number {
  const diff = new Date().getTime() - new Date(dueAt).getTime();
  return Math.max(0, Math.floor(diff / 86400000));
}

export function confidenceLabel(sampleCount: number): string {
  if (sampleCount >= 50) return "高置信度";
  if (sampleCount >= 10) return "中置信度";
  return "低置信度（样本较少）";
}
