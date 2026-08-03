import type { DealRoomKnowledgeFeedbackKind } from "@/types";

/** Keep in sync with API FeedbackKind* and migration CHECK. */
export const FEEDBACK_KINDS = [
  "helpful",
  "wrong_citation",
  "not_answering",
] as const satisfies readonly DealRoomKnowledgeFeedbackKind[];

export function isFeedbackKind(value: string | undefined | null): value is DealRoomKnowledgeFeedbackKind {
  if (!value) return false;
  return (FEEDBACK_KINDS as readonly string[]).includes(value);
}

export function asFeedbackKind(
  value: string | undefined | null,
): DealRoomKnowledgeFeedbackKind | undefined {
  return isFeedbackKind(value) ? value : undefined;
}
