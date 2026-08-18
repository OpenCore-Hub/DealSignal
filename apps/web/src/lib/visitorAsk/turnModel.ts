import {
  createKnowledgeTurn,
  type KnowledgeTurn,
} from "@/lib/knowledge/streamEvents";
import type { PublicAskTurn } from "@/types";

export function isAITurn(turn: PublicAskTurn): boolean {
  return turn.lane === "ai" || (turn.lane === "hybrid" && Boolean(turn.ai_payload));
}

export function isHostQueuedTurn(turn: PublicAskTurn): boolean {
  return turn.lane === "host" || turn.lane === "hybrid";
}

export function isAwaitingHostReply(turn: PublicAskTurn): boolean {
  return (
    isHostQueuedTurn(turn) &&
    (turn.status === "host_pending" || turn.status === "host_escalated")
  );
}

export function isPinnedFAQReplayTurn(turn: PublicAskTurn): boolean {
  return turn.route_reason === "pinned_faq";
}

export function turnNeedsAIStream(turn: PublicAskTurn): boolean {
  if (isPinnedFAQReplayTurn(turn)) return false;
  return turn.lane === "ai" && (turn.status === "ai_streaming" || turn.status === "routing");
}

export function publicAskTurnToKnowledgeTurn(turn: PublicAskTurn): KnowledgeTurn | null {
  if (!isAITurn(turn)) return null;
  if (turnNeedsAIStream(turn)) {
    return createKnowledgeTurn(turn.question, turn.id);
  }
  const payload = turn.ai_payload;
  if (!payload) return null;
  const refused =
    payload.refused ||
    turn.status === "ai_refused" ||
    payload.resultStatus === "refused";
  const phase =
    turn.status === "failed" || payload.resultStatus === "error"
      ? "error"
      : refused
        ? "refused"
        : "done";
  return {
    id: turn.id,
    query: turn.question,
    phase,
    answer: payload.answer ?? "",
    results: payload.hits ?? [],
    refused,
    resultStatus: payload.resultStatus ?? (refused ? "refused" : "answered"),
    activeCite: null,
  };
}

export function isAskQuotaExceededTurn(turn: PublicAskTurn): boolean {
  return turn.route_reason === "ai_quota_exceeded";
}

export function isFormalUnderReview(turn: PublicAskTurn): boolean {
  return (
    turn.formal_status === "pending_review" ||
    turn.formal_status === "scheduled" ||
    (turn.route_reason === "policy_formal" &&
      turn.status === "host_pending" &&
      !turn.host_answer?.trim())
  );
}
