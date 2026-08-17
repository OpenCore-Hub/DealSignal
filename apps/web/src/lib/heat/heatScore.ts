import type {
  Circle,
  HeatLevel,
  HeatScoreConfig,
  HeatScoreResult,
  HeatScoreWeights,
  PageAnalytics,
} from "@/types";
import { displayablePageTitle } from "@/lib/insights/pageTitleDisplay";

const CIRCLE_CONFIGS: Record<Circle, HeatScoreConfig> = {
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
      financials: [
        "financial", "revenue", "projection", "unit economics", "burn", "runway",
        "财务", "营收", "收入", "预测", "损益", "现金流", "融资", "估值", "烧钱", "跑道",
      ],
      team: ["team", "founder", "advisor", "hiring", "团队", "创始人", "顾问", "招聘", "组织"],
      traction: [
        "traction", "growth", "metric", "mrr", "arr", "customer",
        "增长", "牵引", "指标", "客户", "留存", "转化",
      ],
      market: ["market", "tam", "sam", "som", "opportunity", "市场", "商机", "赛道", "竞争"],
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
      performance: ["performance", "return", "irr", "multiple", "nav", "业绩", "回报", "收益", "净值"],
      distribution: ["distribution", "dpi", "rvpi", "tvpi", "capital", "分配", "分红", "出资", "资本"],
      strategy: ["strategy", "thesis", "allocation", "outlook", "策略", "展望", "配置", "主题"],
      portfolio: ["portfolio", "company", "investment", "组合", "被投", "投资", "项目"],
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
      pricing: ["pricing", "price", "cost", "fee", "quote", "proposal", "定价", "价格", "报价", "费用", "方案"],
      security: ["security", "compliance", "soc2", "gdpr", "encryption", "安全", "合规", "加密", "隐私"],
      case_studies: [
        "case study", "customer story", "testimonial", "roi",
        "案例", "客户故事", "证言", "投资回报",
      ],
      implementation: [
        "implementation", "onboarding", "deployment", "timeline",
        "实施", "交付", "上线", "部署", "时间表", "落地",
      ],
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

/** Built-in keyword language filter — mirrors API heat.KeywordLang / Settings → Language. */
export type KeywordLang = "en" | "zh" | "any";

export function keywordLangFromI18n(lng: string | undefined): KeywordLang {
  const lower = (lng ?? "").trim().toLowerCase();
  if (!lower) return "any";
  if (lower === "zh" || lower.startsWith("zh-")) return "zh";
  return "en";
}

export function isCJKKeyword(kw: string): boolean {
  return /[\u3400-\u9fff]/.test(kw);
}

/** Filter built-in keywords by UI language. Workspace extras are never filtered here. */
export function filterKeywordsByLang(kws: readonly string[], lang: KeywordLang): string[] {
  if (lang === "any") return [...kws];
  return kws.filter((kw) => {
    const cjk = isCJKKeyword(kw);
    return lang === "zh" ? cjk : !cjk;
  });
}

/** Category → keywords for Insights disclosure / MSW (mirrors API heat.KeyPageRules). */
export function keyPageRulesForCircle(
  circle: Circle,
  lang: KeywordLang = "any",
): { category: string; keywords: string[] }[] {
  const config = CIRCLE_CONFIGS[circle];
  return Object.keys(config.keyPages)
    .sort()
    .map((category) => ({
      category,
      keywords: filterKeywordsByLang(config.keyPages[category], lang),
    }));
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
        .map((p) => {
          const text = (p.title ?? "").toLowerCase();
          const relevance = keyPages.reduce((sum, kw) => {
            const pattern = kw.toLowerCase();
            let count = 0;
            let idx = text.indexOf(pattern);
            while (idx !== -1) {
              count += 1;
              idx = text.indexOf(pattern, idx + pattern.length);
            }
            return sum + count;
          }, 0);
          return { page: p, relevance };
        })
        .filter(({ page, relevance }) => relevance > 0 && displayablePageTitle(page.title))
        .sort((a, b) => {
          // Rank by relevance weighted by view count so popular key pages surface first.
          const scoreA = a.relevance * (a.page.viewCount || 1);
          const scoreB = b.relevance * (b.page.viewCount || 1);
          return scoreB - scoreA;
        })
        .slice(0, 3)
        .map(({ page }) => displayablePageTitle(page.title))
    : [];

  return { score, level, trend, breakdown, topKeyPages };
}
