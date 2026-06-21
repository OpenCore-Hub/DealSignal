import type {
  Circle,
  HeatLevel,
  HeatScoreConfig,
  HeatScoreResult,
  HeatScoreWeights,
  Link,
  PageAnalytics,
} from "@/types";

export const CIRCLE_CONFIGS: Record<Circle, HeatScoreConfig> = {
  founder: {
    name: "founder",
    weights: {
      opens: 5,
      revisits: 15,
      avgDurationMinutes: 10,
      keyPageViews: 20,
      forwardSignals: 15,
      downloads: 10,
      bouncePenalty: 10,
    },
    keyPages: {
      "fundamentals": ["financials", "traction", "market", "team", "use-of-funds"],
      "narrative": ["deck", "one-pager", "executive-summary"],
      "trust": ["data-room", "cap-table", "due-diligence"],
    },
    thresholds: { hot: 80, warm: 50, cold: 0 },
  },
  investor_ir: {
    name: "investor_ir",
    weights: {
      opens: 5,
      revisits: 15,
      avgDurationMinutes: 10,
      keyPageViews: 20,
      forwardSignals: 15,
      downloads: 10,
      bouncePenalty: 10,
    },
    keyPages: {
      "fundamentals": ["nav", "performance", "attribution", "portfolio"],
      "governance": ["gp-report", "lpa", "side-letters", "aml/kyc"],
      "operations": ["capital-calls", "distributions", "audits"],
    },
    thresholds: { hot: 75, warm: 45, cold: 0 },
  },
  sales: {
    name: "sales",
    weights: {
      opens: 5,
      revisits: 15,
      avgDurationMinutes: 10,
      keyPageViews: 20,
      forwardSignals: 15,
      downloads: 10,
      bouncePenalty: 10,
    },
    keyPages: {
      "problem": ["problem", "challenges", "roi"],
      "solution": ["product", "solution", "features", "pricing"],
      "proof": ["case-study", "testimonials", "security", "implementation"],
    },
    thresholds: { hot: 75, warm: 45, cold: 0 },
  },
};

export interface HeatScoreInput {
  opens: number;
  revisits: number;
  avgDurationMinutes: number;
  keyPageViews: number;
  forwardSignals: number;
  downloads: number;
  bouncePenalty: number;
}

function calculateComponent(
  key: keyof HeatScoreWeights,
  input: HeatScoreInput,
  weights: HeatScoreWeights
): number {
  const value = input[key] ?? 0;
  const weight = weights[key] ?? 0;

  switch (key) {
    case "opens":
      return Math.min(value, 10) * weight;
    case "revisits":
      return value * weight;
    case "avgDurationMinutes":
      return value * weight;
    case "keyPageViews":
      return value * weight;
    case "forwardSignals":
      return value * weight;
    case "downloads":
      return value * weight;
    case "bouncePenalty":
      return -Math.min(value, 5) * weight;
    default:
      return 0;
  }
}

export function computeHeatScore(
  circle: Circle,
  input: HeatScoreInput,
  pageAnalytics?: PageAnalytics[]
): HeatScoreResult {
  const config = CIRCLE_CONFIGS[circle];
  const weights = config.weights;

  const breakdown: Record<string, number> = {};
  for (const key of Object.keys(weights) as (keyof HeatScoreWeights)[]) {
    breakdown[key] = calculateComponent(key, input, weights);
  }

  let score = Object.values(breakdown).reduce((sum, v) => sum + v, 0);
  score = Math.max(0, Math.min(100, Math.round(score)));

  let level: HeatLevel = "cold";
  if (score >= config.thresholds.hot) level = "hot";
  else if (score >= config.thresholds.warm) level = "warm";

  const trend: HeatScoreResult["trend"] =
    input.revisits > 0 && input.avgDurationMinutes > 1
      ? "rising"
      : input.avgDurationMinutes < 0.5 && input.opens > 0
      ? "falling"
      : "stable";

  const keyPages = Object.values(config.keyPages).flat();
  const topKeyPages = pageAnalytics
    ? pageAnalytics
        .filter((p) => keyPages.some((kw) => String(p.pageNumber).includes(kw)))
        .sort((a, b) => b.viewCount - a.viewCount)
        .slice(0, 3)
        .map((p) => `Page ${p.pageNumber}`)
    : [];

  return { score, level, trend, breakdown, topKeyPages };
}

export function summarizeLinkHeat(
  link: Link,
  circle: Circle = "founder"
): HeatScoreResult {
  const input: HeatScoreInput = {
    opens: link.accessCount,
    revisits: Math.floor((link.accessCount ?? 0) / 2),
    avgDurationMinutes: (link.avgDurationSeconds ?? 0) / 60,
    keyPageViews: 0,
    forwardSignals: 0,
    downloads: 0,
    bouncePenalty: 0,
  };
  return computeHeatScore(circle, input);
}
