import type { DealRoomKnowledgeQueryHit } from "@/types";
import { isUngroundedKnowledgeAnswer } from "@/lib/knowledge/trustGates";

/** Locale-ready follow-up chip — render with t(messageKey, params). */
export interface RoomFollowUpSuggestion {
  id: string;
  messageKey: string;
  params?: Record<string, string>;
}

export interface FollowUpTurnInput {
  refused: boolean;
  resultStatus: string;
  hits: Pick<DealRoomKnowledgeQueryHit, "sourceName">[];
  /** Optional answer text — mirrors BE needsFollowUpNarrowing soft-refusal probe. */
  answer?: string | null;
}

/** Max distinct files in the follow-up coverage set (ceiling §3.1). */
export const FOLLOW_UP_COVERAGE_MAX = 3;

/** Narrowing prompts for refuse / no_hits / error — shared by templates. */
export function followUpNeedsNarrowing(turn: FollowUpTurnInput): boolean {
  if (turn.refused || isUngroundedKnowledgeAnswer(turn.answer)) return true;
  switch (turn.resultStatus) {
    case "refused":
    case "no_hits":
    case "error":
      return true;
    default:
      return false;
  }
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
 * Local template fallback for suggested follow-ups (no LLM).
 * Production path: FE shows these immediately, then upgrades via
 * POST …/turns/:turnId/follow-ups when source=llm.
 * Multi-file hits use a coverage set (top-1 + top-2 + cross-file).
 */
export function buildRoomFollowUps(turn: FollowUpTurnInput): RoomFollowUpSuggestion[] {
  if (followUpNeedsNarrowing(turn)) {
    return [
      { id: "narrow-scope", messageKey: "knowledge.followUp.narrowScope" },
      { id: "name-clause", messageKey: "knowledge.followUp.nameClause" },
    ];
  }

  const sources = coverageSourceNames(turn.hits);
  if (sources.length === 0) {
    return [
      { id: "specific-clause", messageKey: "knowledge.followUp.specificClause" },
      { id: "party-obligations", messageKey: "knowledge.followUp.partyObligations" },
    ];
  }

  const top1 = sources[0]!;
  if (sources.length === 1) {
    return [
      {
        id: "liability-in-source",
        messageKey: "knowledge.followUp.liabilityInSource",
        params: { sourceName: top1 },
      },
      {
        id: "definitions-in-source",
        messageKey: "knowledge.followUp.definitionsInSource",
        params: { sourceName: top1 },
      },
      {
        id: "exceptions-in-source",
        messageKey: "knowledge.followUp.exceptionsInSource",
        params: { sourceName: top1 },
      },
    ];
  }

  const top2 = sources[1]!;
  return [
    {
      id: "liability-in-source",
      messageKey: "knowledge.followUp.liabilityInSource",
      params: { sourceName: top1 },
    },
    {
      id: "exceptions-in-second-source",
      messageKey: "knowledge.followUp.exceptionsInSecondSource",
      params: { sourceName: top2 },
    },
    {
      id: "cross-file-consistency",
      messageKey: "knowledge.followUp.crossFileConsistency",
      params: { sourceA: top1, sourceB: top2 },
    },
  ];
}
