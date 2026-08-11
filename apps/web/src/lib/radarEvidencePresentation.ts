import type { RadarProduct } from "@/lib/radarQueue";

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
 * Events without a request fall back to a simple hit count.
 */
export function gateTimelineSummary(
  events: EvidenceSecurityEvent[] | undefined | null,
  requestedAt?: string | null,
): GateTimelineSummary | null {
  if (!events?.length) return null;
  const total = events.length;
  if (!requestedAt) {
    return { kind: "events_only", total };
  }
  const pivot = new Date(requestedAt).getTime();
  if (Number.isNaN(pivot)) {
    return { kind: "events_only", total };
  }
  let before = 0;
  let after = 0;
  for (const e of events) {
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
