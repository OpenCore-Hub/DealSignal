/**
 * Grounded Chat component whitelist (philosophy §6 / ceiling Phase T).
 * Clients may only compose these named surfaces — no arbitrary HTML/layout trees.
 *
 * Owner Knowledge Tab + owner Viewer rail share `GroundedChatShell` + citation helpers.
 * Public Visitor Ask / UnifiedQAPanel stay on a separate product channel.
 */

export const GROUNDED_CHAT_WHITELIST = [
  "TrustChip",
  "CorpusStatus",
  "AnswerStream",
  "CiteMarker",
  "EvidenceRail",
  "EvidenceCard",
  "OpenPageAction",
  "Composer",
] as const;

export type GroundedChatWhitelistId = (typeof GROUNDED_CHAT_WHITELIST)[number];
