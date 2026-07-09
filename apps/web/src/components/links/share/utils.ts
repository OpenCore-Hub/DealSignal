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

export function buildDraft(link?: Link | null, rules?: AccessRule[]): DraftLink {
  const allowedViewers: string[] = [];
  const blockedViewers: string[] = [];
  for (const rule of rules ?? []) {
    const value = rule.ruleType === "domain" ? `*@${rule.value}` : rule.value;
    if (rule.action === "allow") allowedViewers.push(value);
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
      allowDownloading: link.downloadEnabled ?? false,
      enableScreenshotProtection: false,
      aiCopilotEnabled: link.aiCopilotEnabled ?? false,
      enableFileRequests: false,
      enableIndexFileGeneration: false,
      enableQaConversations: false,
      allowedViewers,
      blockedViewers,
      autoAddInvited: true,
      customDomain: link.customDomain ?? "",
      tags: link.tags ?? [],
      notifyOnAccess: link.notifyOnAccess ?? false,
    };
  }

  return {
    name: "",
    ...PRESETS.standard,
    password: "",
    allowedViewers,
    blockedViewers,
    autoAddInvited: true,
    customDomain: "",
    tags: [],
    notifyOnAccess: false,
  };
}

export function inferPreset(draft: DraftLink): LinkPreset {
  for (const name of ["public", "standard", "confidential"] as const) {
    if (isPresetMatch(name, draft)) return name;
  }
  return "custom";
}

export function toAccessRule(value: string, action: "allow" | "block"): AccessRule {
  if (value.startsWith("*@")) {
    return { ruleType: "domain", value: value.slice(2), action };
  }
  return { ruleType: "email", value, action };
}

export function buildRules(draft: DraftLink): AccessRule[] {
  return [
    ...draft.allowedViewers.map((v) => toAccessRule(v, "allow")),
    ...draft.blockedViewers.map((v) => toAccessRule(v, "block")),
  ];
}

export function buildLinkPayload(draft: DraftLink, existingLink?: Link | null): UpdateLinkPayload {
  return {
    document_ids: existingLink?.documentIds ?? [],
    name: draft.name.trim(),
    permission_type: draft.requireNda ? "nda" : "public",
    require_email: draft.requireEmail,
    require_email_verification: draft.requireEmailVerification,
    require_password: draft.requirePassword,
    require_nda: draft.requireNda,
    password: draft.requirePassword && draft.password ? draft.password : undefined,
    expires_at: draft.expiresAt || undefined,
    download_enabled: draft.allowDownloading,
    watermark_enabled: draft.watermarkEnabled,
    ai_copilot_enabled: draft.aiCopilotEnabled,
    custom_domain: draft.customDomain || undefined,
    tags: draft.tags.length > 0 ? draft.tags : [],
    notify_on_access: draft.notifyOnAccess,
  };
}

export function validateDraft(
  draft: DraftLink,
  selectedLink: Link | null,
  t: (key: string, options?: Record<string, unknown>) => string,
  now: number
): Record<string, string> {
  const errors: Record<string, string> = {};
  if (!draft.name.trim()) {
    errors.name = t("share.linkNameRequired");
  }
  if (draft.expiresAt && new Date(draft.expiresAt).getTime() <= now) {
    errors.expiresAt = t("share.expiresAtFuture");
  }
  if (draft.requirePassword && !selectedLink?.requirePassword && draft.password.length < 8) {
    errors.password = t("accessRules.errors.passwordMinLength");
  }
  if (
    draft.customDomain &&
    !/^(?!-)[A-Za-z0-9-]{1,63}(?<!-)(\.(?!-)[A-Za-z0-9-]{1,63}(?<!-))*\.?[A-Za-z]{2,}$/.test(
      draft.customDomain,
    )
  ) {
    errors.customDomain = t("share.customDomainInvalid");
  }
  if (draft.requireEmailVerification && !draft.requireEmail) {
    errors.requireEmail = t("accessRules.errors.verificationRequiresEmail");
  }
  if (draft.allowedViewers.length > 0 && !draft.requireEmail) {
    errors.allowedViewers = t("accessRules.errors.allowRequiresEmail");
  }
  const conflict = draft.allowedViewers.find((v) => draft.blockedViewers.includes(v));
  if (conflict) {
    errors.conflict = t("accessRules.errors.conflict", { value: conflict });
  }
  return errors;
}
