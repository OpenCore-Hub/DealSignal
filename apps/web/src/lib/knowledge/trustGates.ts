/**
 * Desk trust gates mirrored from apps/api/internal/knowledge/refusal.go.
 * Keep needle lists in sync — defense-in-depth for gaps / chips / rails.
 */

const UNGROUNDED_NEEDLES = [
  "does not contain an answer",
  "does not contain the answer",
  "do not contain an answer",
  "do not contain the answer",
  "context does not contain",
  "no relevant information",
  "cannot answer based on the",
  "can't answer based on the",
  "unable to answer based on",
  "cannot be answered based on",
  "can't be answered based on",
  "not enough information in the",
  "insufficient information in the",
  "未找到相关",
  "没有匹配",
  "无法从提供的",
  "上下文中没有",
  "资料中没有",
  "无法根据现有上下文",
  "无法根据提供的上下文",
  "无法根据上下文",
  "无法根据现有资料",
  "无法根据现有文档",
  "无法回答该问题",
  "不能根据现有上下文",
  "不能根据上下文",
  "不足以回答",
  "没有足够信息回答",
  "现有上下文回答",
  "现有材料不足以",
  "不足以确定",
  "无法确定该问题",
  "i cannot determine",
  "i can't determine",
  "cannot determine from",
  "can't determine from",
  "unable to determine from",
  "not possible to answer from",
] as const;

const META_NEEDLES = [
  "无法回答",
  "不能回答",
  "i cannot answer",
  "i can't answer",
  "cannot be answered",
  "can't be answered",
  "based on the provided context, i",
  "based on the existing context, i",
  "根据现有资料无法",
  "没有在提供的上下文",
  "未在提供的上下文",
  "提供的上下文未包含",
  "提供的上下文中未包含",
  "the provided context does not include",
  "provided context does not include",
  "现有材料不足以",
  "不足以确定",
  "i cannot determine",
  "i can't determine",
  "cannot determine from",
] as const;

const OUT_OF_ROOM_NEEDLES = [
  "ebitda multiple",
  "trading multiple",
  "valuation multiple",
  "typically 12x",
  "typically 10x",
  "market-standard",
  "market standard",
  "industry average",
  "industry standard",
  "industry norm",
  "market comps",
  "comparable companies",
  "saas m&a",
  "pe funds usually",
  "private equity usually",
  "nvca model",
  "how do pe funds",
  "how do sponsors usually",
  "同行一般",
  "市场一般怎么",
  "市场惯例",
  "行业惯例",
  "行业通常",
  "对标公司",
  "可比公司",
  "估值倍数通常",
  "倍数通常为",
  "倍数通常是",
] as const;

/** Soft corpus/model refusal — keep in sync with ungroundedAnswerNeedles. */
export function isUngroundedKnowledgeAnswer(answer?: string | null): boolean {
  const text = (answer ?? "").trim().toLowerCase();
  if (!text) return false;
  return UNGROUNDED_NEEDLES.some((n) => text.includes(n));
}

/** Assistant meta that must never become a gap / chip. */
export function looksLikeNonRoomFactMeta(text?: string | null): boolean {
  const t = (text ?? "").trim();
  if (!t) return true;
  if (isUngroundedKnowledgeAnswer(t)) return true;
  const lower = t.toLowerCase();
  return META_NEEDLES.some((m) => lower.includes(m) || t.includes(m));
}

/** Industry/market trivia detached from this room’s facts. */
export function looksLikeOutOfRoomGeneralKnowledge(text?: string | null): boolean {
  const t = (text ?? "").trim();
  if (!t) return false;
  const lower = t.toLowerCase();
  return OUT_OF_ROOM_NEEDLES.some((n) => lower.includes(n) || t.includes(n));
}

/** Desk-safe gap / open-question / chip text (FE defense-in-depth). */
export function isPromotableDeskFollowUpText(text?: string | null): boolean {
  const t = (text ?? "").trim();
  if (t.length < 6) return false;
  if (looksLikeNonRoomFactMeta(t)) return false;
  if (looksLikeOutOfRoomGeneralKnowledge(t)) return false;
  return true;
}

export function filterPromotableDeskTexts(texts: string[]): string[] {
  return texts.map((g) => g.trim()).filter((g) => isPromotableDeskFollowUpText(g));
}
