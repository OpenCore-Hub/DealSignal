import type { OwnerAskTurn, VisitorQuestion } from "@/types";
import { api } from "@/lib/api";

/** Map owner inbox turn to legacy VisitorQuestion for answer APIs. */
export function ownerAskTurnToVisitorQuestion(turn: OwnerAskTurn): VisitorQuestion {
  const id = turn.host_question_id?.trim() || turn.id;
  return {
    id,
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

/** Prefer unified PATCH .../ask/:turnId/host-answer when turn id is known. */
export async function answerOwnerAskQuestion(
  question: VisitorQuestion,
  answer: string,
): Promise<VisitorQuestion> {
  if (question.ask_turn_id) {
    const res = await api.answerAskTurn(question.link_id, question.ask_turn_id, answer);
    return ownerAskTurnToVisitorQuestion(res.data);
  }
  const res = await api.answerQuestion(question.link_id, question.id, answer);
  return res.data;
}
