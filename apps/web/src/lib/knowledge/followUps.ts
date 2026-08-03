import type { DealRoomKnowledgeQueryHit } from "@/types";

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
}

/** Narrowing prompts for refuse / no_hits / error — shared by templates. */
export function followUpNeedsNarrowing(turn: FollowUpTurnInput): boolean {
  if (turn.refused) return true;
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
 * V1 room-scoped follow-up templates (no LLM).
 * Only suggests questions that stay inside the deal-room corpus.
 */
export function buildRoomFollowUps(turn: FollowUpTurnInput): RoomFollowUpSuggestion[] {
  const sourceName =
    turn.hits.map((h) => (h.sourceName ?? "").trim()).find((n) => n.length > 0) ?? "";

  if (followUpNeedsNarrowing(turn)) {
    return [
      { id: "narrow-scope", messageKey: "knowledge.followUp.narrowScope" },
      { id: "name-clause", messageKey: "knowledge.followUp.nameClause" },
    ];
  }

  if (sourceName) {
    return [
      {
        id: "liability-in-source",
        messageKey: "knowledge.followUp.liabilityInSource",
        params: { sourceName },
      },
      {
        id: "definitions-in-source",
        messageKey: "knowledge.followUp.definitionsInSource",
        params: { sourceName },
      },
      {
        id: "exceptions-in-source",
        messageKey: "knowledge.followUp.exceptionsInSource",
        params: { sourceName },
      },
    ].slice(0, 3);
  }

  return [
    { id: "specific-clause", messageKey: "knowledge.followUp.specificClause" },
    { id: "party-obligations", messageKey: "knowledge.followUp.partyObligations" },
  ];
}
