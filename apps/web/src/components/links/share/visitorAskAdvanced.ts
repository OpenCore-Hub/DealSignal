import type { DraftLink } from "./types";

/** Other advanced features that still count one-for-one. */
export const STANDALONE_ADVANCED_KEYS = [
  "enableFileRequests",
  "enableIndexFileGeneration",
] as const satisfies ReadonlyArray<keyof DraftLink>;

/**
 * Advanced enabled count: Visitor Ask (Ask Docs ∨ Ask Host) counts as one
 * capability, plus each remaining advanced toggle.
 */
export function countAdvancedEnabled(draft: Pick<
  DraftLink,
  "aiCopilotEnabled" | "enableQaConversations" | (typeof STANDALONE_ADVANCED_KEYS)[number]
>): number {
  let count = 0;
  if (draft.aiCopilotEnabled || draft.enableQaConversations) {
    count += 1;
  }
  for (const key of STANDALONE_ADVANCED_KEYS) {
    if (draft[key]) count += 1;
  }
  return count;
}

export function visitorAskMasterEnabled(draft: Pick<DraftLink, "aiCopilotEnabled" | "enableQaConversations">): boolean {
  return Boolean(draft.aiCopilotEnabled || draft.enableQaConversations);
}

/** Turning the master off clears both channels; turning on defaults to Ask Docs. */
export function visitorAskMasterPatch(enabled: boolean): Partial<DraftLink> {
  if (!enabled) {
    return { aiCopilotEnabled: false, enableQaConversations: false };
  }
  return { aiCopilotEnabled: true, enableQaConversations: false };
}
