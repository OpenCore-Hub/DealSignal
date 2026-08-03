import { asFeedbackKind } from "@/lib/knowledge/feedback";
import type {
  DealRoomKnowledgeQueryHit,
  DealRoomKnowledgeQATurn,
  DealRoomKnowledgeTurnFeedback,
} from "@/types";

function mapTurnFeedback(
  feedback: DealRoomKnowledgeQATurn["feedback"],
): DealRoomKnowledgeTurnFeedback | undefined {
  const kind = asFeedbackKind(feedback?.kind);
  if (!kind) return undefined;
  return { kind, note: feedback?.note };
}

/**
 * Whitelisted stream events for Deal Room grounded chat.
 * Client renders only these — never arbitrary UI trees (A2UI spirit, not protocol).
 */
export type KnowledgeStreamPhase =
  | "idle"
  | "retrieving"
  | "generating"
  | "done"
  | "refused"
  | "error";

export type KnowledgeResultStatus =
  | "answered"
  | "refused"
  | "no_hits"
  | "error"
  | string;

export type KnowledgeStreamEvent =
  | { type: "phase"; phase: Extract<KnowledgeStreamPhase, "retrieving" | "generating"> }
  | { type: "sources"; results: DealRoomKnowledgeQueryHit[]; grounded: boolean }
  | { type: "token"; text: string }
  | {
      type: "done";
      answer?: string;
      results?: DealRoomKnowledgeQueryHit[];
      refused?: boolean;
      resultStatus?: KnowledgeResultStatus;
    }
  | { type: "error"; message: string };

/** One user question → one grounded judgment (research-desk turn, not chat persona). */
export interface KnowledgeTurn {
  id: string;
  query: string;
  phase: KnowledgeStreamPhase;
  answer: string;
  results: DealRoomKnowledgeQueryHit[];
  /** True when answer is an ungrounded refusal; evidence rail must stay hidden. */
  refused: boolean;
  /** Server audit status — drives follow-up templates (incl. no_hits). */
  resultStatus: KnowledgeResultStatus;
  activeCite: number | null;
  errorMessage?: string;
  /** Current user's Phase C feedback, if any. */
  feedback?: DealRoomKnowledgeTurnFeedback;
}

let turnSeq = 0;

export function createKnowledgeTurn(query: string, id?: string): KnowledgeTurn {
  turnSeq += 1;
  return {
    id: id ?? `kt-${turnSeq}-${Date.now()}`,
    query,
    phase: "retrieving",
    answer: "",
    results: [],
    refused: false,
    resultStatus: "answered",
    activeCite: null,
  };
}

/**
 * Reduce a stream event into turn state.
 * Product rule: never attach evidence unless grounded; refuse clears hits.
 */
export function reduceKnowledgeStream(
  turn: KnowledgeTurn,
  event: KnowledgeStreamEvent,
): KnowledgeTurn {
  switch (event.type) {
    case "phase":
      return { ...turn, phase: event.phase, errorMessage: undefined };
    case "sources":
      if (!event.grounded || turn.refused) {
        return { ...turn, results: [] };
      }
      return { ...turn, results: event.results };
    case "token":
      return {
        ...turn,
        phase: turn.phase === "retrieving" ? "generating" : turn.phase,
        answer: turn.answer + event.text,
      };
    case "done": {
      const refused = !!event.refused;
      const answer = event.answer ?? turn.answer;
      const results =
        refused || event.results === undefined
          ? refused
            ? []
            : turn.results
          : event.results;
      const resultStatus =
        event.resultStatus ??
        (refused ? "refused" : results.length === 0 ? "no_hits" : "answered");
      return {
        ...turn,
        phase: refused ? "refused" : "done",
        refused,
        resultStatus,
        answer,
        results: refused ? [] : results,
      };
    }
    case "error":
      return {
        ...turn,
        phase: "error",
        resultStatus: "error",
        errorMessage: event.message,
      };
    default:
      return turn;
  }
}

/** Whether the evidence rail should mount (P3 + P4). */
export function shouldShowEvidence(turn: KnowledgeTurn): boolean {
  return !turn.refused && turn.results.length > 0;
}

/**
 * Adapt today's non-streaming query API into a terminal turn.
 * Lets the shell ship before SSE exists.
 */
export function turnFromQueryResult(
  query: string,
  result: {
    answer?: string;
    results: DealRoomKnowledgeQueryHit[];
  },
  opts: { refused: boolean },
): KnowledgeTurn {
  const base = createKnowledgeTurn(query);
  return reduceKnowledgeStream(base, {
    type: "done",
    answer: result.answer ?? "",
    results: result.results,
    refused: opts.refused,
  });
}

/** Map a persisted audit turn into the research-desk view model. */
export function turnFromQATurn(
  row: DealRoomKnowledgeQATurn,
  activeCite: number | null = null,
): KnowledgeTurn {
  const status = (row.resultStatus || "").trim() || "answered";

  if (status === "error") {
    return {
      id: row.id,
      query: row.question,
      phase: "error",
      answer: "",
      results: [],
      refused: false,
      resultStatus: "error",
      activeCite: null,
      errorMessage: row.errorSummary,
      feedback: mapTurnFeedback(row.feedback),
    };
  }

  const refused = row.refused || status === "refused";
  if (refused) {
    return {
      id: row.id,
      query: row.question,
      phase: "refused",
      answer: row.answer ?? "",
      results: [],
      refused: true,
      resultStatus: "refused",
      activeCite: null,
      feedback: mapTurnFeedback(row.feedback),
    };
  }

  if (status === "no_hits") {
    return {
      id: row.id,
      query: row.question,
      phase: "done",
      answer: row.answer ?? "",
      results: [],
      refused: false,
      resultStatus: "no_hits",
      activeCite: null,
      feedback: mapTurnFeedback(row.feedback),
    };
  }

  return {
    id: row.id,
    query: row.question,
    phase: "done",
    answer: row.answer ?? "",
    results: row.hits ?? [],
    refused: false,
    resultStatus: status,
    activeCite,
    feedback: mapTurnFeedback(row.feedback),
  };
}
