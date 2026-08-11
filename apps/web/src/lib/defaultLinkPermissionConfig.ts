import type { PermissionConfig } from "@/types";

/**
 * Canonical create defaults for document / bundle share links.
 * Keep library Share dialog and `/links/new` bundle pipeline in sync.
 */
export function createDefaultLinkPermissionConfig(): PermissionConfig {
  return {
    level: "customized",
    isCustomized: true,
    requireEmailVerification: false,
    whitelistEnabled: false,
    whitelist: [],
    passwordEnabled: false,
    ndaEnabled: false,
    ndaDocumentId: "",
    ndaTemplateId: "",
    allowDownload: true,
    watermarkEnabled: true,
    fileRequestsEnabled: false,
    indexFileEnabled: false,
    expiryDays: 30,
    maxViews: "unlimited",
    contactIds: [],
  };
}
