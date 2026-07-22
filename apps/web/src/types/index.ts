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

export type DocumentCategory = "general" | "agreement";

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
  category?: DocumentCategory;
  progress?: number;
  createdAt: string;
  updatedAt: string;
  ingestionJob?: IngestionJob;
}

export interface DocumentSummary {
  id: string;
  title: string;
  sourceType: Document["sourceType"];
  pageCount: number;
  status: DocumentStatus;
  /** File size in bytes (available from v2.5+ backend). */
  fileSize?: number;
  /** Sort order in the link bundle. */
  sortOrder?: number;
}

export interface Link {
  id: string;
  documentId: string;
  documentIds: string[];
  folderPaths: string[];
  /** Deal-room folder scope mode. Missing/undefined treated as allowlist for new drafts. */
  folderScopeMode?: "full" | "allowlist";
  documentTitle: string;
  name?: string;
  shortUrl: string;
  accessCount: number;
  heatLevel: HeatLevel;
  status?: string;
  createdAt: string;
  expiresAt?: string;
  isActive?: boolean;
  avgDurationSeconds?: number;
  lastViewedAt?: string;
  permissionType?: "public" | "email" | "password" | "nda" | "whitelist";
  isBundle: boolean;
  documents: DocumentSummary[];
  downloadEnabled?: boolean;
  watermarkEnabled?: boolean;
  aiCopilotEnabled?: boolean;
  /** Q&A feature toggle (available from v2.7+ backend). */
  qaEnabled?: boolean;
  /** File request feature toggle (available from v2.7+ backend). */
  fileRequestsEnabled?: boolean;
  /** Index file feature toggle (available from v2.7+ backend). */
  indexFileEnabled?: boolean;
  /** Screenshot protection feature toggle (available from v2.7+ backend). */
  screenshotProtectionEnabled?: boolean;
  /** Soft warnings returned on create/update (e.g. Ask Docs auth ⊄ KB). */
  warnings?: Array<{
    code: string;
    message: string;
    missing_folder_paths?: string[];
    missing_document_ids?: string[];
  }>;
  requireEmailVerification?: boolean;
  maxAccessCount?: number;
  /** Allowed emails for whitelist (available from v2.5+ backend). */
  allowedEmails?: string[];
  /** Contact IDs attached to this link for email verification (available from v2.5+ backend). */
  contactIds?: string[];
  /** Explicit email collection requirement flag (available from v2.6+ backend). */
  requireEmail?: boolean;
  /** Explicit NDA requirement flag (available from v2.6+ backend). */
  requireNda?: boolean;
  /** NDA agreement document ID when requireNda is enabled. */
  ndaDocumentId?: string;
  /** Workspace NDA template ID when requireNda is enabled. */
  ndaTemplateId?: string;
  /** Explicit password requirement flag (available from v2.6+ backend). */
  requirePassword?: boolean;
  /** Deal room ID when this is a deal-room share link (available from v2.6+ backend). */
  dealRoomId?: string;
  /** Custom hostname used for the public link URL. */
  customDomain?: string;
  /** Link type: "share" or "file_request" (available from v2.8+ backend). */
  linkType?: "share" | "file_request";
  /** Target folder path for file-request links (available from v2.8+ backend). */
  targetFolderPath?: string;
  /** Whether the link creator should be emailed when the link is accessed. */
  notifyOnAccess?: boolean;
}

export interface VisitorQuestion {
  id: string;
  link_id: string;
  visitor_id: string;
  visitor_email?: string;
  question: string;
  answer?: string;
  answered_by?: string;
  status: "pending" | "answered";
  created_at: string;
  updated_at: string;
}

export interface FileRequest {
  id: string;
  link_id: string;
  visitor_id?: string;
  visitor_email?: string;
  message: string;
  status: "pending" | "approved" | "rejected" | "fulfilled";
  created_at: string;
  updated_at: string;
}

/** Link- or room-level Ask Docs audit list row. */
export interface AskDocsAuditEntry {
  session_id: string;
  link_id?: string;
  visitor_id?: string;
  question_preview: string;
  result_status?: string;
  evidence_count: number;
  created_at: string;
  archived: boolean;
}

export interface AskDocsAuditMessage {
  role: string;
  content: string;
  created_at: string;
}

/** Full Ask Docs audit session detail. */
export interface AskDocsAuditDetail {
  session_id: string;
  visitor_id?: string;
  created_at: string;
  archived: boolean;
  messages: AskDocsAuditMessage[];
  authorized_document_ids: string[];
  retrieval_document_ids: string[];
  evidence: Evidence[];
  result_status?: string;
}

/** Owner-visible Visitor Ask high-risk security event (US#32). */
export interface AskSecurityEvent {
  id: string;
  link_id: string;
  event_type: string;
  visitor_id?: string;
  email?: string;
  reason?: string;
  created_at: string;
}

export interface LinkAccessRequest {
  id: string;
  link_id: string;
  email: string;
  reason?: string;
  signer_name?: string;
  status: "pending" | "approved" | "rejected";
  created_at: string;
  updated_at: string;
}

export interface AccessRule {
  ruleType: "email";
  value: string;
  action: "allow" | "block";
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

export type PermissionPreset =
  | "public"
  | "standard"
  | "confidential"
  | "collaborative"
  | "customized";

export interface PermissionFields {
  requireEmailVerification: boolean;
  whitelistEnabled: boolean;
  whitelist: string[];
  passwordEnabled: boolean;
  password?: string;
  ndaEnabled: boolean;
  /** NDA agreement document ID for bundle/pipeline links. */
  ndaDocumentId?: string;
  allowDownload: boolean;
  watermarkEnabled: boolean;
  aiCopilotEnabled: boolean;
  qaEnabled: boolean;
  fileRequestsEnabled: boolean;
  indexFileEnabled: boolean;
  screenshotProtectionEnabled?: boolean;
  expiryDays: number | "custom";
  maxViews: number | "unlimited";
}

export interface PermissionConfig extends PermissionFields {
  level: PermissionPreset;
  isCustomized: boolean;
  /** Contact IDs attached to this link for email verification (supports multi-contact). */
  contactIds: string[];
  /** Pre-computed expiresAt (ISO 8601). Set by edit-mode reconstruction to prevent
   * round-trip drift from expiryDays → expiresAt → expiryDays conversion.
   * Cleared whenever expiryDays is changed by the user. */
  _editExpiresAt?: string;
}

export interface ChatMessage {
  id: string;
  role: "user" | "assistant";
  content: string;
  evidences?: Evidence[];
  createdAt: string;
  /** Ask Docs audit/result status from public assistant API. */
  resultStatus?: string;
  /** When true, UI may offer switching to Ask Host after a refusal. */
  suggestAskHost?: boolean;
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

export interface LinkAnalytics {
  total_views: number;
  unique_visitors: number;
  download_attempts: number;
  first_access_at?: string;
  last_access_at?: string;
  views_over_time: { day: string; views: number }[];
  average_duration_seconds: number;
  recent_visitors: {
    visitor_id: string;
    visitor_email?: string;
    first_access_at: string;
    last_access_at: string;
    total_views: number;
  }[];
  recent_visitors_has_more?: boolean;
  key_pages: {
    page_number: number;
    views: number;
    average_duration_seconds: number;
  }[];
  qa_records: {
    visitor_email?: string;
    question: string;
    answer?: string;
    created_at: string;
  }[];
  access_code_contacts?: {
    email: string;
    name?: string;
    send_status: "pending" | "sent" | "failed" | string;
    send_error?: string;
    code_sent_at?: string;
    used_at?: string;
    can_resend?: boolean;
  }[];
  access_code_contacts_has_more?: boolean;
  access_code_failed_count?: number;
  access_code_remediable_count?: number;
}

export type LinkAccessCodeContact = NonNullable<LinkAnalytics["access_code_contacts"]>[number];

export type LinkRecentVisitor = LinkAnalytics["recent_visitors"][number];

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
  | "startup-fundraising"
  | "raising-first-fund"
  | "ma-acquisition"
  | "series-a-plus"
  | "real-estate-transaction"
  | "fund-management"
  | "portfolio-management"
  | "project-management"
  | "sales-dataroom"
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

/** Deal-room knowledge base status for Ask Docs corpus. */
export type DealRoomKnowledgeBaseStatus =
  | "none"
  | "building"
  | "ready"
  | "failed"
  | "stale";

export interface DealRoomKnowledgeBase {
  room_id: string;
  status: DealRoomKnowledgeBaseStatus;
  folder_paths: string[];
  document_ids: string[];
  active_document_ids?: string[];
  building_document_ids?: string[];
  active_generation?: number;
  building_generation?: number;
  error_message?: string;
  embedded_count: number;
  folder_count: number;
}

export interface DealRoomKnowledgeBaseSelection {
  folder_paths?: string[];
  document_ids?: string[];
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
  /** Total views across all public/share links for this deal room. */
  viewCount?: number;
  /** Number of currently active share links for this deal room. */
  activeLinkCount?: number;
  /** User-defined tags for filtering and organizing deal rooms. */
  tags?: string[];
  /** Number of unique visitors across all share links for this deal room. */
  visitorCount?: number;
  /** Number of unanswered visitor questions on this deal room's links. */
  unreadQuestions?: number;
  /** Engagement score (0-100) computed from visitor activity. */
  heatScore?: number;
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
  emailEnabled: boolean;
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

// Signal-First dashboard types

export type SignalType = "hot_signal" | "risk_alert" | "follow_up";
export type RiskAlertType =
  | "bounce"
  | "download"
  | "expired"
  | "access_exhausted"
  | "access_revoked"
  | "blocked_attempt"
  | "anomaly"
  | "forward";
export type ActionType = "email" | "call" | "share" | "review" | "approve" | "sign" | "answer" | "renew" | "verify";
export type ActionStatus = "pending" | "done" | "snoozed" | "ignored";
export type Priority = "high" | "medium" | "low";
export type Circle = "founder" | "investor_ir" | "sales";

export interface SignalContext {
  opens: number;
  uniqueVisitors: number;
  durationSeconds: number;
  keyPageCount: number;
  keyPageTitles: string[];
  contactName?: string;
  contactEmail?: string;
  visitorEmail?: string;
  documentTitle?: string;
  question?: string;
  intent?: string;
  actor?: string;
}

export interface Signal {
  id: string;
  type: SignalType;
  subtype?: string;
  title: string;
  description: string;
  explanation: string;
  suggestion: string;
  context?: SignalContext;
  documentId?: string;
  contactId?: string;
  linkId?: string;
  createdAt: string;
  priority: Priority;
}

export interface ActionItem {
  id: string;
  signalId?: string;
  sourceType?:
    | "link_access_request"
    | "room_access_request"
    | "room_nda"
    | "link_question"
    | "uploaded_file"
    | "expiring_link"
    | "expiring_room";
  sourceId?: string;
  title: string;
  impact: Priority;
  dueAt: string;
  status: ActionStatus;
  actionType: ActionType;
  createdAt: string;
  updatedAt: string;
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
  defaultPermissionLevel: PermissionPreset;
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
  type: RiskAlertType;
  priority: Priority;
  title: string;
  description: string;
  metadata?: Record<string, string>;
  linkId?: string;
  documentId?: string;
  createdAt: string;
}
