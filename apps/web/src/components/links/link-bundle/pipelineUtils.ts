import { PRESET_TEMPLATES } from "../smart-link/levelConfig";
import type { Document, DocumentSummary, PermissionConfig, PermissionPreset } from "@/types";

import { LIBRARY_DOCUMENT_CATEGORY } from "@/lib/documentCategory";

/** Share-content picker uses the same partition as the document library. */
export const SHARE_CONTENT_DOCUMENT_CATEGORY = LIBRARY_DOCUMENT_CATEGORY;

/**
 * Edit-mode lists: keep the available picker clean (no agreement / data-room merge-back),
 * while still reconstructing already-selected link documents for the selected tray.
 */
export function buildEditModeDocumentLists(
  pickerDocs: Document[],
  linkDocuments: DocumentSummary[],
): { pickerDocuments: Document[]; selectedDocuments: Document[] } {
  const byId = new Map(pickerDocs.map((d) => [d.id, d]));
  const selectedDocuments: Document[] = linkDocuments.map((ds) => {
    const full = byId.get(ds.id);
    if (full) return full;
    return {
      id: ds.id,
      title: ds.title,
      sourceType: ds.sourceType,
      fileName: ds.title,
      fileType: ds.sourceType,
      fileSize: ds.fileSize ?? 0,
      pageCount: ds.pageCount,
      status: ds.status,
      createdAt: new Date(0).toISOString(),
      updatedAt: new Date(0).toISOString(),
    };
  });
  // Never enlarge the available pool with selected orphans (e.g. agreement / room docs).
  return { pickerDocuments: pickerDocs, selectedDocuments };
}

/** Preset expiry options shown in the Security step (plus Custom). */
export const LINK_EXPIRY_PRESET_DAYS = [7, 15, 30] as const;
export type LinkExpiryPresetDays = (typeof LINK_EXPIRY_PRESET_DAYS)[number];

/** Preset max-views options shown in the Security step (plus Unlimited / Custom). */
export const LINK_MAX_VIEWS_PRESETS = [10, 50, 100] as const;
export type LinkMaxViewsPreset = (typeof LINK_MAX_VIEWS_PRESETS)[number];

/** Inclusive upper bound for custom max views (fits int32 `max_access_count`). */
export const LINK_CUSTOM_MAX_VIEWS_MAX = 1_000_000;

/** Default value when switching Max views to Custom from Unlimited. */
export const LINK_CUSTOM_MAX_VIEWS_DEFAULT = 25;

export function isLinkMaxViewsPreset(
  value: PermissionConfig["maxViews"] | number,
): value is LinkMaxViewsPreset {
  return (LINK_MAX_VIEWS_PRESETS as readonly number[]).includes(Number(value));
}

/**
 * Map a stored expiresAt onto the Security expiry select.
 * Presets snap within ±1 day (ceil + time-of-day); everything else is Custom
 * so the datetime picker can show the exact timestamp.
 */
export function resolveExpiryDaysFromExpiresAt(
  expiresAt: string | undefined | null,
  now: Date = new Date(),
): { expiryDays: number | "custom"; _editExpiresAt?: string } {
  if (!expiresAt) {
    return { expiryDays: 30 };
  }
  const expires = new Date(expiresAt);
  if (Number.isNaN(expires.getTime())) {
    return { expiryDays: 30 };
  }
  const diffMs = expires.getTime() - now.getTime();
  const diffDays = Math.ceil(diffMs / (1000 * 60 * 60 * 24));
  if (diffDays <= 0) {
    return { expiryDays: "custom", _editExpiresAt: expiresAt };
  }
  for (const preset of LINK_EXPIRY_PRESET_DAYS) {
    if (Math.abs(diffDays - preset) <= 1) {
      return { expiryDays: preset, _editExpiresAt: expiresAt };
    }
  }
  return { expiryDays: "custom", _editExpiresAt: expiresAt };
}

/**
 * Map a stored maxAccessCount onto the Security max-views select.
 * 10 / 50 / 100 snap to presets; any other positive count is Custom.
 */
export function resolveMaxViewsFromAccessCount(
  maxAccessCount: number | undefined | null,
): { maxViews: number | "unlimited" | "custom"; _editMaxViews?: number } {
  if (typeof maxAccessCount !== "number" || maxAccessCount <= 0) {
    return { maxViews: "unlimited" };
  }
  if (isLinkMaxViewsPreset(maxAccessCount)) {
    return { maxViews: maxAccessCount };
  }
  return { maxViews: "custom", _editMaxViews: maxAccessCount };
}

export type BundleSecurityGuardReason =
  | "contactRequired"
  | "ndaDocumentRequired"
  | "customExpiresAtRequired"
  | "customExpiresAtFuture"
  | "customMaxViewsRequired"
  | "customMaxViewsInvalid";

export function bundleSecurityGuardI18nKey(
  reason: BundleSecurityGuardReason,
): string {
  switch (reason) {
    case "ndaDocumentRequired":
      return "creator.ndaDocumentRequired";
    case "customExpiresAtRequired":
      return "creator.customExpiresAtRequired";
    case "customExpiresAtFuture":
      return "creator.customExpiresAtFuture";
    case "customMaxViewsRequired":
      return "creator.customMaxViewsRequired";
    case "customMaxViewsInvalid":
      return "creator.customMaxViewsInvalid";
    case "contactRequired":
    default:
      return "creator.contactRequired";
  }
}

/** Client-side guards before create/update — mirrors StepReview checks. */
export function validateBundleSecurityConfig(
  config: PermissionConfig,
): { ok: true } | { ok: false; reason: BundleSecurityGuardReason } {
  const emailRequired =
    config.requireEmailVerification || config.ndaEnabled;
  if (emailRequired && config.contactIds.length === 0) {
    return { ok: false, reason: "contactRequired" };
  }
  if (config.ndaEnabled && !config.ndaTemplateId && !config.ndaDocumentId) {
    return { ok: false, reason: "ndaDocumentRequired" };
  }
  if (config.expiryDays === "custom") {
    if (!config._editExpiresAt) {
      return { ok: false, reason: "customExpiresAtRequired" };
    }
    const ts = new Date(config._editExpiresAt).getTime();
    if (Number.isNaN(ts) || ts <= Date.now()) {
      return { ok: false, reason: "customExpiresAtFuture" };
    }
  }
  if (config.maxViews === "custom") {
    const n = config._editMaxViews;
    if (typeof n !== "number" || !Number.isFinite(n)) {
      return { ok: false, reason: "customMaxViewsRequired" };
    }
    if (!Number.isInteger(n) || n < 1 || n > LINK_CUSTOM_MAX_VIEWS_MAX) {
      return { ok: false, reason: "customMaxViewsInvalid" };
    }
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
    fileRequestsEnabled: template.fileRequestsEnabled ?? false,
    indexFileEnabled: template.indexFileEnabled ?? false,
    expiryDays: template.expiryDays,
    maxViews: template.maxViews,
    contactIds: [],
    ...safeOverrides,
  };
}
