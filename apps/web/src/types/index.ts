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

/** Library partition: general (文档页) | deal_room (数据室) | agreement (协议). */
export type DocumentCategory = "general" | "agreement" | "deal_room";

export type DocumentStatus = "uploading" | "processing" | "ready" | "failed" | "archived";

export type DocumentFilter = "all" | "shared" | "recent" | "popular" | "unshared" | "archived";

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
  /** Present for document shares; omitted/empty for deal-room shares. */
  documentId?: string;
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
  /** Q&A feature toggle (available from v2.7+ backend). */
  qaEnabled?: boolean;
  /** Visitor Ask routing mode (supervised | ai_first). */
  askMode?: "supervised" | "self_serve" | "formal";
  /** Grounded AI answers for deal-room links (Phase B). */
  askAiEnabled?: boolean;
  /** Unified visitor Ask UI (Phase A; requires VISITOR_ASK_UNIFIED=1 on API). */
  visitorAskUnified?: boolean;
  /** File request feature toggle (available from v2.7+ backend). */
  fileRequestsEnabled?: boolean;
  /** Index file feature toggle (available from v2.7+ backend). */
  indexFileEnabled?: boolean;
  /** Screenshot protection feature toggle (available from v2.7+ backend). */
  screenshotProtectionEnabled?: boolean;
  /** Soft warnings returned on create/update. */
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
  /** Unified Ask turn id when loaded from owner /ask inbox. */
  ask_turn_id?: string;
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

/** Unified visitor Ask turn (Phase A host lane). */
export interface PublicAskTurn {
  id: string;
  session_id: string;
  question: string;
  lane: "ai" | "host" | "hybrid";
  status:
    | "routing"
    | "ai_streaming"
    | "ai_answered"
    | "ai_refused"
    | "host_pending"
    | "host_escalated"
    | "host_answered"
    | "failed";
  host_question_id?: string;
  host_answer?: string;
  route_reason?: string;
  pinned_faq_at?: string;
  pinned_faq_by?: string;
  pinned_faq_sort?: number;
  formal_status?: "pending_review" | "scheduled" | "published";
  formal_publish_at?: string;
  formal_published_at?: string;
  formal_anonymize?: boolean;
  ai_payload?: {
    answer?: string;
    refused: boolean;
    resultStatus: string;
    hits?: Array<{
      chunkId: string;
      documentId?: string;
      text: string;
      score: number;
      sourceName?: string;
      pages?: number[];
      sheet?: string;
      viewerPage?: number;
    }>;
  };
  created_at: string;
  updated_at: string;
}

/** Visitor-visible pinned FAQ on a share link (Phase B). */
export interface PublicAskFAQ {
  id: string;
  question: string;
  answer: string;
  source: "ai" | "host" | "hybrid";
  link_id?: string;
  link_name?: string;
  pinned_faq_sort?: number;
  ai_payload?: PublicAskTurn["ai_payload"];
  pinned_at: string;
}

/** Visitor-visible published formal Q&A (Phase C). */
export interface PublicFormalAsk {
  id: string;
  question: string;
  answer: string;
  published_at: string;
  link_id?: string;
  link_name?: string;
}

/** Owner-facing unified Ask turn (host inbox). */
export interface OwnerAskTurn extends PublicAskTurn {
  link_id: string;
  visitor_id: string;
  visitor_email?: string;
  /** Same normalized question count within link (owner inbox). */
  repeat_count?: number;
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

/** Workspace share-inbox row with link/document context. */
export interface PendingLinkAccessRequest extends LinkAccessRequest {
  link_name?: string;
  document_title?: string;
  short_url?: string;
}

export interface AccessRule {
  ruleType: "email";
  value: string;
  action: "allow" | "block";
}

/**
 * Thin deal-room security policy: room-wide blocklist + optional outbound floors.
 * Allowlists and full protection toggles live on each share link.
 */
export interface DealRoomAccessPolicy {
  dealRoomId: string;
  configured: boolean;
  requireEmailVerificationFloor?: boolean;
  requireNdaFloor?: boolean;
  blockedEmails: string[];
  updatedAt?: string;
  /** @deprecated Prefer requireEmailVerificationFloor; mirrored during rollout. */
  requireEmailVerification?: boolean;
  /** @deprecated Prefer requireNdaFloor; mirrored during rollout. */
  requireNda?: boolean;
  /** Legacy wire fields — always empty/false from thin room security API. */
  requireEmail?: boolean;
  requirePassword?: boolean;
  hasPassword?: boolean;
  ndaTemplateId?: string;
  ndaDocumentId?: string;
  watermarkEnabled?: boolean;
  downloadEnabled?: boolean;
  screenshotProtectionEnabled?: boolean;
  fileRequestsEnabled?: boolean;
  indexFileEnabled?: boolean;
  qaEnabled?: boolean;
  allowedEmails?: string[];
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
  /** Workspace NDA template ID when require NDA is enabled (preferred over document). */
  ndaTemplateId?: string;
  allowDownload: boolean;
  watermarkEnabled: boolean;
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
  ask_summary?: {
    total_turns: number;
    ai_answered: number;
    ai_refused: number;
    host_pending: number;
    host_answered: number;
    user_escalated?: number;
    auto_escalated?: number;
    deflection_rate?: number;
    refuse_rate?: number;
    escalation_rate?: number;
  };
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
  /** Structure lock — blocks rename/delete/upload/create-child. */
  locked?: boolean;
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
  /** Structure lock — blocks remove/move. */
  locked?: boolean;
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

/** Aggregated analytics for a deal room's share links (GET /deal-rooms/:id/analytics). */
export interface DealRoomAnalytics {
  totalViews: number;
  uniqueVisitors: number;
  activeLinkCount: number;
  documentCount: number;
  viewsOverTime: { day: string; views: number }[];
  recentVisitors: {
    visitorId: string;
    visitorEmail?: string;
    firstAccessAt: string;
    lastAccessAt: string;
    totalViews: number;
  }[];
}

/** Plan quota pair for the vector library card. */
export interface KnowledgeQuotaPair {
  used: number;
  limit: number;
}

/** External docling-rag knowledge corpus status for a deal room. */
export interface DealRoomKnowledgeCorpus {
  enabled: boolean;
  status: string;
  lastSyncedAt?: string;
  errorMessage?: string;
  progress?: {
    total: number;
    pending: number;
    syncing: number;
    synced: number;
    failed: number;
    jobStatus?: string;
  };
  documents: Array<{
    documentId: string;
    title?: string;
    status: string;
    chunkCount: number;
    lastError?: string;
  }>;
  /** Entitlement snapshot (best-effort from docling-rag). */
  quota?: {
    planCode?: string;
    knowledgeBases: KnowledgeQuotaPair;
    documents: KnowledgeQuotaPair;
    answers: KnowledgeQuotaPair;
  };
}

export interface DealRoomKnowledgeQueryHit {
  chunkId: string;
  documentId?: string;
  text: string;
  score: number;
  sourceName?: string;
  pages?: number[];
  sheet?: string;
  viewerPage?: number;
}

export interface DealRoomKnowledgeQueryResult {
  query: string;
  mode: string;
  answer?: string;
  results: DealRoomKnowledgeQueryHit[];
}

/** Auditable desk state machine (ceiling Phase E) — rewrite may only read this + prior turn. */
export interface DealRoomKnowledgeSessionState {
  entities?: Array<{
    name: string;
    type: string;
    firstTurnId: string;
    hitIds?: string[];
  }>;
  openQuestions?: Array<{ text: string; sourceTurnId: string }>;
  coverageHints?: Array<{ sourceNames: string[]; turnId: string }>;
}

/** Persisted research session for the knowledge tab audit timeline. */
export interface DealRoomKnowledgeQASession {
  id: string;
  roomId: string;
  title?: string;
  status: "active" | "closed" | string;
  createdAt: string;
  updatedAt: string;
  lastTurnAt?: string;
  turnCount?: number;
  questionPreview?: string;
  state?: DealRoomKnowledgeSessionState;
}

export interface DealRoomKnowledgeSessionList {
  items: DealRoomKnowledgeQASession[];
  nextCursor?: string;
}

export type DealRoomKnowledgeFeedbackKind =
  | "helpful"
  | "wrong_citation"
  | "not_answering";

export interface DealRoomKnowledgeTurnFeedback {
  kind: DealRoomKnowledgeFeedbackKind;
  note?: string;
}

/** Sentence↔hit binding (ceiling Phase F). Empty hitIds → no fact styling. */
export interface DealRoomKnowledgeAnswerClaim {
  text: string;
  hitIds?: string[];
  confidence?: "grounded" | "weak" | string;
}

/** Cross-file disagreement in the coverage set (ceiling Phase I). */
export interface DealRoomKnowledgeConflictSide {
  sourceName: string;
  hitId?: string;
  value?: string;
  excerpt: string;
}

export interface DealRoomKnowledgeHitConflict {
  id: string;
  kind: "numeric" | string;
  topic?: string;
  sides: DealRoomKnowledgeConflictSide[];
}

/** Audited second-hop retrieve (ceiling Phase I3). */
export interface DealRoomKnowledgeMultiHopQuery {
  kind: "definition" | "attachment" | string;
  query: string;
  fromHitIds?: string[];
  anchor?: string;
}

export interface DealRoomKnowledgeMultiHop {
  applied: boolean;
  queries?: DealRoomKnowledgeMultiHopQuery[];
  addedHitIds?: string[];
}

/** Typed L2 refusal / retrieval gap (ceiling Phase J). */
export interface DealRoomKnowledgeRefusal {
  kind: "ungrounded" | "no_hits" | "error" | string;
  /** True when ungrounded cleared a non-empty hit set (audit). */
  hadHits?: boolean;
  hitCount?: number;
}

/** Stamp quality for answered turns (ceiling Phase K). */
export interface DealRoomKnowledgeJudgment {
  kind: "grounded" | "partial" | string;
  reason?: "weak_only" | "has_unresolved" | "mixed" | string;
  groundedClaims?: number;
  weakClaims?: number;
  unresolvedCount?: number;
}

export interface DealRoomKnowledgeQATurn {
  id: string;
  sessionId: string;
  sequence: number;
  question: string;
  answer?: string;
  refused: boolean;
  resultStatus: "answered" | "refused" | "no_hits" | "error" | string;
  hits: DealRoomKnowledgeQueryHit[];
  mode?: string;
  topK?: number;
  errorSummary?: string;
  /** Standalone retrieve query when rewrite ran; display question stays in `question`. */
  retrieveQuery?: string;
  rewriteApplied?: boolean;
  /** When rewriteApplied: `state` (used session.state) | `prior_only`. */
  rewriteBasis?: "state" | "prior_only" | string;
  /** Provenanced sentences; prefer over raw answer styling when present. */
  claims?: DealRoomKnowledgeAnswerClaim[];
  /** Factual sentences that could not be bound to a hit. */
  unresolved?: string[];
  /** Cross-file conflicts — list both sides, do not pick (ceiling Phase I). */
  conflicts?: DealRoomKnowledgeHitConflict[];
  /** Deterministic clause→definition→attachment hop audit (ceiling Phase I3). */
  multiHop?: DealRoomKnowledgeMultiHop;
  /** Typed refusal / gap (ceiling Phase J). Present for refused / no_hits / error. */
  refusal?: DealRoomKnowledgeRefusal;
  /** Stamp quality for answered turns (ceiling Phase K). */
  judgment?: DealRoomKnowledgeJudgment;
  /** Room RAG sync-generation fingerprint at ask time (ceiling Phase H). */
  corpusFingerprint?: string;
  /** End-to-end ask latency in milliseconds. */
  durationMs?: number;
  createdAt: string;
  /** Current user's feedback (Phase C); omitted when unset. */
  feedback?: DealRoomKnowledgeTurnFeedback;
}

/** Diligence audit export / cold-archive pack (ceiling Phase H). */
export interface DealRoomKnowledgeDiligencePack {
  schemaVersion: string;
  exportedAt: string;
  workspaceId: string;
  roomId: string;
  sessionId: string;
  corpusFingerprint?: string;
  session: DealRoomKnowledgeQASession;
  turns: DealRoomKnowledgeQATurn[];
}

export interface DealRoomKnowledgeSessionArchive {
  id: string;
  workspaceId: string;
  roomId: string;
  sessionId: string;
  title?: string;
  turnCount: number;
  corpusFingerprint?: string;
  status: "cold" | "restored_readonly" | string;
  archivedAt: string;
}

export interface DealRoomKnowledgeSessionArchiveList {
  items: DealRoomKnowledgeSessionArchive[];
}

export interface DealRoomKnowledgeSessionArchiveDetail {
  archive: DealRoomKnowledgeSessionArchive;
  pack: DealRoomKnowledgeDiligencePack;
}

/** Workspace SLO / cost board for the knowledge desk. */
export interface DealRoomKnowledgeOpsSummary {
  scope: string;
  windowHours: number;
  turnsTotal: number;
  turnsByStatus: Record<string, number>;
  avgDurationMs: number;
  /** p95 ask latency in the window (ceiling Phase M). */
  p95DurationMs?: number;
  /** Deterministic evidence+answer volume proxy (1 ≈ 1k runes). */
  costUnitsTotal?: number;
  refusalsByKind?: Record<string, number>;
  judgmentsByKind?: Record<string, number>;
  /** Gold-review queue by status (ceiling Phase O). */
  evalCandidatesByStatus?: Record<string, number>;
  pendingEvalCandidates?: number;
  answersQuota: { used: number; limit: number; windowHours: number };
  retentionDays: number;
  coldArchiveCount: number;
  roomCorpusFingerprint?: string;
  prometheusHints?: string[];
}

/** Scrubbed hit snapshot for gold review (ceiling Phase O/Q). */
export interface DealRoomKnowledgeEvalHitSnapshot {
  chunkId?: string;
  sourceName?: string;
  pages?: number[];
  sheet?: string;
  excerpt?: string;
}

export interface DealRoomKnowledgeEvalCandidateSnapshot {
  hits?: DealRoomKnowledgeEvalHitSnapshot[];
  claims?: Array<{ text: string; hitIds?: string[]; confidence?: string }>;
  unresolved?: string[];
  expectedSourceNames?: string[];
}

/** Feedback→gold review candidate (ceiling Phase O). */
export interface DealRoomKnowledgeEvalCandidate {
  id: string;
  roomId: string;
  turnId: string;
  feedbackKind: "wrong_citation" | "not_answering" | string;
  question: string;
  answer?: string;
  note?: string;
  corpusFingerprint?: string;
  reviewStatus: "pending" | "accepted" | "rejected" | string;
  expect?: string;
  snapshot?: DealRoomKnowledgeEvalCandidateSnapshot | null;
  createdAt: string;
  reviewedAt?: string;
}

export interface DealRoomKnowledgeEvalSeedExport {
  description: string;
  seeds: Array<{
    id: string;
    kind: string;
    question: string;
    answer?: string;
    note?: string;
    expect: string;
  }>;
}

export interface DealRoomKnowledgeSessionDetail {
  session: DealRoomKnowledgeQASession | null;
  turns: DealRoomKnowledgeQATurn[];
}

export interface DealRoomKnowledgeSessionQueryResult {
  sessionId: string;
  turn: DealRoomKnowledgeQATurn;
  query: string;
  mode: string;
  answer?: string;
  results: DealRoomKnowledgeQueryHit[];
  /** Post-turn auditable desk state (ceiling Phase L). */
  sessionState?: DealRoomKnowledgeSessionState;
}

/** Evidence-grounded (or template) next-question chips for a turn. */
export interface DealRoomKnowledgeFollowUpSuggestion {
  id: string;
  text: string;
}

export type DealRoomKnowledgeFollowUpSource = "llm" | "mission" | "template";

export interface DealRoomKnowledgeFollowUpsResult {
  items: DealRoomKnowledgeFollowUpSuggestion[];
  source: DealRoomKnowledgeFollowUpSource | string;
}

/** Builtin diligence mission pack bound to a room (ceiling Phase G). */
export interface DealRoomKnowledgeMissionPack {
  packId: string;
  title: string;
  source: "room" | "template_default" | "catalog" | string;
  items?: Array<{ id: string; prompt: string }>;
}

/** Mission checklist coverage vs session state (ceiling Phase N). */
export interface DealRoomKnowledgeMissionProgressItem {
  id: string;
  prompt: string;
  covered: boolean;
}

export interface DealRoomKnowledgeMissionProgress {
  packId: string;
  title: string;
  source: "room" | "template_default" | string;
  covered: number;
  total: number;
  items: DealRoomKnowledgeMissionProgressItem[];
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
  metadata?: Record<string, string>;
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
    | "deal_room_link_access_request"
    | "room_access_request"
    | "room_nda"
    | "link_question"
    | "deal_room_link_question"
    | "uploaded_file"
    | "expiring_link"
    | "expiring_room";
  /** Operational identity for upsert/resolve (link id or room id). */
  sourceId?: string;
  /** Navigation parent when sourceId alone is not enough (deal room id for room-share links). */
  targetId?: string;
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
