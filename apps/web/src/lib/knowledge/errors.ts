const KNOWLEDGE_ERROR_CODES = new Set([
  "knowledge_unavailable",
  "forbidden",
  "not_found",
  "query_failed",
  "knowledge_query_busy",
  "knowledge_query_rate_limited",
  "knowledge_query_quota_exceeded",
]);

/** Map stable server error codes to locale strings; never surface raw Go text. */
export function knowledgeErrorMessage(
  t: (key: string) => string,
  code?: string | null,
): string {
  const c = (code ?? "").trim();
  if (c && KNOWLEDGE_ERROR_CODES.has(c)) {
    return t(`knowledge.errors.${c}`);
  }
  return t("knowledge.queryFailed");
}
