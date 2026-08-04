import type { KnowledgeTurn } from "@/lib/knowledge/streamEvents";
import type { DealRoomKnowledgeQATurn } from "@/types";

export type CiteOpenOutcome = "grounded" | "refused" | "unknown";

/**
 * Desk cite_open metric outcome from live stream turn or last audit turn
 * (ceiling Phase V — shared Tab + Viewer rail).
 */
export function resolveCiteOpenOutcome(
  liveTurn: KnowledgeTurn | null | undefined,
  turns: DealRoomKnowledgeQATurn[],
): CiteOpenOutcome {
  if (liveTurn) {
    if (liveTurn.refused) return "refused";
    return liveTurn.results.length > 0 ? "grounded" : "unknown";
  }
  const last = turns[turns.length - 1];
  if (last?.refused) return "refused";
  if ((last?.hits?.length ?? 0) > 0) return "grounded";
  return "unknown";
}
