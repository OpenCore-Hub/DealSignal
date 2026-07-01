export type HeatLevel = "hot" | "warm" | "cold";

export interface Workspace {
  id: string;
  slug: string;
  name: string;
  logoUrl?: string;
}

export interface User {
  id: string;
  email: string;
  name: string;
  avatarUrl?: string;
}

export interface IngestionJob {
  id: string;
  status: "queued" | "processing" | "completed" | "failed";
  attempts: number;
  errorMessage?: string | null;
}

export type DocumentStatus = "uploading" | "processing" | "ready" | "failed" | "archived";

export type DocumentFilter = "all" | "recent" | "popular" | "unshared" | "archived";

export interface Document {
  id: string;
  title: string;
  sourceType: "pdf" | "docx" | "pptx" | "xlsx";
  fileName: string;
  fileType: "pdf" | "docx" | "pptx" | "xlsx";
  fileSize: number;
  pageCount: number;
  status: DocumentStatus;
  progress?: number;
  createdAt: string;
  updatedAt: string;
  ingestionJob?: IngestionJob;
}

export interface Link {
  id: string;
  documentId: string;
  documentTitle: string;
  shortUrl: string;
  accessCount: number;
  heatLevel: HeatLevel;
  createdAt: string;
  expiresAt?: string;
  isActive?: boolean;
  avgDurationSeconds?: number;
  lastViewedAt?: string;
  permissionType?: "public" | "email" | "password" | "nda";
}

export interface HeatAlert {
  id: string;
  linkId: string;
  documentTitle: string;
  visitorEmail: string;
  heatLevel: HeatLevel;
  score: number;
  lastSeenAt: string;
  suggestion: string;
}

export interface PermissionConfig {
  level: "low" | "medium" | "high";
  requireEmailVerification: boolean;
  contactId?: string;
  whitelistEnabled: boolean;
  whitelist: string[];
  passwordEnabled: boolean;
  password?: string;
  ndaEnabled: boolean;
  allowDownload: boolean;
  watermarkEnabled: boolean;
  expiryDays: number | "custom";
  maxViews: number | "unlimited";
}

export interface ChatMessage {
  id: string;
  role: "user" | "assistant";
  content: string;
  evidences?: Evidence[];
  createdAt: string;
}

export interface Evidence {
  chunk_id: string;
  document_id?: string;
  quote: string;
  page_number: number;
  boxes: Array<{ x: number; y: number; w: number; h: number }>;
  score: number;
  match_type?: string;
}

export interface Contact {
  id: string;
  email: string;
  name: string;
  organization?: string;
  role?: string;
  heatLevel: HeatLevel;
  score: number;
  scoreHistory: { date: string; score: number }[];
  totalVisits: number;
  totalDurationSeconds: number;
  lastSeenAt?: string;
  viewedDocuments: string[];
}

export interface Activity {
  id: string;
  contactId: string;
  contactEmail: string;
  linkId: string;
  documentTitle: string;
  eventType: "open" | "page_view" | "revisit" | "download" | "share";
  pageNumber?: number;
  durationSeconds: number;
  timestamp: string;
  description: string;
}

export interface AccessLog {
  id: string;
  linkId: string;
  visitorEmail: string;
  visitorName?: string;
  pageNumber?: number;
  durationSeconds: number;
  device?: string;
  location?: string;
  timestamp: string;
}

export interface VisitorSummary {
  visitorId: string;
  visitorEmail: string;
  pageViewCount: number;
  avgDurationSeconds: number;
  lastSeenAt: string;
}

export interface PageAnalytics {
  pageNumber: number;
  viewCount: number;
  avgDurationSeconds: number;
  exitRate: number;
  /** 页面标题或提取的关键文本，用于匹配关键页 */
  title?: string;
}

export interface Suggestion {
  id: string;
  contactId: string;
  contactEmail: string;
  documentTitle: string;
  linkId: string;
  heatLevel: HeatLevel;
  score: number;
  reason: string;
  action: string;
  lastActivityAt: string;
}

export type DealRoomTemplateScenario =
  | "seed"
  | "series-a"
  | "series-b"
  | "lp-update"
  | "sales-proposal"
  | "ma"
  | "custom";

export interface DealRoomFolder {
  path: string;
  name: string;
  description?: string;
  sort_order: number;
}

export interface DealRoomDocumentItem {
  id: string;
  document_id: string;
  title: string;
  folder_path: string;
  sort_order: number;
  source_type: Document["sourceType"];
  status: Document["status"];
  page_count?: number;
  file_size?: number;
  created_at: string;
}

export interface DealRoomFolderDocs {
  folder: string;
  permission: "none" | "view" | "download" | "admin";
  documents: DealRoomDocumentItem[];
}

export type DealRoomMemberRole = "owner" | "admin" | "member" | "viewer";

export interface DealRoomMember {
  id: string;
  email: string;
  role: DealRoomMemberRole;
  nda_status: "none" | "pending" | "signed";
  status: "active" | "pending" | "suspended";
  name?: string;
  nda_signed_at?: string;
}

export interface DealRoomAccessRequest {
  id: string;
  email: string;
  status: "pending" | "approved" | "rejected";
  reason?: string;
  reviewed_at?: string;
}

export interface DealRoom {
  id: string;
  name: string;
  description: string;
  slug?: string;
  template: DealRoomTemplateScenario;
  documentCount: number;
  memberCount: number;
  pendingApprovals: number;
  ndaEnabled: boolean;
  requiresApproval?: boolean;
  isPublic?: boolean;
  createdAt: string;
  lastAccessedAt?: string;
  status: "active" | "archived" | "pending";
  uploadedFiles?: string[];
  recentVisitors?: { email: string; name?: string; heatLevel: HeatLevel; lastSeenAt: string }[];
  folders?: DealRoomFolder[];
  documents?: DealRoomFolderDocs[];
  members?: DealRoomMember[];
  accessRequests?: DealRoomAccessRequest[];
}

export interface WorkspaceMember {
  id: string;
  userId: string;
  email: string;
  name: string;
  role: "owner" | "admin" | "member" | "guest";
  joinedAt: string;
  status: "active" | "pending" | "suspended";
  avatarUrl?: string;
}

export interface WorkspaceInvitation {
  id: string;
  email: string;
  role: "owner" | "admin" | "member" | "guest";
  status: "pending" | "accepted" | "expired";
  expiresAt: string;
  createdAt: string;
}

export interface WorkspaceSettings {
  name: string;
  slug: string;
  brandColor: string;
  viewerDomain: string;
  logoUrl?: string;
}

export interface BillingInfo {
  plan: string;
  period: string;
  storageUsed: number;
  storageLimit: number;
  linksUsed: number;
  linksLimit: number;
  roomsUsed: number;
  roomsLimit: number;
}

export interface IntegrationStatus {
  slack: boolean;
  hubspot: boolean;
  zapier: boolean;
}

export interface SecuritySettings {
  forceEmailVerification: boolean;
  watermarkDownloads: boolean;
  twoFactorEnabled: boolean;
}

export interface AuditLog {
  id: string;
  actor: string;
  action: "upload" | "download" | "permission_change" | "member_invite" | "login" | "share";
  target: string;
  timestamp: string;
  ip?: string;
}

// v2.1.1 Signal-First 新增类型

export type SignalType = "hot" | "warm" | "cold" | "risk";
export type ActionType = "email" | "call" | "share" | "review";
export type ActionStatus = "pending" | "done" | "snoozed" | "ignored";
export type Priority = "high" | "medium" | "low";
export type Circle = "founder" | "investor_ir" | "sales";

export interface Signal {
  id: string;
  type: SignalType;
  title: string;
  description: string;
  explanation: string;
  suggestion: string;
  documentId?: string;
  contactId?: string;
  linkId?: string;
  createdAt: string;
  priority: Priority;
}

export interface ActionItem {
  id: string;
  signalId: string;
  title: string;
  impact: Priority;
  dueAt: string;
  status: ActionStatus;
  actionType: ActionType;
}

export interface HeatScoreWeights {
  opens: number;
  revisits: number;
  avgDurationMinutes: number;
  keyPageViews: number;
  forwardSignals: number;
  downloads: number;
  bouncePenalty: number;
}

export interface HeatScoreConfig {
  name: Circle;
  weights: HeatScoreWeights;
  keyPages: Record<string, string[]>;
  thresholds: {
    hot: number;
    warm: number;
    cold: number;
  };
}

export interface HeatScoreResult {
  score: number;
  level: HeatLevel;
  trend: "rising" | "stable" | "falling";
  breakdown: Record<string, number>;
  topKeyPages: string[];
}

export interface ContactProfile {
  id: string;
  email: string;
  name: string;
  organization?: string;
  role?: string;
  heatLevel: HeatLevel;
  score: number;
  scoreHistory: { date: string; score: number }[];
  relatedContacts: string[];
  notes?: string;
}

export interface DealRoomTemplate {
  id: string;
  name: string;
  description: string;
  scenario: DealRoomTemplateScenario;
  folderStructure: { name: string; description?: string }[];
  recommendedFiles: string[];
  defaultPermissionLevel: "low" | "medium" | "high";
  ndaEnabled: boolean;
}

export interface AIConversation {
  id: string;
  documentId: string;
  messages: {
    id: string;
    role: "user" | "assistant";
    content: string;
    evidences?: Evidence[];
    createdAt: string;
  }[];
}

export interface RiskAlert {
  id: string;
  type: "location" | "expired" | "download" | "forward";
  title: string;
  description: string;
  linkId?: string;
  documentId?: string;
  createdAt: string;
}
