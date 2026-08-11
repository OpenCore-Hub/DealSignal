/**
 * Dense Deal Radar feed for MSW / Playwright UX acceptance.
 * Covers all six products with multi-deal strands and product-shaped evidence.
 */
import { groupIntoStrands, type RadarFeed, type RadarProduct, type RadarWorkItem } from "@/lib/radarQueue";

export const RADAR_STRESS_PRODUCTS: RadarProduct[] = [
  "buying_window",
  "diligence_gate",
  "commitment_ask",
  "leak_watch",
  "access_decay",
  "abuse_guard",
];

const DEALS = [
  { key: "deal:northstar", name: "Northstar Series A", scenario: "startup-fundraising" },
  { key: "deal:harbor", name: "Harbor Fund I", scenario: "raising-first-fund" },
  { key: "deal:apex", name: "Apex M&A", scenario: "ma-acquisition" },
  { key: "deal:riverview", name: "Riverview Closing", scenario: "real-estate-transaction" },
  { key: "deal:folio", name: "Portfolio IR Q3", scenario: "portfolio-management" },
  { key: "deal:pipeline", name: "Enterprise Pipeline", scenario: "sales-dataroom" },
  { key: "deal:seed", name: "Seed Round Deck", scenario: "startup-fundraising" },
  { key: "deal:bridge", name: "Bridge Diligence", scenario: "series-a-plus" },
  { key: "deal:ops", name: "Fund Ops Room", scenario: "fund-management" },
  { key: "deal:project", name: "Project Atlas", scenario: "project-management" },
] as const;

const ACTORS = [
  "sarah.chen@a16z.com",
  "lp@vc.com",
  "analyst@sequoia.com",
  "buyer@corp.com",
  "ir@pension.org",
  "partner@benchmark.com",
];

type StressOpts = {
  /** Items per product (default 12 → 72 total). */
  perProduct?: number;
  workspaceSlug?: string;
};

function verbFor(product: RadarProduct): RadarWorkItem["verb"] {
  switch (product) {
    case "diligence_gate":
      return "approve";
    case "commitment_ask":
      return "reply";
    case "buying_window":
      return "email";
    case "access_decay":
      return "renew";
    default:
      return "review";
  }
}

function headlineFor(
  product: RadarProduct,
  dealName: string,
  actor: string,
  i: number,
): { headline: string; subtitle: string; evidence: RadarWorkItem["evidence"] } {
  switch (product) {
    case "buying_window":
      return {
        headline: `${actor} reopened ${dealName} (hot window #${i + 1})`,
        subtitle: "Multiple key-page views in the last 2h — reply while intent is warm.",
        evidence: [
          { kind: "engagement", count: 6 + (i % 4) },
          { kind: "key_page", count: 2 },
        ],
      };
    case "diligence_gate":
      return {
        headline: `Approve access request from ${actor}`,
        subtitle: "Waiting at the email gate — still blocked after requesting.",
        evidence: [{ kind: "gate", count: 4 }, { kind: "access", count: 1 }],
      };
    case "commitment_ask":
      return {
        headline: `Reply to Ask from ${actor}`,
        subtitle: "Formal / visitor Ask waiting on host response.",
        evidence: [{ kind: "ask", count: 1 + (i % 3) }],
      };
    case "leak_watch":
      return {
        headline: `Forward risk on ${dealName}`,
        subtitle: "Unrecognized forward / download pattern — review before it spreads.",
        evidence: [
          { kind: "forward", count: 2 + (i % 3) },
          { kind: "download", count: 1 },
        ],
      };
    case "access_decay":
      return {
        headline: `Renew expiring access for ${dealName}`,
        subtitle: "Link expired or near expiry — visitor still trying to open.",
        evidence: [{ kind: "access", count: 1 }],
      };
    case "abuse_guard":
      return {
        headline: `Abuse / rate-limit on ${dealName}`,
        subtitle: "Capture attempt or Ask rate limit — tighten before quota burn.",
        evidence: [{ kind: "abuse", count: 1 }, { kind: "capture", count: i % 2 }],
      };
  }
}

/** Build a dense, product-balanced radar feed for UX acceptance. */
export function buildRadarStressFeed(opts: StressOpts = {}): RadarFeed {
  const perProduct = opts.perProduct ?? 12;
  const slug = opts.workspaceSlug ?? "acme-capital";
  const now = Date.now();
  const items: RadarWorkItem[] = [];

  for (const product of RADAR_STRESS_PRODUCTS) {
    for (let i = 0; i < perProduct; i += 1) {
      const deal = DEALS[(i + RADAR_STRESS_PRODUCTS.indexOf(product)) % DEALS.length];
      const actor = ACTORS[(i + RADAR_STRESS_PRODUCTS.indexOf(product)) % ACTORS.length];
      const id = `stress_${product}_${i}`;
      const createdAt = new Date(now - (i * 37 + RADAR_STRESS_PRODUCTS.indexOf(product) * 11) * 60_000).toISOString();
      const slaDueAt = new Date(now + (i % 5 === 0 ? -2 : 4 + i) * 3_600_000).toISOString();
      const copy = headlineFor(product, deal.name, actor, i);
      const linkId = `link_stress_${deal.key.replace(":", "_")}_${i}`;
      items.push({
        id,
        product,
        headline: copy.headline,
        subtitle: copy.subtitle,
        actor,
        verb: verbFor(product),
        priority: i % 5 === 0 ? "high" : i % 3 === 0 ? "medium" : "low",
        confidence:
          product === "leak_watch" ? (i % 2 === 0 ? "medium" : "low") : undefined,
        slaDueAt,
        createdAt,
        dealKey: deal.key,
        dealName: deal.name,
        dealRoomId: deal.key.startsWith("deal:") ? `room_${deal.key.slice(5)}` : undefined,
        linkId,
        documentId: `doc_stress_${i % 6}`,
        contactEmail: actor,
        documentTitle: deal.name,
        actionId: id,
        signalId: `sig_stress_${product}_${i}`,
        navigatePath:
          product === "diligence_gate"
            ? `/${slug}/documents?tab=shared&linkId=${linkId}`
            : `/${slug}/links/${linkId}`,
        evidencePath: `/${slug}/links/${linkId}`,
        whyNowCode: product,
        whyNowHours: 1 + (i % 6),
        evidence: copy.evidence,
        scenario: deal.scenario,
        state: "open",
        headlineCode: `radar.headline.${product}`,
      });
    }
  }

  // Stable sort: overdue / high priority first within product mix for next-up realism.
  items.sort((a, b) => {
    const aOver = new Date(a.slaDueAt).getTime() < now ? 0 : 1;
    const bOver = new Date(b.slaDueAt).getTime() < now ? 0 : 1;
    if (aOver !== bOver) return aOver - bOver;
    const pri = { high: 0, medium: 1, low: 2 };
    if (pri[a.priority] !== pri[b.priority]) return pri[a.priority] - pri[b.priority];
    return new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime();
  });

  const counts: Record<string, number> = { all: items.length };
  for (const p of RADAR_STRESS_PRODUCTS) {
    counts[p] = items.filter((i) => i.product === p).length;
  }

  return {
    nextUp: items[0] ?? null,
    strands: groupIntoStrands(items),
    items,
    clearedToday: 7,
    counts,
    lens: "founder",
    defaultLens: "founder",
    lensSource: "default",
    scenarioPack: {
      scenario: "startup-fundraising",
      defaultCircle: "founder",
      depth: "p0",
    },
    noiseHints: [
      {
        product: "leak_watch",
        falsePositiveRate: 0.38,
        sample: Math.max(5, counts.leak_watch ?? 5),
        demoteBoost: 2,
      },
    ],
  };
}
