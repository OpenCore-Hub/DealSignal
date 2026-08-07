import type { OwnerAskTurn, VisitorQuestion } from "@/types";
import { api } from "@/lib/api";

/** Map owner inbox turn for reply UI (turn id is canonical). */
export function ownerAskTurnToVisitorQuestion(turn: OwnerAskTurn): VisitorQuestion {
  return {
    id: turn.id,
    ask_turn_id: turn.id,
    link_id: turn.link_id,
    visitor_id: turn.visitor_id,
    visitor_email: turn.visitor_email,
    question: turn.question,
    answer: turn.host_answer,
    status: turn.status === "host_answered" ? "answered" : "pending",
    created_at: turn.created_at,
    updated_at: turn.updated_at,
  };
}

export function ownerAskTurnsToVisitorQuestions(turns: OwnerAskTurn[]): VisitorQuestion[] {
  return turns.map(ownerAskTurnToVisitorQuestion);
}

/** Reply via unified PATCH .../ask/:turnId/host-answer. */
export async function answerOwnerAskQuestion(
  question: VisitorQuestion,
  answer: string,
): Promise<VisitorQuestion> {
  const turnId = question.ask_turn_id?.trim() || question.id;
  const res = await api.answerAskTurn(question.link_id, turnId, answer);
  return ownerAskTurnToVisitorQuestion(res.data);
}
