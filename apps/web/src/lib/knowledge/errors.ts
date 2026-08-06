const KNOWLEDGE_ERROR_CODES = new Set([
  "knowledge_unavailable",
  "forbidden",
  "not_found",
  "query_failed",
  "client_cancelled",
  "query_timeout",
  "answer_requires_session",
  "knowledge_corpus_not_ready",
  "knowledge_query_busy",
  "knowledge_query_rate_limited",
  "knowledge_query_quota_exceeded",
  "knowledge_query_quota_unavailable",
  "rate_limit_exceeded",
  "stream_incomplete",
]);

/** Codes / statuses that never persist an audit turn (skip post-fail hydrate). */
const KNOWLEDGE_ASK_GATE_CODES = new Set([
  "knowledge_corpus_not_ready",
  "answer_requires_session",
  "invalid_input",
  "forbidden",
  "not_found",
  "knowledge_query_busy",
  "knowledge_query_rate_limited",
  "knowledge_query_quota_exceeded",
  "knowledge_query_quota_unavailable",
  "rate_limit_exceeded",
]);

/** Map stable server error codes to locale strings; never surface raw Go text. */
export function knowledgeErrorMessage(
  t: (key: string) => string,
  code?: string | null,
): string {
  const c = (code ?? "").trim();
  if (c === "rate_limit_exceeded") {
    return t("knowledge.errors.knowledge_query_rate_limited");
  }
  if (c && KNOWLEDGE_ERROR_CODES.has(c)) {
    return t(`knowledge.errors.${c}`);
  }
  return t("knowledge.queryFailed");
}

/** True when the ask failed before an audit turn could be written. */
export function isKnowledgeAskGateReject(
  status: number,
  code?: string | null,
): boolean {
  if (status === 429 || status === 409) return true;
  const c = (code ?? "").trim();
  return c !== "" && KNOWLEDGE_ASK_GATE_CODES.has(c);
}
