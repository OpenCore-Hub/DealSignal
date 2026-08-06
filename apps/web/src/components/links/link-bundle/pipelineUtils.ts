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
