import { buildDraft, buildRules, type DraftLink } from "@/components/links/share";
import type { AccessRule, DealRoomAccessPolicy } from "@/types";
import type { UpsertDealRoomAccessPolicyPayload } from "@/lib/api";

export type RoomSecurityFloors = {
  requireEmailVerification: boolean;
  requireNda: boolean;
};

function truthyFlag(...values: unknown[]): boolean {
  for (const value of values) {
    if (value === true || value === 1 || value === "true" || value === "1") return true;
  }
  return false;
}

/** Resolve outbound floors from thin or legacy policy wire shapes. */
export function roomSecurityFloors(
  policy: DealRoomAccessPolicy | null | undefined,
): RoomSecurityFloors {
  if (!policy) {
    return { requireEmailVerification: false, requireNda: false };
  }
  // Defensive: accept camelCase (API), snake_case, and nested { data } envelopes.
  const raw = (
    "dealRoomId" in policy && policy.dealRoomId
      ? policy
      : ((policy as { data?: DealRoomAccessPolicy }).data ?? policy)
  ) as DealRoomAccessPolicy & {
    require_email_verification_floor?: boolean;
    require_nda_floor?: boolean;
    require_email_verification?: boolean;
    require_nda?: boolean;
    RequireEmailVerificationFloor?: boolean;
    RequireNdaFloor?: boolean;
    RequireEmailVerification?: boolean;
    RequireNda?: boolean;
  };
  return {
    requireEmailVerification: truthyFlag(
      raw.requireEmailVerificationFloor,
      raw.require_email_verification_floor,
      raw.RequireEmailVerificationFloor,
      raw.requireEmailVerification,
      raw.require_email_verification,
      raw.RequireEmailVerification,
    ),
    requireNda: truthyFlag(
      raw.requireNdaFloor,
      raw.require_nda_floor,
      raw.RequireNdaFloor,
      raw.requireNda,
      raw.require_nda,
      raw.RequireNda,
    ),
  };
}

/**
 * Hard-clamp a share-link draft to room floors.
 * Callers must use this before validate/save so UI state cannot bypass policy.
 */
export function clampDraftToRoomSecurityFloors(
  draft: DraftLink,
  policy: DealRoomAccessPolicy | null | undefined,
): DraftLink {
  const floors = roomSecurityFloors(policy);
  if (!floors.requireEmailVerification && !floors.requireNda) {
    return draft;
  }
  return {
    ...draft,
    requireEmailVerification: floors.requireEmailVerification
      ? true
      : draft.requireEmailVerification,
    requireEmail: floors.requireEmailVerification ? false : draft.requireEmail,
    requireNda: floors.requireNda ? true : draft.requireNda,
  };
}

/** Normalized room-wide blocklist emails from policy. */
export function roomBlockedEmails(
  policy: DealRoomAccessPolicy | null | undefined,
): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of policy?.blockedEmails ?? []) {
    const v = raw.trim().toLowerCase();
    if (!v || seen.has(v)) continue;
    seen.add(v);
    out.push(v);
  }
  return out;
}

/**
 * Enforce room blocklist on a share-link draft: room blocks stay blocked,
 * cannot appear in allow list, and link-only blocks are preserved.
 */
export function clampDraftToRoomBlocklist(
  draft: DraftLink,
  policy: DealRoomAccessPolicy | null | undefined,
): DraftLink {
  const roomBlocked = roomBlockedEmails(policy);
  if (roomBlocked.length === 0) return draft;
  const roomSet = new Set(roomBlocked);
  const allowedViewers = draft.allowedViewers.filter(
    (v) => !roomSet.has(v.trim().toLowerCase()),
  );
  const linkOnlyBlocked = draft.blockedViewers.filter(
    (v) => !roomSet.has(v.trim().toLowerCase()),
  );
  return {
    ...draft,
    allowedViewers,
    blockedViewers: [...roomBlocked, ...linkOnlyBlocked],
  };
}

/** Apply room security floors then mandatory room blocklist. */
export function clampDraftToRoomPolicy(
  draft: DraftLink,
  policy: DealRoomAccessPolicy | null | undefined,
): DraftLink {
  return clampDraftToRoomBlocklist(clampDraftToRoomSecurityFloors(draft, policy), policy);
}

/** Link-scoped blocked viewers (excludes room-wide blocks). */
export function linkOnlyBlockedViewers(
  draft: DraftLink,
  policy: DealRoomAccessPolicy | null | undefined,
): string[] {
  const roomSet = new Set(roomBlockedEmails(policy));
  return draft.blockedViewers.filter((v) => !roomSet.has(v.trim().toLowerCase()));
}

/** Build access rules persisted on the link — room blocks are runtime-only. */
export function buildLinkScopedRules(
  draft: DraftLink,
  policy: DealRoomAccessPolicy | null | undefined,
): AccessRule[] {
  return buildRules({
    ...draft,
    blockedViewers: linkOnlyBlockedViewers(draft, policy),
  });
}

/**
 * Hydrate Room Security editor state from policy.
 * Allowlist and per-link protections are intentionally absent.
 */
export function draftFromRoomAccessPolicy(policy: DealRoomAccessPolicy | null | undefined): DraftLink {
  const base = buildDraft(null, []);
  if (!policy) return { ...base, allowedViewers: [], blockedViewers: [] };

  const floors = roomSecurityFloors(policy);
  return {
    ...base,
    requireEmail: false,
    requireEmailVerification: floors.requireEmailVerification,
    requirePassword: false,
    password: "",
    watermarkEnabled: false,
    requireNda: floors.requireNda,
    ndaDocumentId: "",
    ndaTemplateId: "",
    allowDownloading: false,
    enableScreenshotProtection: false,
    enableFileRequests: false,
    enableIndexFileGeneration: false,
    allowedViewers: [],
    blockedViewers: [...(policy.blockedEmails ?? [])],
    folderScopeMode: "full",
    folderPaths: [],
  };
}

/**
 * Create-link draft: start from standard preset, apply room blocklist + floors.
 * Never inherit allowlist or room-wide protection toggles.
 */
export function hydrateCreateDraftFromRoomPolicy(
  policy: DealRoomAccessPolicy | null | undefined,
): DraftLink {
  const draft = buildDraft(null, []);
  return clampDraftToRoomPolicy(
    {
      ...draft,
      blockedViewers: [...(policy?.blockedEmails ?? [])],
      allowedViewers: [],
      folderScopeMode: "full",
      folderPaths: [],
    },
    policy,
  );
}

/** Edit-link draft: load link rules, then force room floors + blocklist on. */
export function hydrateEditDraftFromRoomPolicy(
  link: Parameters<typeof buildDraft>[0],
  rules: Parameters<typeof buildDraft>[1],
  policy: DealRoomAccessPolicy | null | undefined,
): DraftLink {
  return clampDraftToRoomPolicy(buildDraft(link, rules), policy);
}

/** Payload for PUT /deal-rooms/:id/access-policy (thin room security). */
export function roomAccessPolicyPayloadFromDraft(
  draft: DraftLink,
): UpsertDealRoomAccessPolicyPayload {
  return {
    require_email_verification_floor: draft.requireEmailVerification,
    require_nda_floor: draft.requireNda,
    // Dual-write legacy keys so older API builds still persist the same columns.
    require_email_verification: draft.requireEmailVerification,
    require_nda: draft.requireNda,
    blocked_emails: draft.blockedViewers.map((v) => v.trim().toLowerCase()).filter(Boolean),
  };
}
