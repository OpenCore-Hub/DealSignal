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
      opens: 3,
      revisits: 18,
      avgDurationMinutes: 12,
      keyPageViews: 25,
      forwardSignals: 15,
      downloads: 8,
      bouncePenalty: 10,
    },
    keyPages: {
      financials: ["financial", "revenue", "projection", "unit economics", "burn", "runway"],
      team: ["team", "founder", "advisor", "hiring"],
      traction: ["traction", "growth", "metric", "mrr", "arr", "customer"],
      market: ["market", "tam", "sam", "som", "opportunity"],
    },
    thresholds: { hot: 75, warm: 40, cold: 0 },
  },
  investor_ir: {
    name: "investor_ir",
    weights: {
      opens: 2,
      revisits: 12,
      avgDurationMinutes: 10,
      keyPageViews: 20,
      forwardSignals: 8,
      downloads: 5,
      bouncePenalty: 10,
    },
    keyPages: {
      performance: ["performance", "return", "irr", "multiple", "nav"],
      distribution: ["distribution", "dpi", "rvpi", "tvpi", "capital"],
      strategy: ["strategy", "thesis", "allocation", "outlook"],
      portfolio: ["portfolio", "company", "investment"],
    },
    thresholds: { hot: 70, warm: 35, cold: 0 },
  },
  sales: {
    name: "sales",
    weights: {
      opens: 2,
      revisits: 15,
      avgDurationMinutes: 10,
      keyPageViews: 28,
      forwardSignals: 20,
      downloads: 5,
      bouncePenalty: 12,
    },
    keyPages: {
      pricing: ["pricing", "price", "cost", "fee", "quote", "proposal"],
      security: ["security", "compliance", "soc2", "gdpr", "encryption"],
      case_studies: ["case study", "customer story", "testimonial", "roi"],
      implementation: ["implementation", "onboarding", "deployment", "timeline"],
    },
    thresholds: { hot: 72, warm: 38, cold: 0 },
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
        .filter((p) => {
          const text = [p.title, String(p.pageNumber)].filter(Boolean).join(" ").toLowerCase();
          return keyPages.some((kw) => text.includes(kw.toLowerCase()));
        })
        .sort((a, b) => b.viewCount - a.viewCount)
        .slice(0, 3)
        .map((p) => p.title ?? `Page ${p.pageNumber}`)
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
