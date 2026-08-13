import type {
  BillingInfo,
  BillingPlanOffer,
  BillingPlansResponse,
  IntegrationStatus,
  OutboundWebhookConfig,
  PermissionConfig,
  WorkspaceMember,
  WorkspaceSettings,
  WorkspaceViewerDomain,
} from "@/types";

export interface CreateLinkPayload {
  document_ids?: string[];
  folder_paths?: string[];
  folder_scope_mode?: "full" | "allowlist";
  name?: string;
  permission_type?: string;
  require_email?: boolean;
  require_email_verification?: boolean;
  require_password?: boolean;
  require_nda?: boolean;
  nda_document_id?: string;
  nda_template_id?: string;
  allowed_emails?: string[];
  password?: string;
  contact_ids?: string[];
  expires_at?: string;
  max_access_count?: number;
  download_enabled?: boolean;
  watermark_enabled?: boolean;
  qa_enabled?: boolean;
  ask_ai_enabled?: boolean;
  ask_mode?: "supervised" | "self_serve" | "formal";
  file_requests_enabled?: boolean;
  index_file_enabled?: boolean;
  screenshot_protection_enabled?: boolean;
  link_type?: "share" | "file_request";
  target_folder_path?: string;
  custom_domain?: string;
  notify_on_access?: boolean;
}

export type UpdateLinkPayload = CreateLinkPayload;

/** POST /deal-rooms/:roomId/links */
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
  ask_ai_enabled?: boolean;
  ask_mode?: "supervised" | "self_serve" | "formal";
  file_requests_enabled?: boolean;
  index_file_enabled?: boolean;
  screenshot_protection_enabled?: boolean;
  custom_domain?: string;
  tags?: string[];
  notify_on_access?: boolean;
  folder_paths?: string[];
  folder_scope_mode?: "full" | "allowlist";
}

// Note: document sort_order is implicit — the backend stores the array index
// position (i=0,1,2…) of each document_id in the `document_ids` field as the
// `link_documents.sort_order`. Frontend array order IS the display order.

function mapConfigToPermissionType(
  config: PermissionConfig,
): string {
  // Derive the closest legacy permission_type for backward compatibility.
  // Priority must match backend normalizeSecurityConfig: nda > public.
  // Whitelist and password have been removed from the UI, so they are never
  // emitted even if an old local draft still contains them.
  if (config.ndaEnabled) return "nda";
  // Modern email verification is controlled by the independent boolean flag,
  // not by the legacy permission_type, so it should remain "public".
  return "public";
}

export function toCreateLinkPayload(
  documentIds: string[],
  config: PermissionConfig,
  name?: string,
): CreateLinkPayload {
  // Derived: email verification is required when any of the source flags is on.
  const requireEmailVerification =
    config.requireEmailVerification ||
    config.ndaEnabled;

  const payload: CreateLinkPayload = {
    document_ids: documentIds,
    name,
    permission_type: mapConfigToPermissionType(config),
    require_email_verification: requireEmailVerification,
    require_password: false,
    require_nda: config.ndaEnabled,
    // Match deal-room share: require an explicit NDA template/document — never
    // fall back to the shared content documentIds[0].
    nda_template_id:
      config.ndaEnabled && config.ndaTemplateId
        ? config.ndaTemplateId
        : undefined,
    nda_document_id:
      config.ndaEnabled && config.ndaDocumentId
        ? config.ndaDocumentId
        : undefined,
    allowed_emails: undefined,
    password: undefined,
    contact_ids:
      requireEmailVerification && config.contactIds.length > 0
        ? config.contactIds
        : undefined,
    download_enabled: config.allowDownload,
    watermark_enabled: config.watermarkEnabled,
    screenshot_protection_enabled: config.screenshotProtectionEnabled,
  };

  // Prefer exact expiresAt from edit-mode reconstruction to avoid round-trip
  // drift (config.expiryDays → Date setDate → expiryDays each round-trip
  // shifts by ±1 day due to time component truncation).
  // If expiryDays was changed by the user, _editExpiresAt is expected to have
  // been cleared by the reducer. However, as a defense-in-depth measure, compute
  // the expiry from expiryDays and only use _editExpiresAt when the two agree
  // within 25 hours (covering timezone edge cases).
  if (config._editExpiresAt && typeof config.expiryDays === "number") {
    const fromExpiryDays = new Date();
    fromExpiryDays.setDate(fromExpiryDays.getDate() + config.expiryDays);
    const diffMs = Math.abs(new Date(config._editExpiresAt).getTime() - fromExpiryDays.getTime());
    // Allow ±25 hours to cover timezone + clock drift; beyond that, user
    // intentionally changed expiry — discard the stale _editExpiresAt.
    if (diffMs < 25 * 60 * 60 * 1000) {
      payload.expires_at = config._editExpiresAt;
    } else {
      payload.expires_at = fromExpiryDays.toISOString();
    }
  } else if (config._editExpiresAt) {
    payload.expires_at = config._editExpiresAt;
  } else if (typeof config.expiryDays === "number") {
    const expiresAt = new Date();
    expiresAt.setDate(expiresAt.getDate() + config.expiryDays);
    payload.expires_at = expiresAt.toISOString();
  }

  if (typeof config.maxViews === "number") {
    payload.max_access_count = config.maxViews;
  }

  return payload;
}

export interface CreateDealRoomPayload {
  name: string;
  slug: string;
  description?: string;
  template_type?: string;
  requires_nda?: boolean;
  requires_approval?: boolean;
}

export function toCreateDealRoomPayload(
  input: {
    name: string;
    slug: string;
    description?: string;
    template?: string;
    ndaEnabled?: boolean;
    requiresApproval?: boolean;
  },
): CreateDealRoomPayload {
  return {
    name: input.name,
    slug: input.slug,
    description: input.description,
    template_type: input.template,
    requires_nda: input.ndaEnabled,
    requires_approval: input.requiresApproval,
  };
}

// Backend integration settings shape (snake_case, *_connected flags).
export interface BackendIntegrationStatus {
  workspace_id?: string;
  email_enabled?: boolean;
  daily_digest_enabled?: boolean;
  key_page_slack_enabled?: boolean;
  slack_webhook_url?: string;
  slack_connected?: boolean;
  hubspot_connected?: boolean;
  salesforce_connected?: boolean;
  can_manage?: boolean;
  updated_at?: string;
}

export function toIntegrationStatus(
  backend: BackendIntegrationStatus,
): IntegrationStatus {
  return {
    emailEnabled: backend.email_enabled ?? true,
    dailyDigestEnabled: backend.daily_digest_enabled ?? false,
    keyPageSlackEnabled: backend.key_page_slack_enabled ?? false,
    slack: backend.slack_connected ?? false,
    hubspot: backend.hubspot_connected ?? false,
    canManage: backend.can_manage ?? false,
  };
}

export function toBackendIntegrationStatus(
  status: IntegrationStatus,
): BackendIntegrationStatus {
  return {
    email_enabled: status.emailEnabled,
    daily_digest_enabled: status.dailyDigestEnabled,
    key_page_slack_enabled: status.keyPageSlackEnabled,
    slack_connected: status.slack,
    hubspot_connected: status.hubspot,
  };
}

export type BackendOutboundWebhook = {
  configured: boolean;
  enabled: boolean;
  url?: string;
  event_types?: string[];
  secret_hint?: string;
  secret?: string;
  updated_at?: string;
};

/** GET/PUT /settings — backend uses snake_case; UI uses WorkspaceSettings. */
export type BackendWorkspaceSettings = {
  name?: string;
  slug?: string;
  brand_color?: string;
  brandColor?: string;
  viewer_domain?: string;
  viewerDomain?: string;
  logo_url?: string;
  logoUrl?: string;
  data?: BackendWorkspaceSettings;
};

export function toWorkspaceSettings(
  backend: BackendWorkspaceSettings | null | undefined,
): WorkspaceSettings {
  const src = backend?.data ?? backend ?? {};
  return {
    name: src.name ?? "",
    slug: src.slug ?? "",
    brandColor: src.brand_color ?? src.brandColor ?? "",
    viewerDomain: src.viewer_domain ?? src.viewerDomain ?? "",
    logoUrl: src.logo_url ?? src.logoUrl ?? "",
  };
}

export function toUpdateWorkspaceSettingsPayload(settings: WorkspaceSettings): {
  name: string;
  slug: string;
  brand_color: string;
  logo_url?: string;
} {
  return {
    name: settings.name,
    slug: settings.slug,
    brand_color: settings.brandColor,
    logo_url: settings.logoUrl || undefined,
  };
}

/** GET/PUT /viewer-domain — backend uses snake_case. */
export type BackendWorkspaceViewerDomain = {
  hostname?: string;
  status?: string;
  cname_host?: string;
  cnameHost?: string;
  cname_target?: string;
  cnameTarget?: string;
  verified_at?: string;
  verifiedAt?: string;
};

export function toWorkspaceViewerDomain(
  backend: BackendWorkspaceViewerDomain | null | undefined,
): WorkspaceViewerDomain {
  const src = backend ?? {};
  const status = src.status === "pending" || src.status === "verified" ? src.status : "";
  return {
    hostname: src.hostname ?? "",
    status,
    cnameHost: src.cname_host ?? src.cnameHost ?? "",
    cnameTarget: src.cname_target ?? src.cnameTarget ?? "",
    verifiedAt: src.verified_at ?? src.verifiedAt,
  };
}

export function toOutboundWebhookConfig(backend: BackendOutboundWebhook): OutboundWebhookConfig {
  return {
    configured: backend.configured ?? false,
    enabled: backend.enabled ?? false,
    url: backend.url,
    eventTypes: backend.event_types,
    secretHint: backend.secret_hint,
    secret: backend.secret,
    updatedAt: backend.updated_at,
  };
}

export type BackendWorkspaceMember = {
  id?: string;
  user_id?: string;
  userId?: string;
  email?: string;
  name?: string;
  role?: string;
  joined_at?: string;
  joinedAt?: string;
  status?: string;
  avatar_url?: string;
  avatarUrl?: string;
};

const WORKSPACE_MEMBER_ROLES = new Set(["owner", "admin", "member", "guest"]);
const WORKSPACE_MEMBER_STATUSES = new Set(["active", "pending", "suspended"]);

function asWorkspaceMemberRole(value: string | undefined): WorkspaceMember["role"] {
  if (value && WORKSPACE_MEMBER_ROLES.has(value)) {
    return value as WorkspaceMember["role"];
  }
  return "member";
}

function asWorkspaceMemberStatus(value: string | undefined): WorkspaceMember["status"] {
  if (value && WORKSPACE_MEMBER_STATUSES.has(value)) {
    return value as WorkspaceMember["status"];
  }
  return "active";
}

/** GET /workspaces/:slug/members — snake_case API → FE WorkspaceMember. */
export function toWorkspaceMember(backend: BackendWorkspaceMember | null | undefined): WorkspaceMember {
  const src = backend ?? {};
  const email = src.email ?? "";
  const name = (src.name ?? "").trim();
  return {
    id: src.id ?? src.user_id ?? src.userId ?? email,
    userId: src.user_id ?? src.userId ?? "",
    email,
    name: name || email,
    role: asWorkspaceMemberRole(src.role),
    joinedAt: src.joined_at ?? src.joinedAt ?? "",
    status: asWorkspaceMemberStatus(src.status),
    avatarUrl: src.avatar_url ?? src.avatarUrl,
  };
}

export function toWorkspaceMembers(
  backend: BackendWorkspaceMember[] | { data?: BackendWorkspaceMember[] } | null | undefined,
): WorkspaceMember[] {
  const rows = Array.isArray(backend) ? backend : (backend?.data ?? []);
  return rows.map((row) => toWorkspaceMember(row));
}

/** GET /billing — backend uses snake_case bytes/counts; UI uses BillingInfo. */
export type BackendBillingInfo = {
  plan?: string;
  period?: string;
  trial_expired?: boolean;
  trialExpired?: boolean;
  trial_ends_at?: string;
  trialEndsAt?: string;
  storage_used?: number;
  storageUsed?: number;
  storage_limit?: number;
  storageLimit?: number;
  links_used?: number;
  linksUsed?: number;
  links_limit?: number;
  linksLimit?: number;
  rooms_used?: number;
  roomsUsed?: number;
  rooms_limit?: number;
  roomsLimit?: number;
  seats_used?: number;
  seatsUsed?: number;
  seats_limit?: number;
  seatsLimit?: number;
  documents_used?: number;
  documentsUsed?: number;
  documents_limit?: number;
  documentsLimit?: number;
  ask_ai_used?: number;
  askAiUsed?: number;
  ask_ai_limit?: number;
  askAiLimit?: number;
  knowledge_answers_used?: number;
  knowledgeAnswersUsed?: number;
  knowledge_answers_limit?: number;
  knowledgeAnswersLimit?: number;
  max_upload_bytes?: number;
  maxUploadBytes?: number;
  custom_domain_enabled?: boolean;
  customDomainEnabled?: boolean;
  watermark_enabled?: boolean;
  watermarkEnabled?: boolean;
  nda_enabled?: boolean;
  ndaEnabled?: boolean;
  visitor_ask_ai_enabled?: boolean;
  visitorAskAiEnabled?: boolean;
  branding_enabled?: boolean;
  brandingEnabled?: boolean;
  access_controls_enabled?: boolean;
  accessControlsEnabled?: boolean;
  knowledge_desk_enabled?: boolean;
  knowledgeDeskEnabled?: boolean;
  webhooks_enabled?: boolean;
  webhooksEnabled?: boolean;
  hubspot_enabled?: boolean;
  hubspotEnabled?: boolean;
  daily_digest_enabled?: boolean;
  dailyDigestEnabled?: boolean;
  slack_alerts_enabled?: boolean;
  slackAlertsEnabled?: boolean;
  room_analytics_enabled?: boolean;
  roomAnalyticsEnabled?: boolean;
  room_insights_enabled?: boolean;
  roomInsightsEnabled?: boolean;
  formal_ask_enabled?: boolean;
  formalAskEnabled?: boolean;
  billing_status?: string;
  billingStatus?: string;
  has_stripe_subscription?: boolean;
  hasStripeSubscription?: boolean;
  current_period_end?: string;
  currentPeriodEnd?: string;
  data?: BackendBillingInfo;
};

function asFiniteNumber(value: unknown, fallback = 0): number {
  const n = typeof value === "number" ? value : Number(value);
  return Number.isFinite(n) ? n : fallback;
}

export function toBillingInfo(backend: BackendBillingInfo | null | undefined): BillingInfo {
  const src = backend?.data ?? backend ?? {};
  const trialEndsAtRaw = src.trial_ends_at ?? src.trialEndsAt;
  const trialEndsAt =
    typeof trialEndsAtRaw === "string" && trialEndsAtRaw.trim() ? trialEndsAtRaw.trim() : undefined;
  return {
    plan: (src.plan ?? "free").trim() || "free",
    period: (src.period ?? "monthly").trim() || "monthly",
    trialExpired: Boolean(src.trial_expired ?? src.trialExpired),
    ...(trialEndsAt ? { trialEndsAt } : {}),
    storageUsed: asFiniteNumber(src.storage_used ?? src.storageUsed),
    storageLimit: asFiniteNumber(src.storage_limit ?? src.storageLimit),
    linksUsed: asFiniteNumber(src.links_used ?? src.linksUsed),
    linksLimit: asFiniteNumber(src.links_limit ?? src.linksLimit),
    roomsUsed: asFiniteNumber(src.rooms_used ?? src.roomsUsed),
    roomsLimit: asFiniteNumber(src.rooms_limit ?? src.roomsLimit),
    seatsUsed: asFiniteNumber(src.seats_used ?? src.seatsUsed),
    seatsLimit: asFiniteNumber(src.seats_limit ?? src.seatsLimit),
    documentsUsed: asFiniteNumber(src.documents_used ?? src.documentsUsed),
    documentsLimit: asFiniteNumber(src.documents_limit ?? src.documentsLimit),
    askAiUsed: asFiniteNumber(src.ask_ai_used ?? src.askAiUsed),
    askAiLimit: asFiniteNumber(src.ask_ai_limit ?? src.askAiLimit),
    knowledgeAnswersUsed: asFiniteNumber(src.knowledge_answers_used ?? src.knowledgeAnswersUsed),
    knowledgeAnswersLimit: asFiniteNumber(src.knowledge_answers_limit ?? src.knowledgeAnswersLimit),
    maxUploadBytes: asFiniteNumber(src.max_upload_bytes ?? src.maxUploadBytes),
    customDomainEnabled: Boolean(src.custom_domain_enabled ?? src.customDomainEnabled),
    watermarkEnabled: Boolean(src.watermark_enabled ?? src.watermarkEnabled),
    ndaEnabled: Boolean(src.nda_enabled ?? src.ndaEnabled),
    visitorAskAiEnabled: Boolean(src.visitor_ask_ai_enabled ?? src.visitorAskAiEnabled),
    brandingEnabled: Boolean(src.branding_enabled ?? src.brandingEnabled),
    accessControlsEnabled: Boolean(src.access_controls_enabled ?? src.accessControlsEnabled),
    knowledgeDeskEnabled: Boolean(src.knowledge_desk_enabled ?? src.knowledgeDeskEnabled),
    webhooksEnabled: Boolean(src.webhooks_enabled ?? src.webhooksEnabled),
    hubspotEnabled: Boolean(src.hubspot_enabled ?? src.hubspotEnabled),
    dailyDigestEnabled: Boolean(src.daily_digest_enabled ?? src.dailyDigestEnabled),
    slackAlertsEnabled: Boolean(src.slack_alerts_enabled ?? src.slackAlertsEnabled),
    roomAnalyticsEnabled: Boolean(src.room_analytics_enabled ?? src.roomAnalyticsEnabled),
    roomInsightsEnabled: Boolean(src.room_insights_enabled ?? src.roomInsightsEnabled),
    formalAskEnabled: Boolean(src.formal_ask_enabled ?? src.formalAskEnabled),
    hasStripeSubscription: Boolean(src.has_stripe_subscription ?? src.hasStripeSubscription),
    ...(typeof (src.billing_status ?? src.billingStatus) === "string" &&
    String(src.billing_status ?? src.billingStatus).trim()
      ? { billingStatus: String(src.billing_status ?? src.billingStatus).trim() }
      : {}),
    ...(typeof (src.current_period_end ?? src.currentPeriodEnd) === "string" &&
    String(src.current_period_end ?? src.currentPeriodEnd).trim()
      ? { currentPeriodEnd: String(src.current_period_end ?? src.currentPeriodEnd).trim() }
      : {}),
  };
}

export type BackendBillingPlanOffer = {
  code?: string;
  internal_seats?: number;
  internalSeats?: number;
  storage_bytes?: number;
  storageBytes?: number;
  documents?: number;
  links?: number;
  rooms?: number;
  max_upload_bytes?: number;
  maxUploadBytes?: number;
  visitor_ask_ai_monthly?: number;
  visitorAskAiMonthly?: number;
  custom_domain?: boolean;
  customDomain?: boolean;
  watermark?: boolean;
  nda?: boolean;
  visitor_ask_ai?: boolean;
  visitorAskAi?: boolean;
  branding?: boolean;
  access_controls?: boolean;
  accessControls?: boolean;
  formal_ask?: boolean;
  formalAsk?: boolean;
  price_monthly_usd?: number;
  priceMonthlyUsd?: number;
  custom_pricing?: boolean;
  customPricing?: boolean;
  highlighted?: boolean;
};

export type BackendBillingPlansResponse = {
  current_plan?: string;
  currentPlan?: string;
  current_period?: string;
  currentPeriod?: string;
  trial_expired?: boolean;
  trialExpired?: boolean;
  trial_ends_at?: string;
  trialEndsAt?: string;
  billing_status?: string;
  billingStatus?: string;
  has_stripe_subscription?: boolean;
  hasStripeSubscription?: boolean;
  plans?: BackendBillingPlanOffer[];
  data?: BackendBillingPlansResponse;
};

export function toBillingPlanOffer(raw: BackendBillingPlanOffer | null | undefined): BillingPlanOffer {
  const src = raw ?? {};
  return {
    code: (src.code ?? "free").trim().toLowerCase() || "free",
    internalSeats: asFiniteNumber(src.internal_seats ?? src.internalSeats),
    storageBytes: asFiniteNumber(src.storage_bytes ?? src.storageBytes),
    documents: asFiniteNumber(src.documents),
    links: asFiniteNumber(src.links),
    rooms: asFiniteNumber(src.rooms),
    maxUploadBytes: asFiniteNumber(src.max_upload_bytes ?? src.maxUploadBytes),
    visitorAskAiMonthly: asFiniteNumber(src.visitor_ask_ai_monthly ?? src.visitorAskAiMonthly),
    customDomain: Boolean(src.custom_domain ?? src.customDomain),
    watermark: Boolean(src.watermark),
    nda: Boolean(src.nda),
    visitorAskAi: Boolean(src.visitor_ask_ai ?? src.visitorAskAi),
    branding: Boolean(src.branding),
    accessControls: Boolean(src.access_controls ?? src.accessControls),
    formalAsk: Boolean(src.formal_ask ?? src.formalAsk),
    priceMonthlyUsd: asFiniteNumber(src.price_monthly_usd ?? src.priceMonthlyUsd),
    customPricing: Boolean(src.custom_pricing ?? src.customPricing),
    highlighted: Boolean(src.highlighted),
  };
}

export function toBillingPlansResponse(
  backend: BackendBillingPlansResponse | null | undefined,
): BillingPlansResponse {
  const src = backend?.data ?? backend ?? {};
  const trialEndsAtRaw = src.trial_ends_at ?? src.trialEndsAt;
  const trialEndsAt =
    typeof trialEndsAtRaw === "string" && trialEndsAtRaw.trim() ? trialEndsAtRaw.trim() : undefined;
  const plans = Array.isArray(src.plans) ? src.plans.map((p) => toBillingPlanOffer(p)) : [];
  return {
    currentPlan: (src.current_plan ?? src.currentPlan ?? "free").trim().toLowerCase() || "free",
    currentPeriod:
      (src.current_period ?? src.currentPeriod ?? "monthly").trim().toLowerCase() || "monthly",
    trialExpired: Boolean(src.trial_expired ?? src.trialExpired),
    ...(trialEndsAt ? { trialEndsAt } : {}),
    billingStatus:
      typeof (src.billing_status ?? src.billingStatus) === "string"
        ? String(src.billing_status ?? src.billingStatus).trim() || undefined
        : undefined,
    hasStripeSubscription: Boolean(src.has_stripe_subscription ?? src.hasStripeSubscription),
    plans,
  };
}
