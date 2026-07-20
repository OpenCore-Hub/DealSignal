import type { AccessRule, Link } from "@/types";
import type { UpdateLinkPayload } from "@/lib/apiAdapters";
import { PRESETS, isPresetMatch } from "./presets";
import type { DraftLink, LinkPreset } from "./types";

export function getPublicUrl(link: Link | null): string {
  if (!link) return "";
  const token = link.shortUrl.split("/").pop() ?? "";
  if (!token) return "";
  if (link.customDomain) {
    return `${window.location.protocol}//${link.customDomain.replace(/\/$/, "")}/l/${token}`;
  }
  return `${window.location.origin}/l/${token}`;
}

export function toDateTimeLocal(iso?: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  d.setMinutes(d.getMinutes() - d.getTimezoneOffset());
  return d.toISOString().slice(0, 16);
}

export function toRFC3339(localValue: string): string {
  if (!localValue) return "";
  const [datePart, timePart] = localValue.split("T");
  if (!datePart || !timePart) return "";
  const [year, month, day] = datePart.split("-").map((s) => parseInt(s, 10));
  const [hour, minute] = timePart.split(":").map((s) => parseInt(s, 10));
  const d = new Date(year, month - 1, day, hour, minute);
  if (Number.isNaN(d.getTime())) return "";
  const offset = d.getTimezoneOffset();
  const sign = offset <= 0 ? "+" : "-";
  const abs = Math.abs(offset);
  const hh = String(Math.floor(abs / 60)).padStart(2, "0");
  const mm = String(abs % 60).padStart(2, "0");
  return `${localValue}:00${sign}${hh}:${mm}`;
}

export function buildDraft(link?: Link | null, rules?: AccessRule[]): DraftLink {
  const allowedViewers: string[] = [];
  const blockedViewers: string[] = [];
  for (const rule of rules ?? []) {
    // Defensive: some deployed API builds return PascalCase field names for
    // AccessRule because they predate the lowercase json tags. Accept both.
    const value = (rule as { value?: string; Value?: string }).value ?? (rule as { Value?: string }).Value;
    const action = (rule as { action?: string; Action?: string }).action ?? (rule as { Action?: string }).Action;
    if (!value || typeof value !== "string") continue;
    if (action === "allow") allowedViewers.push(value);
    else blockedViewers.push(value);
  }

  if (link) {
    return {
      name: link.name ?? "",
      expiresAt: toDateTimeLocal(link.expiresAt),
      requireEmail: link.requireEmail ?? false,
      requireEmailVerification: link.requireEmailVerification ?? false,
      requirePassword: link.requirePassword ?? false,
      password: "",
      watermarkEnabled: link.watermarkEnabled ?? false,
      requireNda: link.requireNda ?? false,
      ndaDocumentId: link.ndaDocumentId ?? "",
      allowDownloading: link.downloadEnabled ?? false,
      enableScreenshotProtection: link.screenshotProtectionEnabled ?? false,
      aiCopilotEnabled: link.aiCopilotEnabled ?? false,
      enableFileRequests: link.fileRequestsEnabled ?? false,
      enableIndexFileGeneration: link.indexFileEnabled ?? false,
      enableQaConversations: link.qaEnabled ?? false,
      allowedViewers,
      blockedViewers,
      customDomain: link.customDomain ?? "",
      notifyOnAccess: link.notifyOnAccess ?? false,
      folderPaths: link.folderPaths ?? [],
      contactIds: link.contactIds ?? [],
    };
  }

  return {
    name: "",
    ...PRESETS.standard,
    password: "",
    allowedViewers,
    blockedViewers,
    customDomain: "",
    notifyOnAccess: false,
    folderPaths: [],
    contactIds: [],
  };
}

export function inferPreset(draft: DraftLink): LinkPreset {
  for (const name of ["public", "standard", "confidential"] as const) {
    if (isPresetMatch(name, draft)) return name;
  }
  return "custom";
}

export function isValidCustomDomain(domain: string): boolean {
  return /^(?!-)[A-Za-z0-9-]{1,63}(?<!-)(\.(?!-)[A-Za-z0-9-]{1,63}(?<!-))*\.?[A-Za-z]{2,}$/.test(domain);
}

export function toAccessRule(value: string, action: "allow" | "block"): AccessRule {
  const safeValue = typeof value === "string" ? value.trim() : "";
  return { ruleType: "email", value: safeValue, action };
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === "string" && value.length > 0;
}

export function buildRules(draft: DraftLink): AccessRule[] {
  return [
    ...draft.allowedViewers.filter(isNonEmptyString).map((v) => toAccessRule(v, "allow")),
    ...draft.blockedViewers.filter(isNonEmptyString).map((v) => toAccessRule(v, "block")),
  ];
}

export function buildAllowedLists(draft: DraftLink): {
  allowedEmails: string[];
  blockedEmails: string[];
} {
  return {
    allowedEmails: draft.allowedViewers.filter(isNonEmptyString),
    blockedEmails: draft.blockedViewers.filter(isNonEmptyString),
  };
}

export function buildLinkPayload(draft: DraftLink, existingLink?: Link | null): UpdateLinkPayload {
  const permissionType = draft.requireNda
    ? "nda"
    : draft.requireEmailVerification || draft.requireEmail
      ? "email_required"
      : "public";
  return {
    document_ids: existingLink?.documentIds ?? [],
    folder_paths: draft.folderPaths,
    name: draft.name.trim(),
    permission_type: permissionType,
    require_email: draft.requireEmail,
    require_email_verification: draft.requireEmailVerification,
    require_password: draft.requirePassword,
    require_nda: draft.requireNda,
    nda_document_id: draft.requireNda ? draft.ndaDocumentId : undefined,
    password: draft.requirePassword && draft.password ? draft.password : undefined,
    expires_at: toRFC3339(draft.expiresAt) || undefined,
    download_enabled: draft.allowDownloading,
    watermark_enabled: draft.watermarkEnabled,
    ai_copilot_enabled: draft.aiCopilotEnabled,
    qa_enabled: draft.enableQaConversations,
    file_requests_enabled: draft.enableFileRequests,
    index_file_enabled: draft.enableIndexFileGeneration,
    screenshot_protection_enabled: draft.enableScreenshotProtection,
    custom_domain: draft.customDomain || undefined,
    notify_on_access: draft.notifyOnAccess,
    contact_ids:
      draft.requireEmailVerification && !existingLink?.dealRoomId && draft.contactIds.length > 0
        ? draft.contactIds
        : undefined,
  };
}

export function validateDraft(
  draft: DraftLink,
  selectedLink: Link | null,
  t: (key: string, options?: Record<string, unknown>) => string,
  now: number,
  isDealRoomLink?: boolean,
  existingNames?: string[]
): Record<string, string> {
  const errors: Record<string, string> = {};
  const trimmedName = draft.name.trim();
  if (!trimmedName) {
    errors.name = t("share.linkNameRequired");
  } else if (
    existingNames?.some((name) => name.trim().toLowerCase() === trimmedName.toLowerCase())
  ) {
    errors.name = t("share.linkNameDuplicate");
  }
  const expiresAtRFC = toRFC3339(draft.expiresAt);
  if (expiresAtRFC && new Date(expiresAtRFC).getTime() <= now) {
    errors.expiresAt = t("share.expiresAtFuture");
  }
  if (draft.requirePassword && !selectedLink?.requirePassword && draft.password.length < 8) {
    errors.password = t("accessRules.errors.passwordMinLength");
  }
  if (
    draft.customDomain &&
    !isValidCustomDomain(draft.customDomain)
  ) {
    errors.customDomain = t("share.customDomainInvalid");
  }
  if (draft.allowedViewers.length > 0 && !draft.requireEmail && !draft.requireEmailVerification) {
    errors.allowedViewers = t("accessRules.errors.allowRequiresEmail");
  }
  if ((draft.requireEmail || draft.requireEmailVerification) && draft.allowedViewers.length === 0) {
    errors.allowedViewers = t("accessRules.errors.allowedViewersRequired");
  }
  const dealRoomLink = isDealRoomLink ?? !!selectedLink?.dealRoomId;
  if (draft.requireEmailVerification && !dealRoomLink && draft.contactIds.length === 0) {
    errors.requireVerificationContacts = t("accessRules.errors.requireVerificationContacts");
  }
  const conflict = draft.allowedViewers.find((v) => draft.blockedViewers.includes(v));
  if (conflict) {
    errors.conflict = t("accessRules.errors.conflict", { value: conflict });
  }
  if (draft.requireNda && !draft.ndaDocumentId) {
    errors.ndaDocumentId = t("accessRules.errors.ndaDocumentRequired");
  }
  return errors;
}
