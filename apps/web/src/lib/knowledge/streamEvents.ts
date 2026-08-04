import { asFeedbackKind } from "@/lib/knowledge/feedback";
import type {
  DealRoomKnowledgeAnswerClaim,
  DealRoomKnowledgeHitConflict,
  DealRoomKnowledgeJudgment,
  DealRoomKnowledgeMultiHop,
  DealRoomKnowledgeQueryHit,
  DealRoomKnowledgeQATurn,
  DealRoomKnowledgeRefusal,
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
  | {
      type: "phase";
      phase: Extract<KnowledgeStreamPhase, "retrieving" | "generating">;
      /** Present on generating when the server rewrote the retrieve query. */
      retrieveQuery?: string;
      rewriteApplied?: boolean;
    }
  | { type: "sources"; results: DealRoomKnowledgeQueryHit[]; grounded: boolean }
  | { type: "token"; text: string }
  | {
      type: "done";
      answer?: string;
      results?: DealRoomKnowledgeQueryHit[];
      refused?: boolean;
      resultStatus?: KnowledgeResultStatus;
      retrieveQuery?: string;
      rewriteApplied?: boolean;
      claims?: DealRoomKnowledgeAnswerClaim[];
      unresolved?: string[];
      conflicts?: DealRoomKnowledgeHitConflict[];
      multiHop?: DealRoomKnowledgeMultiHop;
      refusal?: DealRoomKnowledgeRefusal;
      judgment?: DealRoomKnowledgeJudgment;
    }
  | { type: "error"; message: string };

function applyRewriteAudit(
  turn: KnowledgeTurn,
  patch: { retrieveQuery?: string; rewriteApplied?: boolean },
): Pick<KnowledgeTurn, "retrieveQuery" | "rewriteApplied"> {
  if (!patch.rewriteApplied) return {};
  const retrieveQuery = (patch.retrieveQuery || "").trim();
  if (!retrieveQuery) return { rewriteApplied: true };
  return { retrieveQuery, rewriteApplied: true };
}

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
  /** Standalone retrieve query when the server rewrote an elliptical ask. */
  retrieveQuery?: string;
  rewriteApplied?: boolean;
  /** Sentence↔hit binding when the server produced a bound answer. */
  claims?: DealRoomKnowledgeAnswerClaim[];
  unresolved?: string[];
  /** Cross-file conflicts — both sides listed, no pick (Phase I). */
  conflicts?: DealRoomKnowledgeHitConflict[];
  /** Second-hop retrieve audit (Phase I3). */
  multiHop?: DealRoomKnowledgeMultiHop;
  /** Typed L2 refusal / gap (Phase J). */
  refusal?: DealRoomKnowledgeRefusal;
  /** Stamp quality for answered turns (Phase K). */
  judgment?: DealRoomKnowledgeJudgment;
}

/** Muted “searched as …” disclosure under the user question, or null when absent. */
export function turnRetrieveDisclosure(turn: KnowledgeTurn): string | null {
  if (!turn.rewriteApplied) return null;
  const retrieve = (turn.retrieveQuery || "").trim();
  if (!retrieve) return null;
  if (retrieve === turn.query.trim()) return null;
  return retrieve;
}

function rewriteAuditFromRow(row: DealRoomKnowledgeQATurn): Pick<
  KnowledgeTurn,
  "retrieveQuery" | "rewriteApplied"
> {
  if (!row.rewriteApplied) return {};
  const retrieveQuery = (row.retrieveQuery || "").trim();
  if (!retrieveQuery) return { rewriteApplied: true };
  return { retrieveQuery, rewriteApplied: true };
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
      return {
        ...turn,
        phase: event.phase,
        errorMessage: undefined,
        ...applyRewriteAudit(turn, event),
      };
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
      let results =
        refused || event.results === undefined
          ? refused
            ? []
            : turn.results
          : event.results;
      const resultStatus =
        event.resultStatus ??
        (refused ? "refused" : results.length === 0 ? "no_hits" : "answered");
      // P4: refuse / typed no_hits / error never mount an evidence rail.
      if (
        refused ||
        resultStatus === "no_hits" ||
        resultStatus === "refused" ||
        resultStatus === "error"
      ) {
        results = [];
      }
      const claims = refused ? undefined : (event.claims ?? turn.claims);
      const unresolved = refused
        ? undefined
        : (event.unresolved ?? turn.unresolved);
      const conflicts = refused
        ? undefined
        : (event.conflicts ?? turn.conflicts);
      const multiHop = refused
        ? undefined
        : (event.multiHop ?? turn.multiHop);
      const refusal = event.refusal ?? turn.refusal;
      const judgment = refused
        ? undefined
        : (event.judgment ?? turn.judgment);
      return {
        ...turn,
        phase: refused ? "refused" : "done",
        refused,
        resultStatus,
        answer,
        results,
        claims: refused ? undefined : claims,
        unresolved: refused ? undefined : unresolved,
        conflicts: refused ? undefined : conflicts,
        multiHop: refused ? undefined : multiHop,
        refusal,
        judgment: refused ? undefined : judgment,
        ...applyRewriteAudit(turn, event),
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
  if (turn.refused || turn.results.length === 0) return false;
  const status = (turn.resultStatus || "").trim();
  if (status === "no_hits" || status === "refused" || status === "error") {
    return false;
  }
  // Typed refusal envelope (Phase J) — hide low-score comfort hits (philosophy P4).
  const kind = (turn.refusal?.kind || "").trim();
  if (kind === "no_hits" || kind === "ungrounded" || kind === "error") {
    return false;
  }
  return true;
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

  const rewrite = rewriteAuditFromRow(row);

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
      refusal: row.refusal ?? { kind: "error" },
      ...rewrite,
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
      refusal: row.refusal ?? { kind: "ungrounded" },
      ...rewrite,
    };
  }

  if (status === "no_hits") {
    // P4: never mount evidence for typed no_hits (hadHits stays on refusal audit).
    const hadHits = (row.hits?.length ?? 0) > 0 || Boolean(row.refusal?.hadHits);
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
      refusal: row.refusal ?? {
        kind: "no_hits",
        hadHits,
        hitCount: row.refusal?.hitCount ?? row.hits?.length ?? 0,
      },
      ...rewrite,
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
    claims: row.claims,
    unresolved: row.unresolved,
    conflicts: row.conflicts,
    multiHop: row.multiHop,
    refusal: row.refusal,
    judgment: row.judgment,
    ...rewrite,
  };
}
