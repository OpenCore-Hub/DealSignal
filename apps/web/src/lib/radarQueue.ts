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
  state?: "open";
  scenario?: RadarScenario | string;
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
  depth: "base" | "p0" | "p1" | "lite" | string;
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
  /** Ranking-critical facets that failed to load — not “clean zero”. */
  degradedSections?: Array<
    | "internal_emails"
    | "capture_metrics"
    | "ip_metrics"
    | string
  >;
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

/** Decrement all + product bucket after optimistic clear/snooze/ignore. */
export function decrementRadarCounts(
  counts: Record<string, number> | undefined,
  itemCount: number,
  product: RadarProduct | undefined,
): Record<string, number> {
  const next: Record<string, number> = {
    ...(counts ?? {}),
    all: Math.max(0, (counts?.all ?? itemCount) - 1),
  };
  if (product) {
    next[product] = Math.max(0, (counts?.[product] ?? 0) - 1);
  }
  return next;
}

/**
 * Role-lens product urgency (lower = sooner). Mirrors apps/api/internal/radar
 * productRankForCircle — used by MSW to re-rank when ?circle= changes.
 */
export function productRankForCircle(
  circle: RadarCircle,
  product: RadarProduct,
): number {
  switch (circle) {
    case "investor_ir":
      switch (product) {
        case "leak_watch":
        case "buying_window":
          return 0;
        case "diligence_gate":
        case "commitment_ask":
          return 1;
        case "access_decay":
          return 2;
        case "abuse_guard":
          return 3;
        default:
          return 9;
      }
    case "sales":
      switch (product) {
        case "buying_window":
          return 0;
        case "diligence_gate":
        case "access_decay":
          return 1;
        case "commitment_ask":
        case "leak_watch":
          return 2;
        case "abuse_guard":
          return 3;
        default:
          return 9;
      }
    default:
      switch (product) {
        case "diligence_gate":
        case "leak_watch":
          return 0;
        case "commitment_ask":
          return 1;
        case "buying_window":
          return 2;
        case "access_decay":
        case "abuse_guard":
          return 3;
        default:
          return 9;
      }
  }
}

/** Re-rank a compiled feed for a role lens (MSW / client preview). */
export function applyRadarCircleLens(
  feed: RadarFeed,
  circle: RadarCircle,
): RadarFeed {
  const byRank = (a: RadarWorkItem, b: RadarWorkItem) => {
    const ra = productRankForCircle(circle, a.product);
    const rb = productRankForCircle(circle, b.product);
    if (ra !== rb) return ra - rb;
    return a.createdAt < b.createdAt ? -1 : a.createdAt > b.createdAt ? 1 : 0;
  };
  const items = [...feed.items].sort(byRank);
  const strands = feed.strands
    .map((s) => ({
      ...s,
      items: [...s.items].sort(byRank),
    }))
    .sort((a, b) => {
      const pa = a.items[0]
        ? productRankForCircle(circle, a.items[0].product)
        : 9;
      const pb = b.items[0]
        ? productRankForCircle(circle, b.items[0].product)
        : 9;
      return pa - pb;
    });
  return {
    ...feed,
    items,
    strands,
    nextUp: items[0] ?? null,
    lens: circle,
    lensSource: "query",
  };
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

export interface RadarEvidenceAccessRequest {
  email: string;
  reason?: string;
  signerName?: string;
  status: string;
  requestedAt: string;
  surface: "document_link" | "deal_room_link" | "room" | string;
}

export interface RadarEvidencePack {
  itemId: string;
  product: RadarProduct;
  headline: string;
  headlineCode?: string;
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
  /** Pending authorization application — primary Diligence gate evidence. */
  accessRequest?: RadarEvidenceAccessRequest;
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
  /** Facets that failed to load — never treat as “zero engagement”. */
  degradedSections?: Array<
    | "metrics"
    | "top_pages"
    | "recent_visitors"
    | "security_events"
    | "link_id"
    | "access_request"
    | string
  >;
}

/** True when any 24h engagement counter is non-zero. */
export function evidenceMetricsHaveActivity(
  metrics?: RadarEvidencePack["metrics"] | null,
): boolean {
  if (!metrics) return false;
  return (
    metrics.opens24h > 0 ||
    metrics.uniqueVisitors24h > 0 ||
    metrics.forwardSignals24h > 0 ||
    metrics.downloads24h > 0 ||
    (metrics.captureAttempts24h ?? 0) > 0
  );
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
