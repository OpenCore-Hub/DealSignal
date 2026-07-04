import type { PermissionConfig } from "@/types";

export interface CreateLinkPayload {
  document_id: string;
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
}

function mapConfigToPermissionType(
  config: PermissionConfig,
): string {
  // Derive the closest legacy permission_type for backward compatibility.
  if (config.ndaEnabled) return "nda";
  if (config.passwordEnabled) return "password";
  if (config.whitelistEnabled && config.whitelist.length > 0) return "whitelist";
  // Modern email verification is controlled by the independent boolean flag,
  // not by the legacy permission_type, so it should remain "public".
  return "public";
}

export function toCreateLinkPayload(
  documentId: string,
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

  const payload: CreateLinkPayload = {
    document_id: documentId,
    name,
    permission_type: mapConfigToPermissionType(config),
    require_email_verification:
      config.requireEmailVerification ||
      config.whitelistEnabled ||
      config.ndaEnabled,
    require_password: config.passwordEnabled,
    require_nda: config.ndaEnabled,
    allowed_emails:
      allowedEmails.length > 0 ? allowedEmails : undefined,
    allowed_domains:
      allowedDomains.length > 0 ? allowedDomains : undefined,
    password: config.passwordEnabled ? config.password : undefined,
    contact_ids:
      config.requireEmailVerification && config.contactId
        ? [config.contactId]
        : undefined,
    download_enabled: config.allowDownload,
    watermark_enabled: config.watermarkEnabled,
  };

  if (typeof config.expiryDays === "number") {
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
