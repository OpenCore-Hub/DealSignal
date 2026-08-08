import type { Circle } from "@/types";
import { keyPageRulesForCircle } from "./heatScore";

/** Parse comma / newline separated keyword drafts (EN/ZH punctuation). */
export function parseKeywordDraft(raw: string): string[] {
  return raw
    .split(/[\n,，]/)
    .map((s) => s.trim())
    .filter(Boolean);
}

/** Categories shown in the workspace extras editor for a heat circle. */
export function editorCategoriesForCircle(
  circle: Circle,
  extras: Record<string, string[]>,
): string[] {
  const cats = new Set(keyPageRulesForCircle(circle).map((r) => r.category));
  for (const cat of Object.keys(extras)) {
    if ((extras[cat]?.length ?? 0) > 0) cats.add(cat);
  }
  cats.add("custom");
  return [...cats].sort((a, b) => {
    if (a === "custom") return 1;
    if (b === "custom") return -1;
    return a.localeCompare(b);
  });
}

/** Map saved extras → per-category textarea drafts. */
export function draftsFromExtras(
  categories: string[],
  extras: Record<string, string[]>,
): Record<string, string> {
  const out: Record<string, string> = {};
  for (const cat of categories) {
    out[cat] = (extras[cat] ?? []).join(", ");
  }
  return out;
}

/**
 * Build PUT extraKeywords from per-category drafts.
 * Empty categories are omitted. Unknown empty keys drop from the payload.
 */
export function extrasFromDrafts(drafts: Record<string, string>): Record<string, string[]> {
  const out: Record<string, string[]> = {};
  for (const [cat, raw] of Object.entries(drafts)) {
    const kws = parseKeywordDraft(raw);
    if (kws.length > 0) out[cat] = kws;
  }
  return out;
}
