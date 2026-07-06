import type { IntegrationStatus, PermissionConfig } from "@/types";

export interface CreateLinkPayload {
  document_ids: string[];
  name?: string;
  permission_type?: string;
  require_email_verification?: boolean;
  require_password?: boolean;
  require_nda?: boolean;
  allowed_emails?: string[];
  allowed_domains?: string[];
  password?: string;
  contact_ids?: string[];
  expires_at?: string;
  max_access_count?: number;
  download_enabled?: boolean;
  watermark_enabled?: boolean;
  ai_copilot_enabled?: boolean;
}

export type UpdateLinkPayload = CreateLinkPayload;

// Note: document sort_order is implicit — the backend stores the array index
// position (i=0,1,2…) of each document_id in the `document_ids` field as the
// `link_documents.sort_order`. Frontend array order IS the display order.

function mapConfigToPermissionType(
  config: PermissionConfig,
): string {
  // Derive the closest legacy permission_type for backward compatibility.
  // Priority must match backend normalizeSecurityConfig: password > nda > whitelist > public.
  if (config.passwordEnabled) return "password";
  if (config.ndaEnabled) return "nda";
  if (config.whitelistEnabled && config.whitelist.length > 0) return "whitelist";
  // Modern email verification is controlled by the independent boolean flag,
  // not by the legacy permission_type, so it should remain "public".
  return "public";
}

export function toCreateLinkPayload(
  documentIds: string[],
  config: PermissionConfig,
  name?: string,
): CreateLinkPayload {
  const whitelist = (config.whitelistEnabled ? config.whitelist : [])
    .map((s) => s.trim())
    .filter(Boolean);
  const allowedEmails = whitelist.filter(
    (s) => s.includes("@") && !s.startsWith("@"),
  );
  const allowedDomains = whitelist.filter(
    (s) => !s.includes("@") || s.startsWith("@"),
  );

  // Derived: email verification is required when any of the source flags is on.
  const requireEmailVerification =
    config.requireEmailVerification ||
    config.whitelistEnabled ||
    config.ndaEnabled;

  const payload: CreateLinkPayload = {
    document_ids: documentIds,
    name,
    permission_type: mapConfigToPermissionType(config),
    require_email_verification: requireEmailVerification,
    require_password: config.passwordEnabled,
    require_nda: config.ndaEnabled,
    allowed_emails:
      allowedEmails.length > 0 ? allowedEmails : undefined,
    allowed_domains:
      allowedDomains.length > 0 ? allowedDomains : undefined,
    password: config.passwordEnabled ? config.password : undefined,
    contact_ids:
      requireEmailVerification && config.contactIds.length > 0
        ? config.contactIds
        : undefined,
    download_enabled: config.allowDownload,
    watermark_enabled: config.watermarkEnabled,
    ai_copilot_enabled: config.aiCopilotEnabled,
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
    template_type: input.template
      ? input.template.replace(/-/g, "_")
      : undefined,
    requires_nda: input.ndaEnabled,
    requires_approval: input.requiresApproval,
  };
}

// Backend integration settings shape (snake_case, *_connected flags).
export interface BackendIntegrationStatus {
  workspace_id?: string;
  email_enabled?: boolean;
  slack_webhook_url?: string;
  slack_connected?: boolean;
  hubspot_connected?: boolean;
  salesforce_connected?: boolean;
  updated_at?: string;
}

export function toIntegrationStatus(
  backend: BackendIntegrationStatus,
): IntegrationStatus {
  return {
    emailEnabled: backend.email_enabled ?? true,
    slack: backend.slack_connected ?? false,
    hubspot: backend.hubspot_connected ?? false,
    // Zapier is not yet supported by the backend; keep it as a UI placeholder.
    zapier: false,
  };
}

export function toBackendIntegrationStatus(
  status: IntegrationStatus,
): BackendIntegrationStatus {
  return {
    email_enabled: status.emailEnabled,
    slack_connected: status.slack,
    hubspot_connected: status.hubspot,
    // Zapier state is local-only until backend support is added.
  };
}
