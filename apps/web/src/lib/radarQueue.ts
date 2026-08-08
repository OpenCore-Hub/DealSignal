/**
 * Client helpers for Deal Radar feeds compiled by GET /radar.
 * Ranking, productization, and coalesce happen on the server.
 */

export type RadarProduct =
  | "buying_window"
  | "diligence_gate"
  | "commitment_ask"
  | "leak_watch"
  | "access_decay"
  | "abuse_guard";

export type RadarFilter = "all" | RadarProduct;

export type RadarVerb =
  | "approve"
  | "reply"
  | "email"
  | "renew"
  | "review"
  | "open";

export type RadarPriority = "high" | "medium" | "low";

export type RadarConfidence = "low" | "medium" | "high";

export type RadarCircle = "founder" | "investor_ir" | "sales";

/** Deal-room template business scenario (not a role lens). */
export type RadarScenario =
  | "startup-fundraising"
  | "raising-first-fund"
  | "ma-acquisition"
  | "series-a-plus"
  | "real-estate-transaction"
  | "fund-management"
  | "portfolio-management"
  | "project-management"
  | "sales-dataroom"
  | "custom";

export type RadarLensSource = "query" | "inferred" | "default";

export type RadarOutcome =
  | "acted"
  | "false_positive"
  | "renewed"
  | "approved"
  | "replied"
  | "other";

export const RADAR_CIRCLES: RadarCircle[] = [
  "founder",
  "investor_ir",
  "sales",
];

export type RadarEvidenceKind =
  | "forward"
  | "download"
  | "capture"
  | "key_page"
  | "ask"
  | "engagement"
  | "gate"
  | "access"
  | "abuse"
  | "coalesced";

export interface RadarEvidenceChip {
  kind: RadarEvidenceKind | string;
  count?: number;
}

export type RadarWhyNowCode =
  | "sla_overdue"
  | "coalesced"
  | "buying_window"
  | "diligence_gate"
  | "commitment_ask"
  | "leak_watch"
  | "access_decay"
  | "abuse_guard";

export interface RadarWorkItem {
  id: string;
  product: RadarProduct;
  headline: string;
  subtitle: string;
  actor?: string;
  verb: RadarVerb;
  priority: RadarPriority;
  confidence?: RadarConfidence;
  slaDueAt: string;
  createdAt: string;
  dealKey: string;
  dealName: string;
  dealRoomId?: string;
  linkId?: string;
  documentId?: string;
  contactId?: string;
  actionId: string;
  signalId?: string;
  navigatePath?: string;
  evidencePath?: string;
  contactEmail?: string;
  documentTitle?: string;
  coalescedFrom?: string[];
  whyNowCode?: RadarWhyNowCode | string;
  whyNowHours?: number;
  evidence?: RadarEvidenceChip[];
  /** Scenario Pack narrative id; prefer i18n over raw headline. */
  headlineCode?: string;
  state?: "open" | "snoozed" | "done" | "dismissed" | string;
  scenario?: RadarScenario | string;
  /** Populated client-side for mailto CTAs. */
  mailtoHref?: string | null;
}

export interface RadarStrand {
  dealKey: string;
  dealName: string;
  dealRoomId?: string;
  scenario?: RadarScenario | string;
  items: RadarWorkItem[];
}

export interface RadarNoiseHint {
  scenario?: string;
  product: RadarProduct;
  falsePositiveRate: number;
  sample: number;
  demoteBoost: number;
}

export interface RadarScenarioPackMeta {
  scenario: string;
  defaultCircle: RadarCircle | string;
  depth: "base" | "p0" | string;
  keyPageCategories?: string[];
  insightsKpi?: string[];
}

export interface RadarFeed {
  nextUp: RadarWorkItem | null;
  strands: RadarStrand[];
  items: RadarWorkItem[];
  clearedToday: number;
  counts: Record<string, number>;
  lens?: RadarCircle;
  defaultLens?: RadarCircle;
  lensSource?: RadarLensSource | string;
  scenarios?: string[];
  scenarioPack?: RadarScenarioPackMeta | null;
  noiseHints?: RadarNoiseHint[];
}

export function parseRadarCircle(
  raw: string | null | undefined,
): RadarCircle {
  if (raw === "investor_ir" || raw === "sales" || raw === "founder") {
    return raw;
  }
  return "founder";
}

/** i18n key for why-now copy; scenario pack narrative when present. */
export function radarWhyNowKey(item: Pick<RadarWorkItem, "scenario" | "whyNowCode">): string {
  if (!item.whyNowCode) return "";
  if (item.scenario) {
    return `radar.scenario.${item.scenario}.whyNow.${item.whyNowCode}`;
  }
  return `radar.whyNow.${item.whyNowCode}`;
}

/** Generic why-now key used as fallback when a scenario key is missing. */
export function radarWhyNowFallbackKey(
  item: Pick<RadarWorkItem, "whyNowCode">,
): string {
  if (!item.whyNowCode) return "";
  return `radar.whyNow.${item.whyNowCode}`;
}

/** Scenario Pack headline i18n key (empty when no headlineCode). */
export function radarHeadlineKey(
  item: Pick<RadarWorkItem, "scenario" | "headlineCode">,
): string {
  if (!item.headlineCode || !item.scenario) return "";
  return `radar.scenario.${item.scenario}.headline.${item.headlineCode}`;
}

/** True when a keydown target is editable (skip radar day-clear shortcuts). */
export function isEditableKeyboardTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  if (target.isContentEditable) return true;
  const tag = target.tagName;
  return tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT";
}

/** Default completion outcome when the user marks done without choosing. */
export function defaultOutcomeForProduct(product: RadarProduct): RadarOutcome {
  switch (product) {
    case "diligence_gate":
      return "approved";
    case "commitment_ask":
      return "replied";
    case "access_decay":
      return "renewed";
    default:
      return "acted";
  }
}

/** Outcomes offered in the complete menu for a product. */
export function outcomesForProduct(product: RadarProduct): RadarOutcome[] {
  switch (product) {
    case "leak_watch":
    case "abuse_guard":
      return ["acted", "false_positive"];
    case "diligence_gate":
      return ["approved", "false_positive"];
    case "commitment_ask":
      return ["replied", "acted"];
    case "access_decay":
      return ["renewed", "acted"];
    default:
      return ["acted"];
  }
}

export const RADAR_FILTERS: RadarFilter[] = [
  "all",
  "buying_window",
  "diligence_gate",
  "commitment_ask",
  "leak_watch",
  "access_decay",
  "abuse_guard",
];

/** Map legacy inbox filter query values to product filters. */
const LEGACY_FILTER_MAP: Record<string, RadarFilter> = {
  follow_up: "buying_window",
  ask: "commitment_ask",
  approve: "diligence_gate",
  risk: "leak_watch",
};

export function parseRadarFilter(raw: string | null | undefined): RadarFilter {
  if (!raw) return "all";
  if (raw === "all") return "all";
  if ((RADAR_FILTERS as string[]).includes(raw)) return raw as RadarFilter;
  return LEGACY_FILTER_MAP[raw] ?? "all";
}

export function filterRadarItems(
  items: RadarWorkItem[],
  filter: RadarFilter,
): RadarWorkItem[] {
  if (filter === "all") return items;
  return items.filter((item) => item.product === filter);
}

export function countRadarFilters(
  items: RadarWorkItem[],
  serverCounts?: Record<string, number>,
): Record<RadarFilter, number> {
  if (serverCounts) {
    return {
      all: serverCounts.all ?? items.length,
      buying_window: serverCounts.buying_window ?? 0,
      diligence_gate: serverCounts.diligence_gate ?? 0,
      commitment_ask: serverCounts.commitment_ask ?? 0,
      leak_watch: serverCounts.leak_watch ?? 0,
      access_decay: serverCounts.access_decay ?? 0,
      abuse_guard: serverCounts.abuse_guard ?? 0,
    };
  }
  return {
    all: items.length,
    buying_window: items.filter((i) => i.product === "buying_window").length,
    diligence_gate: items.filter((i) => i.product === "diligence_gate").length,
    commitment_ask: items.filter((i) => i.product === "commitment_ask").length,
    leak_watch: items.filter((i) => i.product === "leak_watch").length,
    access_decay: items.filter((i) => i.product === "access_decay").length,
    abuse_guard: items.filter((i) => i.product === "abuse_guard").length,
  };
}

export function buildFollowUpMailto(
  email: string,
  subject: string,
  body: string,
): string {
  const params = new URLSearchParams();
  params.set("subject", subject);
  params.set("body", body);
  return `mailto:${email}?${params.toString()}`;
}

/** Attach localized mailto hrefs for email verbs. */
export function withMailtoHrefs(
  items: RadarWorkItem[],
  opts: {
    subject: (documentTitle: string) => string;
    body: (args: {
      email: string;
      document: string;
      action: string;
    }) => string;
  },
): RadarWorkItem[] {
  return items.map((item) => {
    if (item.verb !== "email" || !item.contactEmail) return item;
    const doc = item.documentTitle || item.headline;
    return {
      ...item,
      mailtoHref: buildFollowUpMailto(
        item.contactEmail,
        opts.subject(doc),
        opts.body({
          email: item.contactEmail,
          document: doc,
          action: item.headline,
        }),
      ),
    };
  });
}

/** Group filtered items into deal strands (preserves item order / first-seen deal). */
export function groupIntoStrands(items: RadarWorkItem[]): RadarStrand[] {
  const idx = new Map<string, number>();
  const strands: RadarStrand[] = [];
  for (const item of items) {
    const existing = idx.get(item.dealKey);
    if (existing === undefined) {
      idx.set(item.dealKey, strands.length);
      strands.push({
        dealKey: item.dealKey,
        dealName: item.dealName,
        dealRoomId: item.dealRoomId,
        scenario: item.scenario,
        items: [item],
      });
      continue;
    }
    strands[existing].items.push(item);
  }
  return strands;
}

export interface RadarEvidencePack {
  itemId: string;
  product: RadarProduct;
  headline: string;
  whyNow?: string;
  whyNowCode?: RadarWhyNowCode | string;
  whyNowHours?: number;
  actor?: string;
  dealName?: string;
  linkId?: string;
  documentId?: string;
  navigatePath?: string;
  evidencePath?: string;
  insightsPath?: string;
  metrics?: {
    opens24h: number;
    uniqueVisitors24h: number;
    forwardSignals24h: number;
    downloads24h: number;
    captureAttempts24h?: number;
  };
  keyPageTitles?: string[];
  topPages?: Array<{
    pageNumber: number;
    views: number;
    avgDurationSeconds: number;
  }>;
  recentVisitors?: Array<{
    visitorId: string;
    email?: string;
    totalViews: number;
    lastAccessAt?: string;
  }>;
  securityEvents?: Array<{
    eventType: string;
    reason?: string;
    email?: string;
    createdAt: string;
  }>;
}

/**
 * Filter server-compiled strands (preserve deal order / item order).
 * Optionally drop the Next Up id so strands only show the remainder.
 */
export function filterServerStrands(
  strands: RadarStrand[],
  filter: RadarFilter,
  excludeId?: string | null,
): RadarStrand[] {
  const out: RadarStrand[] = [];
  for (const strand of strands) {
    const items = strand.items.filter((item) => {
      if (excludeId && item.id === excludeId) return false;
      if (filter !== "all" && item.product !== filter) return false;
      return true;
    });
    if (items.length === 0) continue;
    out.push({ ...strand, items });
  }
  return out;
}

/** Flat selection order: next-up first, then strand items in server order. */
export function flatRadarOrder(
  nextUp: RadarWorkItem | undefined,
  strands: RadarStrand[],
): RadarWorkItem[] {
  const seen = new Set<string>();
  const out: RadarWorkItem[] = [];
  if (nextUp) {
    out.push(nextUp);
    seen.add(nextUp.id);
  }
  for (const strand of strands) {
    for (const item of strand.items) {
      if (seen.has(item.id)) continue;
      seen.add(item.id);
      out.push(item);
    }
  }
  return out;
}
