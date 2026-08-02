import type { DraftLink } from "./types";

/** Other advanced features that still count one-for-one. */
export const STANDALONE_ADVANCED_KEYS = [
  "enableFileRequests",
  "enableIndexFileGeneration",
] as const satisfies ReadonlyArray<keyof DraftLink>;

/** Advanced enabled count: Visitor Ask (Ask Host) counts as one capability, plus each remaining advanced toggle. */
export function countAdvancedEnabled(draft: Pick<
  DraftLink,
  "enableQaConversations" | (typeof STANDALONE_ADVANCED_KEYS)[number]
>): number {
  let count = 0;
  if (draft.enableQaConversations) {
    count += 1;
  }
  for (const key of STANDALONE_ADVANCED_KEYS) {
    if (draft[key]) count += 1;
  }
  return count;
}

export function visitorAskMasterEnabled(draft: Pick<DraftLink, "enableQaConversations">): boolean {
  return Boolean(draft.enableQaConversations);
}

export function visitorAskMasterPatch(enabled: boolean): Partial<DraftLink> {
  return { enableQaConversations: enabled };
}
