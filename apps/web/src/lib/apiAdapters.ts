import type { PermissionConfig } from "@/types";

export interface CreateLinkPayload {
  document_id: string;
  name?: string;
  permission_type?: string;
  allowed_emails?: string[];
  allowed_domains?: string[];
  password?: string;
  expires_at?: string;
  max_access_count?: number;
  download_enabled?: boolean;
  watermark_enabled?: boolean;
}

function mapPermissionLevel(config: PermissionConfig): string {
  switch (config.level) {
    case "low":
      return "public";
    case "medium":
      return "email_required";
    case "high":
      if (config.whitelistEnabled && config.whitelist.length > 0) {
        return "whitelist";
      }
      if (config.passwordEnabled && config.password) {
        return "password";
      }
      return "whitelist";
    default:
      return "public";
  }
}

export function toCreateLinkPayload(
  documentId: string,
  config: PermissionConfig,
  name?: string
): CreateLinkPayload {
  const whitelist = (config.whitelistEnabled ? config.whitelist : [])
    .map((s) => s.trim())
    .filter(Boolean);
  const allowedEmails = whitelist.filter((s) => s.includes("@") && !s.startsWith("@"));
  const allowedDomains = whitelist.filter((s) => !s.includes("@") || s.startsWith("@"));

  const payload: CreateLinkPayload = {
    document_id: documentId,
    name,
    permission_type: mapPermissionLevel(config),
    allowed_emails: allowedEmails.length > 0 ? allowedEmails : undefined,
    allowed_domains: allowedDomains.length > 0 ? allowedDomains : undefined,
    password: config.passwordEnabled ? config.password : undefined,
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
  }
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
