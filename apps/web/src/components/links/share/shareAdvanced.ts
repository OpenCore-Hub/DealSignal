import type { DraftLink } from "./types";

/** Advanced share-link capabilities (optional toggles in Access tab). */
export const STANDALONE_ADVANCED_KEYS = [
  "enableFileRequests",
  "enableIndexFileGeneration",
] as const satisfies ReadonlyArray<keyof DraftLink>;

export function countAdvancedEnabled(
  draft: Pick<DraftLink, (typeof STANDALONE_ADVANCED_KEYS)[number] | "enableAiAssistant">,
  options?: { countAiAssistant?: boolean },
): number {
  let count = 0;
  for (const key of STANDALONE_ADVANCED_KEYS) {
    if (draft[key]) count += 1;
  }
  if (options?.countAiAssistant && draft.enableAiAssistant) {
    count += 1;
  }
  return count;
}
