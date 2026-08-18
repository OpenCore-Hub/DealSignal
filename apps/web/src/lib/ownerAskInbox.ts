import type { OwnerAskTurn } from "@/types";

/** Owner Ask Inbox lane tabs (Phase B/C). */
export type OwnerAskInboxView = "all" | "needs_host" | "ai_handled" | "pinned_faq" | "formal_queue";

const ownerAskInboxViews = new Set<OwnerAskInboxView>([
  "all",
  "needs_host",
  "ai_handled",
  "pinned_faq",
  "formal_queue",
]);

/** Parse ?askInbox= deep links from dashboard or bookmarks. */
export function parseOwnerAskInboxView(raw: string | null | undefined): OwnerAskInboxView | undefined {
  if (!raw) return undefined;
  return ownerAskInboxViews.has(raw as OwnerAskInboxView)
    ? (raw as OwnerAskInboxView)
    : undefined;
}

export function ownerAskInboxQuery(view: OwnerAskInboxView): {
  lane?: string;
  status?: string;
} {
  switch (view) {
    case "needs_host":
      return { lane: "host", status: "host_pending" };
    case "ai_handled":
      return { lane: "ai", status: "ai_answered" };
    case "formal_queue":
      return { status: "formal_queue" };
    default:
      return {};
  }
}

export function ownerAskInboxUsesPinnedFAQApi(view: OwnerAskInboxView): boolean {
  return view === "pinned_faq";
}

export function isOwnerAskNeedsHostStatus(status: OwnerAskTurn["status"]): boolean {
  return status === "host_pending" || status === "host_escalated";
}

/** Pending host attention across a room or link (nav badges, metrics). */
export function countUnreadOwnerAskTurns(turns: OwnerAskTurn[]): number {
  return turns.filter((turn) => isOwnerAskNeedsHostStatus(turn.status)).length;
}

/** Mirrors backend inbox filter semantics for MSW and tests. */
export function matchesOwnerAskInboxFilter(
  turn: OwnerAskTurn,
  lane?: string | null,
  status?: string | null,
): boolean {
  const l = lane ?? "";
  const s = status ?? "";
  if (!l && !s) return true;
  if (s === "formal_queue") {
    return ownerAskTurnIsFormalQueueActive(turn);
  }
  if (turn.route_reason === "pinned_faq" && (l || s)) {
    return false;
  }
  if (s === "host_pending" && l === "host") {
    if (ownerAskTurnIsFormalQueue(turn)) return false;
    return isOwnerAskNeedsHostStatus(turn.status) && (turn.lane === "host" || turn.lane === "hybrid");
  }
  if (s === "ai_answered" && l === "ai") {
    return turn.lane === "ai" && turn.status === "ai_answered";
  }
  if (l && turn.lane !== l) return false;
  if (s && turn.status !== s) return false;
  return true;
}

export function ownerAskTurnNeedsHostReply(turn: OwnerAskTurn): boolean {
  if (ownerAskTurnIsFormalQueue(turn)) return false;
  return isOwnerAskNeedsHostStatus(turn.status) && (turn.lane === "host" || turn.lane === "hybrid");
}

export function ownerAskTurnIsFormalQueue(turn: OwnerAskTurn): boolean {
  return (
    turn.route_reason === "policy_formal" ||
    turn.formal_status === "pending_review" ||
    turn.formal_status === "scheduled"
  );
}

/** Host-lane fallback when Formal mode is configured but plan is not entitled. */
export function ownerAskTurnIsFormalDegraded(turn: OwnerAskTurn): boolean {
  return turn.route_reason === "formal_not_entitled";
}

/** Badge / nav attention: needs-host replies + active Formal review queue. */
export function countOwnerAskPendingAttention(
  needsHostTurns: OwnerAskTurn[],
  formalQueueTurns: OwnerAskTurn[],
): number {
  return (
    needsHostTurns.filter((turn) => ownerAskTurnNeedsHostReply(turn)).length +
    formalQueueTurns.filter((turn) => ownerAskTurnNeedsFormalPublish(turn)).length
  );
}

/** Mirrors backend isFormalQueueActive — pending/scheduled only, not published. */
export function ownerAskTurnNeedsFormalPublish(turn: OwnerAskTurn): boolean {
  return turn.formal_status === "pending_review" || turn.formal_status === "scheduled";
}

export function ownerAskTurnIsFormalQueueActive(turn: OwnerAskTurn): boolean {
  return ownerAskTurnIsFormalQueue(turn) && ownerAskTurnNeedsFormalPublish(turn);
}

export function ownerAskTurnCanPinFAQ(turn: OwnerAskTurn): boolean {
  if (turn.pinned_faq_at) return false;
  const hostAnswer = turn.host_answer?.trim();
  const aiAnswer = turn.ai_payload?.answer?.trim();
  const refused =
    Boolean(turn.ai_payload?.refused) || turn.ai_payload?.resultStatus === "refused";
  if (turn.status === "ai_answered") {
    return Boolean(aiAnswer) && !refused;
  }
  if (turn.status === "host_answered") {
    if (hostAnswer) return true;
    return Boolean(aiAnswer) && !refused;
  }
  return false;
}

export function ownerAskTurnCanUnpinFAQ(turn: OwnerAskTurn): boolean {
  return Boolean(turn.pinned_faq_at);
}

/** Normalize visitor question text for same-link repeat detection. */
export function normalizeAskQuestionKey(question: string): string {
  return question
    .trim()
    .toLowerCase()
    .replace(/[^\p{L}\p{N}\s]+/gu, " ")
    .replace(/\s+/g, " ")
    .trim();
}

export const OWNER_ASK_REPEAT_PIN_THRESHOLD = 3;

export function countSimilarAskQuestions(
  turns: OwnerAskTurn[],
  turn: OwnerAskTurn,
): number {
  const key = normalizeAskQuestionKey(turn.question);
  if (!key) return 0;
  return turns.filter(
    (t) => t.link_id === turn.link_id && normalizeAskQuestionKey(t.question) === key,
  ).length;
}

export function attachOwnerAskRepeatCounts(turns: OwnerAskTurn[]): OwnerAskTurn[] {
  const counts = new Map<string, number>();
  for (const turn of turns) {
    const qKey = normalizeAskQuestionKey(turn.question);
    if (!qKey) continue;
    const key = `${turn.link_id}:${qKey}`;
    counts.set(key, (counts.get(key) ?? 0) + 1);
  }
  return turns.map((turn) => {
    const qKey = normalizeAskQuestionKey(turn.question);
    return {
      ...turn,
      repeat_count: qKey ? (counts.get(`${turn.link_id}:${qKey}`) ?? 0) : 0,
    };
  });
}

/** Suggest pinning when the same question repeats within a link. */
export function ownerAskTurnSuggestPinFAQ(
  turn: OwnerAskTurn,
  threshold = OWNER_ASK_REPEAT_PIN_THRESHOLD,
): boolean {
  if (!ownerAskTurnCanPinFAQ(turn)) return false;
  return (turn.repeat_count ?? 0) >= threshold;
}

export function sortOwnerAskPinnedFAQs(turns: OwnerAskTurn[]): OwnerAskTurn[] {
  return [...turns].sort((a, b) => {
    const aSort = a.pinned_faq_sort ?? Number.MAX_SAFE_INTEGER;
    const bSort = b.pinned_faq_sort ?? Number.MAX_SAFE_INTEGER;
    if (aSort !== bSort) return aSort - bSort;
    const aPinned = a.pinned_faq_at ? new Date(a.pinned_faq_at).getTime() : 0;
    const bPinned = b.pinned_faq_at ? new Date(b.pinned_faq_at).getTime() : 0;
    return bPinned - aPinned;
  });
}

export function moveOwnerAskPinnedFAQ(
  turns: OwnerAskTurn[],
  turnId: string,
  direction: "up" | "down",
): OwnerAskTurn[] {
  const sorted = sortOwnerAskPinnedFAQs(turns);
  const index = sorted.findIndex((turn) => turn.id === turnId);
  if (index < 0) return sorted;
  const target = direction === "up" ? index - 1 : index + 1;
  if (target < 0 || target >= sorted.length) return sorted;
  const next = [...sorted];
  [next[index], next[target]] = [next[target], next[index]];
  return next.map((turn, idx) => ({ ...turn, pinned_faq_sort: idx }));
}

export function ownerAskFaqReorderEnabled(
  view: OwnerAskInboxView,
  scope: { type: "room" | "link"; linkFilter?: string },
): boolean {
  if (view !== "pinned_faq") return false;
  if (scope.type === "link") return true;
  return Boolean(scope.linkFilter && scope.linkFilter !== "all");
}

export function ownerAskTurnHasAIPreview(turn: OwnerAskTurn): boolean {
  return (turn.lane === "ai" || turn.lane === "hybrid") && Boolean(turn.ai_payload?.answer);
}

export function ownerAskTurnStatusBadgeVariant(
  turn: OwnerAskTurn,
): "default" | "warm" | "secondary" | "outline" {
  if (turn.lane === "ai" || turn.lane === "hybrid") {
    if (turn.status === "ai_answered" || turn.status === "host_answered") return "default";
    if (turn.status === "ai_refused" || turn.status === "failed") return "outline";
    return "secondary";
  }
  if (turn.status === "host_answered") return "default";
  return "warm";
}
