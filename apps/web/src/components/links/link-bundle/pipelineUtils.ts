import { PRESET_TEMPLATES } from "../smart-link/levelConfig";
import type { PermissionConfig, PermissionPreset } from "@/types";

export function buildConfigFromPreset(
  preset: PermissionPreset,
  overrides?: Partial<PermissionConfig>,
): PermissionConfig {
  const template = PRESET_TEMPLATES[preset];
  // Strip level/isCustomized from overrides to prevent callers from accidentally
  // reverting a preset's identity (e.g., passing { level: "public" } would break
  // the preset classifier).
  const { level: _, isCustomized: __, ...safeOverrides } = overrides ?? {};
  return {
    level: preset,
    isCustomized: preset === "customized",
    requireEmailVerification: template.requireEmailVerification,
    whitelistEnabled: template.whitelistEnabled,
    whitelist: template.whitelist,
    passwordEnabled: template.passwordEnabled,
    ndaEnabled: template.ndaEnabled,
    allowDownload: template.allowDownload,
    watermarkEnabled: template.watermarkEnabled,
    aiCopilotEnabled: template.aiCopilotEnabled,
    expiryDays: template.expiryDays,
    maxViews: template.maxViews,
    contactIds: [],
    ...safeOverrides,
  };
}
