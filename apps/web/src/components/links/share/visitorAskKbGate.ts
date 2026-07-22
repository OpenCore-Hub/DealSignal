import type { DealRoomKnowledgeBaseStatus } from "@/types";

/** Q4: Ask Docs may stay on for ready or soft-stale room KBs. */
export function isAskDocsKnowledgeBaseReady(
  status: DealRoomKnowledgeBaseStatus | null | undefined,
): boolean {
  return status === "ready" || status === "stale";
}

/**
 * When true, AccessTab should disable enabling Ask Docs and show a create/rebuild guide.
 * Non-deal-room links and unknown status do not block (save-time gate still applies).
 */
export function shouldBlockAskDocsForKnowledgeBase(
  isDealRoomLink: boolean,
  status: DealRoomKnowledgeBaseStatus | null | undefined,
): boolean {
  if (!isDealRoomLink) return false;
  if (status == null) return false;
  return !isAskDocsKnowledgeBaseReady(status);
}
