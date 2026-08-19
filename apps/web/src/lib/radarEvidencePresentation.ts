import { isAccessGatePromptReason } from "@/lib/accessEventLabels";
import { diligenceRemediationPath } from "@/lib/actionNavigation";
import type { RadarProduct } from "@/lib/radarQueue";

/** Gate-hold cards: waiting-to-enter, including allowlist/block holds. */
export function isRadarGateHoldItem(item: {
  product: RadarProduct;
  verb?: string;
}): boolean {
  return item.product === "diligence_gate" && item.verb === "review";
}

export function looksLikeEmail(value: string): boolean {
  const s = value.trim();
  return s.includes("@") && !/\s/.test(s);
}

/**
 * Card identities for a radar row.
 * Waiting-to-enter: a non-email actor is the share contact (legacy rows).
 * An email actor is the person held; a distinct contactEmail is who the
 * link was sent to.
 */
export function radarRowIdentities(item: {
  product: RadarProduct;
  actor?: string;
  contactEmail?: string;
}): { primary: string | null; shareContact: string | null } {
  const actor = item.actor?.trim() ?? "";
  const contactEmail = item.contactEmail?.trim() ?? "";
  if (!actor) return { primary: null, shareContact: null };
  if (item.product !== "diligence_gate") {
    return { primary: actor, shareContact: null };
  }
  if (!looksLikeEmail(actor)) {
    return { primary: null, shareContact: actor };
  }
  const shareContact =
    looksLikeEmail(contactEmail) &&
    contactEmail.toLowerCase() !== actor.toLowerCase()
      ? contactEmail
      : null;
  return { primary: actor, shareContact };
}

export type EvidenceSecurityEvent = {
  eventType: string;
  reason?: string;
  email?: string;
  createdAt: string;
};

/**
 * i18n key for product-primary empty evidence (never lead with four zero tiles).
 * Returns null when the product should keep honest zero metrics (none today).
 */
export function evidenceEmptyPrimaryKey(
  product: RadarProduct,
  opts: {
    metricsActive: boolean;
    hasSecurityEvents: boolean;
  },
): string | null {
  if (opts.metricsActive) return null;
  // Diligence keeps the gate-specific empty line (access-request block is separate).
  if (product === "diligence_gate") {
    return "radar.evidenceRail.gateNoSuccessfulOpens";
  }
  // Security events are the primary facet for risk products — skip empty metrics copy.
  if (
    opts.hasSecurityEvents &&
    (product === "leak_watch" ||
      product === "abuse_guard" ||
      product === "access_decay")
  ) {
    return null;
  }
  switch (product) {
    case "leak_watch":
      return "radar.evidenceRail.emptyPrimary.leak_watch";
    case "abuse_guard":
      return "radar.evidenceRail.emptyPrimary.abuse_guard";
    case "access_decay":
      return "radar.evidenceRail.emptyPrimary.access_decay";
    case "buying_window":
      return "radar.evidenceRail.emptyPrimary.buying_window";
    case "commitment_ask":
      return "radar.evidenceRail.emptyPrimary.commitment_ask";
    default:
      return null;
  }
}

export type CoalescedSecurityEvent = {
  key: string;
  eventType: string;
  reason?: string;
  email?: string;
  count: number;
  firstAt: string;
  lastAt: string;
  occurrences: EvidenceSecurityEvent[];
};

/** Group identical gate/security rows (type + reason) for a compact evidence rail. */
export function coalesceSecurityEvents(
  events: EvidenceSecurityEvent[] | undefined | null,
): CoalescedSecurityEvent[] {
  if (!events?.length) return [];
  const sorted = [...events].sort(
    (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime(),
  );
  const groups = new Map<string, CoalescedSecurityEvent>();
  for (const e of sorted) {
    const key = `${e.eventType}\0${e.reason ?? ""}\0${e.email ?? ""}`;
    const existing = groups.get(key);
    if (!existing) {
      groups.set(key, {
        key,
        eventType: e.eventType,
        reason: e.reason,
        email: e.email,
        count: 1,
        firstAt: e.createdAt,
        lastAt: e.createdAt,
        occurrences: [e],
      });
      continue;
    }
    existing.count += 1;
    existing.occurrences.push(e);
    if (new Date(e.createdAt).getTime() < new Date(existing.firstAt).getTime()) {
      existing.firstAt = e.createdAt;
    }
    if (new Date(e.createdAt).getTime() > new Date(existing.lastAt).getTime()) {
      existing.lastAt = e.createdAt;
    }
  }
  // Newest activity first.
  return [...groups.values()].sort(
    (a, b) => new Date(b.lastAt).getTime() - new Date(a.lastAt).getTime(),
  );
}

export type GateTimelineSummary =
  | { kind: "before_and_after"; before: number; after: number; total: number }
  | { kind: "before_only"; before: number; total: number }
  | { kind: "after_only"; after: number; total: number }
  | { kind: "events_only"; total: number };

/**
 * Decision narrative relative to the access-request timestamp.
 * Empty-form prompts are not holds and must not inflate "still blocked".
 */
export function gateTimelineSummary(
  events: EvidenceSecurityEvent[] | undefined | null,
  requestedAt?: string | null,
): GateTimelineSummary | null {
  const holds = (events ?? []).filter((e) => !isAccessGatePromptReason(e.reason));
  if (!holds.length) return null;
  const total = holds.length;
  if (!requestedAt) {
    return { kind: "events_only", total };
  }
  const pivot = new Date(requestedAt).getTime();
  if (Number.isNaN(pivot)) {
    return { kind: "events_only", total };
  }
  let before = 0;
  let after = 0;
  for (const e of holds) {
    const ts = new Date(e.createdAt).getTime();
    if (Number.isNaN(ts) || ts >= pivot) after += 1;
    else before += 1;
  }
  if (before > 0 && after > 0) {
    return { kind: "before_and_after", before, after, total };
  }
  if (before > 0) {
    return { kind: "before_only", before, total };
  }
  if (after > 0) {
    return { kind: "after_only", after, total };
  }
  return { kind: "events_only", total };
}

/**
 * Timeline copy key. Pending is a fact when status is pending.
 * Allowlist-in-effect is the no-request case. Do not say "still blocked"
 * unless a fault is proven — this rail cannot prove that.
 */
export function gateTimelineI18nKey(
  summary: GateTimelineSummary,
  requestStatus?: string | null,
): string {
  const pending = requestStatus?.trim().toLowerCase() === "pending";
  switch (summary.kind) {
    case "before_and_after":
      return pending
        ? "radar.evidenceRail.gateTimeline.beforeAndAfterPending"
        : "radar.evidenceRail.gateTimeline.beforeAndAfter";
    case "before_only":
      return pending
        ? "radar.evidenceRail.gateTimeline.beforeOnlyPending"
        : "radar.evidenceRail.gateTimeline.beforeOnly";
    case "after_only":
      return pending
        ? "radar.evidenceRail.gateTimeline.afterOnlyPending"
        : "radar.evidenceRail.gateTimeline.afterOnly";
    case "events_only":
      return pending
        ? "radar.evidenceRail.gateTimeline.eventsOnlyPending"
        : "radar.evidenceRail.gateTimeline.eventsOnly";
  }
}

/** True when top pages span more than one document (bundle collision risk). */
export function topPagesSpanMultipleDocuments(
  pages: Array<{ documentId?: string }>,
): boolean {
  const ids = new Set<string>();
  for (const page of pages) {
    const id = page.documentId?.trim();
    if (id) ids.add(id);
  }
  return ids.size > 1;
}

function trimPath(value?: string | null): string {
  return value?.trim() ?? "";
}

function isWorkspaceInsightsOverview(path: string): boolean {
  const pathname = path.split("?")[0]?.replace(/\/+$/, "") ?? "";
  return pathname.endsWith("/insights/overview");
}

export type RadarEvidenceOpenPaths = {
  product: RadarProduct;
  workspaceSlug: string;
  dealRoomId?: string;
  linkId?: string;
  navigatePath?: string;
  insightsPath?: string;
  evidencePath?: string;
};

/**
 * Bottom-rail destination: Diligence rooms → Access; library gate holds →
 * share link (`/links/:id`); library approve keeps the Share inbox when
 * navigatePath already points there. Other products → the share
 * (`/links/:id` via insightsPath), never an arbitrary document in a bundle.
 */
export function radarEvidenceOpenPath(input: RadarEvidenceOpenPaths): string | null {
  const nav = trimPath(input.navigatePath);
  const insights = trimPath(input.insightsPath);
  const evidence = trimPath(input.evidencePath);
  if (input.product === "diligence_gate") {
    const remediation = diligenceRemediationPath(input.workspaceSlug, {
      dealRoomId: input.dealRoomId,
      linkId: input.linkId,
    });
    if (trimPath(input.dealRoomId)) {
      return remediation || nav || insights || evidence || null;
    }
    if (isLibraryShareInboxPath(nav)) {
      return nav;
    }
    return remediation || nav || insights || evidence || null;
  }
  if (insights && !isWorkspaceInsightsOverview(insights)) {
    return insights;
  }
  return evidence || nav || insights || null;
}

export function radarEvidenceOpenLabelKey(
  product: RadarProduct,
  openPath?: string | null,
): string {
  if (product === "diligence_gate" && isShareOrAccessRemediationPath(openPath)) {
    return "radar.evidenceRail.openShareInbox";
  }
  return "radar.evidenceRail.openFull";
}

function isLibraryShareInboxPath(path: string): boolean {
  if (!path) return false;
  const [pathname, query = ""] = path.split("?");
  const clean = pathname.replace(/\/+$/, "");
  if (!/(?:^|\/)documents$/.test(clean)) return false;
  return new URLSearchParams(query).get("tab") === "shared";
}

function isShareOrAccessRemediationPath(path?: string | null): boolean {
  const value = trimPath(path);
  if (!value) return false;
  if (isLibraryShareInboxPath(value)) return true;
  const query = value.split("?")[1] ?? "";
  return new URLSearchParams(query).get("tab") === "access";
}
