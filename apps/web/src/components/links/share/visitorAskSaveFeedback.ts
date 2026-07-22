export type AskDocsCoverageWarning = {
  code: string;
  message: string;
  missing_folder_paths?: string[];
  missing_document_ids?: string[];
};

type Translate = (key: string) => string;

/** Hard gate when enabling Ask Docs without a ready/stale room KB. */
export function visitorAskSaveErrorMessage(
  err: { code?: string } | null | undefined,
  t: Translate
): string | null {
  if (err?.code === "knowledge_base_required") {
    return t("accessRules.advanced.knowledgeBaseRequired");
  }
  return null;
}

/** Soft warning when link authorization is not covered by KB selection. */
export function askDocsCoverageWarningMessage(
  warnings: AskDocsCoverageWarning[] | undefined | null,
  t: Translate
): string | null {
  const hit = warnings?.find((w) => w.code === "ask_docs_scope_not_in_kb");
  if (!hit) return null;
  return t("accessRules.advanced.askDocsScopeNotInKb");
}

export function extractAskDocsWarnings(payload: unknown): AskDocsCoverageWarning[] | undefined {
  if (!payload || typeof payload !== "object") return undefined;
  const warnings = (payload as { warnings?: unknown }).warnings;
  if (!Array.isArray(warnings)) return undefined;
  return warnings.filter(
    (w): w is AskDocsCoverageWarning =>
      !!w && typeof w === "object" && typeof (w as AskDocsCoverageWarning).code === "string"
  );
}
