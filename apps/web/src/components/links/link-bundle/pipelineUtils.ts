import { PRESET_TEMPLATES } from "../smart-link/levelConfig";
import type { PermissionConfig, PermissionPreset } from "@/types";

/** Client-side guards before create/update — mirrors StepReview checks. */
export function validateBundleSecurityConfig(
  config: PermissionConfig,
): { ok: true } | { ok: false; reason: "contactRequired" | "ndaDocumentRequired" } {
  const emailRequired =
    config.requireEmailVerification || config.ndaEnabled;
  if (emailRequired && config.contactIds.length === 0) {
    return { ok: false, reason: "contactRequired" };
  }
  if (config.ndaEnabled && !config.ndaTemplateId && !config.ndaDocumentId) {
    return { ok: false, reason: "ndaDocumentRequired" };
  }
  return { ok: true };
}

export function buildConfigFromPreset(
  preset: PermissionPreset,
  overrides?: Partial<PermissionConfig>,
): PermissionConfig {
  const template = PRESET_TEMPLATES[preset];
  // Strip level/isCustomized from overrides to prevent callers from accidentally
  // reverting a preset's identity (e.g., passing { level: "public" } would break
  // the preset classifier).
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const { level, isCustomized, ...safeOverrides } = overrides ?? {};
  return {
    level: preset,
    isCustomized: preset === "customized",
    requireEmailVerification: template.requireEmailVerification,
    whitelistEnabled: template.whitelistEnabled,
    whitelist: template.whitelist,
    passwordEnabled: template.passwordEnabled,
    ndaEnabled: template.ndaEnabled,
    ndaDocumentId: "",
    ndaTemplateId: "",
    allowDownload: template.allowDownload,
    watermarkEnabled: template.watermarkEnabled,
    qaEnabled: template.qaEnabled ?? false,
    fileRequestsEnabled: template.fileRequestsEnabled ?? false,
    indexFileEnabled: template.indexFileEnabled ?? false,
    expiryDays: template.expiryDays,
    maxViews: template.maxViews,
    contactIds: [],
    ...safeOverrides,
  };
}
