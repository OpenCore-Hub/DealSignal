import type { DealRoomKnowledgeQueryHit } from "@/types";
import { isPromotableDeskFollowUpText, isUngroundedKnowledgeAnswer, looksLikeNonRoomFactMeta } from "@/lib/knowledge/trustGates";

/** Locale-ready follow-up chip — render with t(messageKey, params). */
export interface RoomFollowUpSuggestion {
  id: string;
  messageKey: string;
  params?: Record<string, string>;
  kind?: "verify" | "conflict" | "consequence" | "cover" | "narrow";
  slot?: number;
}

export interface FollowUpPackItem {
  id: string;
  prompt: string;
  keywords?: string[];
  covered?: boolean;
}

export interface FollowUpTurnInput {
  refused: boolean;
  resultStatus: string;
  hits: Pick<DealRoomKnowledgeQueryHit, "sourceName" | "chunkId" | "text">[];
  /** Optional answer text — mirrors BE needsFollowUpNarrowing soft-refusal probe. */
  answer?: string | null;
  question?: string | null;
  claims?: Array<{ text: string; hitIds?: string[]; confidence?: string }>;
  unresolved?: string[];
  packItems?: FollowUpPackItem[];
}

/** Max distinct files in the follow-up coverage set (ceiling §3.1). */
export const FOLLOW_UP_COVERAGE_MAX = 3;
const FOLLOW_UP_SLOT_MAX = 3;
const FOLLOW_UP_ANCHOR_MAX = 24;
const FOLLOW_UP_CLAIM_PREVIEW = 48;

/** Narrowing prompts for refuse / no_hits / error — shared by templates. */
export function followUpNeedsNarrowing(turn: FollowUpTurnInput): boolean {
  if (turn.refused || isUngroundedKnowledgeAnswer(turn.answer)) return true;
  const answer = (turn.answer ?? "").trim();
  // Composer-only: RAG context-missing meta is not a slot0 gap (mirrors BE).
  if (answer && looksLikeNonRoomFactMeta(answer)) return true;
  switch (turn.resultStatus) {
    case "refused":
    case "no_hits":
    case "error":
      return true;
    default:
      break;
  }
  if (
    answer &&
    !questionTopicGrounded(turn) &&
    !hasActionableUnresolvedTurn(turn)
  ) {
    return true;
  }
  return false;
}

/**
 * Ordered unique source names from hits (retrieval order), capped for chips.
 */
export function coverageSourceNames(
  hits: FollowUpTurnInput["hits"],
  max = FOLLOW_UP_COVERAGE_MAX,
): string[] {
  const out: string[] = [];
  const seen = new Set<string>();
  for (const h of hits) {
    const name = (h.sourceName ?? "").trim();
    if (!name) continue;
    const key = name.toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(name);
    if (out.length >= max) break;
  }
  return out;
}

/**
 * Local split fallback for suggested follow-ups (mirrors BE buildSplitFollowUps).
 * First paint: narrowing, or slot0 continuation (+ rewritten cover when pack items exist).
 * Liability / definition / exception templates are gone from composer.
 */
export function buildRoomFollowUps(turn: FollowUpTurnInput): RoomFollowUpSuggestion[] {
  if (followUpNeedsNarrowing(turn)) {
    return [
      { id: "narrow-scope", messageKey: "knowledge.followUp.narrowScope", kind: "narrow", slot: 0 },
      { id: "name-clause", messageKey: "knowledge.followUp.nameClause", kind: "narrow", slot: 1 },
    ];
  }

  const out: RoomFollowUpSuggestion[] = [];
  const seen = new Set<string>();
  const add = (chip: RoomFollowUpSuggestion): boolean => {
    const key = `${chip.messageKey}:${JSON.stringify(chip.params ?? {})}`;
    if (seen.has(key)) return false;
    if (!chip.messageKey) return false;
    seen.add(key);
    out.push({ ...chip, slot: out.length });
    return true;
  };

  const slot0 = splitSlot0(turn);
  if (!slot0 || !add(slot0)) return [];

  for (const extra of splitContinuationExtras(turn, slot0)) {
    if (out.length >= FOLLOW_UP_SLOT_MAX) break;
    if (extra.kind && extra.kind === slot0.kind) continue;
    add(extra);
  }

  if (turn.packItems?.length && out.length < FOLLOW_UP_SLOT_MAX) {
    const corpus = followUpCoverageCorpus(turn);
    const anchor = turnAnchor(turn);
    for (const item of turn.packItems) {
      if (out.length >= FOLLOW_UP_SLOT_MAX) break;
      if (item.covered || packItemCovered(item, corpus)) continue;
      const topic = coverTopic(item);
      if (!anchor || !topic) continue;
      if (anchor.toLowerCase() === topic.toLowerCase()) continue;
      const textProbe = `Given ${anchor}, how do this room’s docs treat ${topic}?`;
      if (!isPromotableDeskFollowUpText(textProbe)) continue;
      if (compactText(item.prompt) === compactText(textProbe)) continue;
      add({
        id: `cover-${item.id}`,
        messageKey: "knowledge.followUp.coverRewrite",
        params: { anchor, topic },
        kind: "cover",
      });
    }
  }

  return out;
}

function splitSlot0(turn: FollowUpTurnInput): RoomFollowUpSuggestion | null {
  for (let i = 0; i < (turn.unresolved ?? []).length; i++) {
    const prompt = (turn.unresolved?.[i] ?? "").trim();
    if (!isPromotableDeskFollowUpText(prompt)) continue;
    return {
      id: `gap-unresolved-${i + 1}`,
      messageKey: "knowledge.followUp.conflictUnresolved",
      params: { prompt: truncateRunes(prompt, 120) },
      kind: "conflict",
    };
  }

  const claim = strongestGroundedClaim(turn);
  if (claim) {
    const preview = truncateRunes(claim.text.trim(), FOLLOW_UP_CLAIM_PREVIEW);
    if (claim.sourceName) {
      return {
        id: "gap-verify-claim",
        messageKey: "knowledge.followUp.verifyClaimInSource",
        params: { sourceName: claim.sourceName, preview },
        kind: "verify",
      };
    }
    return {
      id: "gap-verify-claim",
      messageKey: "knowledge.followUp.verifyClaim",
      params: { preview },
      kind: "verify",
    };
  }

  const sources = coverageSourceNames(turn.hits);
  const answer = turn.answer ?? "";
  if (
    sources.length >= 2 &&
    !(
      answer.toLowerCase().includes(sources[0]!.toLowerCase()) &&
      answer.toLowerCase().includes(sources[1]!.toLowerCase())
    )
  ) {
    return {
      id: "gap-cross-file",
      messageKey: "knowledge.followUp.conflictCrossFile",
      params: { sourceA: sources[0]!, sourceB: sources[1]! },
      kind: "conflict",
    };
  }

  const anchor = turnAnchor(turn);
  if (!anchor) return null;
  return {
    id: "gap-verify-question",
    messageKey: "knowledge.followUp.verifyQuestion",
    params: { anchor },
    kind: "verify",
  };
}

function splitContinuationExtras(
  turn: FollowUpTurnInput,
  slot0: RoomFollowUpSuggestion,
): RoomFollowUpSuggestion[] {
  const extras: RoomFollowUpSuggestion[] = [];
  const sources = coverageSourceNames(turn.hits);
  if (sources.length >= 2 && slot0.id !== "gap-cross-file") {
    extras.push({
      id: "gap-cross-file",
      messageKey: "knowledge.followUp.conflictCrossFile",
      params: { sourceA: sources[0]!, sourceB: sources[1]! },
      kind: "conflict",
    });
  }
  const anchor = turnAnchor(turn);
  const slot0Hay = `${slot0.messageKey} ${JSON.stringify(slot0.params ?? {})}`.toLowerCase();
  if (anchor && !slot0Hay.includes(anchor.toLowerCase())) {
    extras.push({
      id: "gap-consequence",
      messageKey: "knowledge.followUp.consequenceAnchor",
      params: { anchor },
      kind: "consequence",
    });
  }
  return extras;
}

function strongestGroundedClaim(
  turn: FollowUpTurnInput,
): { text: string; sourceName: string } | null {
  const hitName = new Map<string, string>();
  for (const h of turn.hits) {
    const id = (h.chunkId ?? "").trim();
    if (!id) continue;
    hitName.set(id, (h.sourceName ?? "").trim());
  }
  let best: { text: string; sourceName: string } | null = null;
  let bestHits = -1;
  for (const c of turn.claims ?? []) {
    const text = (c.text ?? "").trim();
    if (!text) continue;
    const hitIds = c.hitIds ?? [];
    if (c.confidence !== "grounded" || hitIds.length === 0) continue;
    if (!claimOverlapsQuestion(text, turn.question)) continue;
    if (hitIds.length < bestHits) continue;
    best = {
      text,
      sourceName: hitIds[0] ? (hitName.get(hitIds[0]) ?? "") : "",
    };
    bestHits = hitIds.length;
  }
  return best;
}

function questionTopicGrounded(turn: FollowUpTurnInput): boolean {
  return (turn.claims ?? []).some((c) => {
    if (c.confidence !== "grounded" || (c.hitIds ?? []).length === 0) return false;
    return claimOverlapsQuestion(c.text ?? "", turn.question);
  });
}

function hasActionableUnresolvedTurn(turn: FollowUpTurnInput): boolean {
  return (turn.unresolved ?? []).some((u) => isPromotableDeskFollowUpText(u));
}

function claimOverlapsQuestion(claimText: string, question?: string | null): boolean {
  const qTokens = questionTopicTokens(question ?? "");
  if (!qTokens.length) return false;
  const hay = claimText.toLowerCase();
  return qTokens.some((tok) => hay.includes(tok));
}

function questionTopicTokens(question: string): string[] {
  const weak = new Set(["多少", "什么", "如何", "怎样", "哪个", "是否", "much", "many", "which"]);
  const out: string[] = [];
  const seen = new Set<string>();
  const add = (t: string) => {
    const v = t.trim();
    if (!v || weak.has(v) || seen.has(v)) return;
    seen.add(v);
    out.push(v);
  };
  for (const t of splitTurnAnchorTokens(question.toLowerCase())) {
    add(t);
    const peeled = peelQuestionSuffix(t);
    if (peeled !== t) add(peeled);
  }
  return out;
}

function peelQuestionSuffix(tok: string): string {
  const suffixes = ["是多少", "是什么", "如何", "怎样", "多少", "什么", "吗"];
  for (const s of suffixes) {
    if (!tok.endsWith(s)) continue;
    const rest = tok.slice(0, tok.length - s.length);
    if ([...rest].length >= 2) return rest;
  }
  return tok;
}

function turnAnchor(turn: FollowUpTurnInput): string {
  const corpus = `${turn.question ?? ""} ${turn.answer ?? ""} ${(turn.claims ?? [])
    .map((c) => c.text)
    .join(" ")}`;
  const tokens = splitTurnAnchorTokens(corpus.toLowerCase());
  const qTokens = splitTurnAnchorTokens((turn.question ?? "").toLowerCase());
  const num = tokens.find((tok) => looksAnchorNumberToken(tok)) ?? "";
  const topic = qTokens.find((tok) => !looksAnchorNumberToken(tok)) ?? "";
  if (topic && num) return truncateRunes(`${topic} ${num}`, FOLLOW_UP_ANCHOR_MAX);
  if (num) return truncateRunes(num, FOLLOW_UP_ANCHOR_MAX);
  if (topic) return truncateRunes(topic, FOLLOW_UP_ANCHOR_MAX);
  const q = (turn.question ?? "").trim();
  if (q) return truncateRunes(q, FOLLOW_UP_ANCHOR_MAX);
  return tokens[0] ? truncateRunes(tokens[0], FOLLOW_UP_ANCHOR_MAX) : "";
}

function splitTurnAnchorTokens(corpus: string): string[] {
  const stop = new Set(["the", "and", "for", "with", "this", "that", "from", "are", "was", "how", "what", "does"]);
  const out: string[] = [];
  const seen = new Set<string>();
  let buf = "";
  let cls = 0;
  const flush = () => {
    const f = buf;
    buf = "";
    if (!f || stop.has(f) || seen.has(f)) return;
    const chars = [...f];
    const first = chars[0] ?? "";
    const keep = looksAnchorNumberToken(f)
      ? true
      : /[\u4e00-\u9fff]/.test(first)
        ? chars.length >= 2
        : /[a-z]/.test(first)
          ? chars.length >= 3
          : false;
    if (!keep) return;
    seen.add(f);
    out.push(f);
  };
  for (const r of corpus) {
    const c = /[\u4e00-\u9fff]/.test(r) ? 1 : /[a-z]/.test(r) ? 2 : /[0-9]/.test(r) ? 3 : 0;
    if (c === 0) {
      if (cls === 3 && (r === "." || r === "," || r === "%") && buf) {
        buf += r;
        continue;
      }
      flush();
      cls = 0;
      continue;
    }
    if (buf && c !== cls) flush();
    cls = c;
    buf += r;
  }
  flush();
  return out;
}

function looksAnchorNumberToken(tok: string): boolean {
  if (!tok) return false;
  let digits = 0;
  for (const r of tok) {
    if (r >= "0" && r <= "9") {
      digits++;
      continue;
    }
    if (r === "." || r === "," || r === "%" || r === "+" || r === "-") continue;
    if (r >= "a" && r <= "z") {
      if (digits === 0) return false;
      continue;
    }
    return false;
  }
  return digits > 0;
}

function truncateRunes(s: string, max: number): string {
  const chars = [...s];
  if (chars.length <= max) return s;
  return chars.slice(0, max).join("");
}

function followUpCoverageCorpus(turn: FollowUpTurnInput): string {
  return [
    turn.question ?? "",
    turn.answer ?? "",
    ...(turn.unresolved ?? []),
    ...(turn.claims ?? []).map((c) => c.text),
  ]
    .join(" ")
    .toLowerCase();
}

function packItemCovered(item: FollowUpPackItem, corpus: string): boolean {
  const prompt = (item.prompt ?? "").trim().toLowerCase();
  if (prompt.length >= 8 && corpus.includes(prompt)) return true;
  const kws = (item.keywords ?? []).map((k) => k.trim().toLowerCase()).filter(Boolean);
  let hits = 0;
  let strong = false;
  for (const kw of kws) {
    if (!corpus.includes(kw)) continue;
    hits++;
    if (/[\u4e00-\u9fff]/.test(kw) ? [...kw].length >= 2 : kw.length >= 5) strong = true;
  }
  if (hits === 0) return false;
  if (strong) return true;
  return hits >= (kws.length === 1 ? 1 : 2);
}

function coverTopic(item: FollowUpPackItem): string {
  for (const kw of item.keywords ?? []) {
    const t = kw.trim();
    if (!t) continue;
    if (/[\u4e00-\u9fff]/.test(t) || t.length >= 4) return t;
  }
  return "";
}

function compactText(s: string): string {
  return s.trim().toLowerCase().split(/\s+/).join(" ");
}
