/**
 * Pre-send heuristic for Visitor Ask V1.5 channel suggestion.
 * Returns true when the draft looks like a request for missing materials
 * that is better suited to Ask Host than Ask Docs.
 */
export function suggestAskHostFromDraft(text: string): boolean {
  const draft = text.trim().toLowerCase();
  if (!draft) return false;

  const patterns = [
    /能否提供/,
    /可以提供/,
    /请提供/,
    /缺少/,
    /有没有.*(?:文件|文档|材料|资料)/,
    /can you provide/,
    /could you provide/,
    /please provide/,
    /is .+ missing/,
    /are .+ missing/,
    /missing from/,
    /do you have .+ (?:file|document|deck|model)/,
  ];

  return patterns.some((re) => re.test(draft));
}
