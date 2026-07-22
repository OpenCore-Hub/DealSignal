export type AskDocsCoverageWarning = {
  code: string;
  message: string;
  missing_folder_paths?: string[];
  missing_document_ids?: string[];
};

type Translate = (key: string, options?: Record<string, unknown>) => string;

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

function formatCoverageGaps(warning: AskDocsCoverageWarning): string[] {
  const folders = (warning.missing_folder_paths ?? []).map((p) => p.trim()).filter(Boolean);
  const docs = (warning.missing_document_ids ?? []).map((id) => id.trim()).filter(Boolean);
  return [...folders, ...docs];
}

/** Soft warning when link authorization is not covered by KB selection. */
export function askDocsCoverageWarningMessage(
  warnings: AskDocsCoverageWarning[] | undefined | null,
  t: Translate
): string | null {
  const hit = warnings?.find((w) => w.code === "ask_docs_scope_not_in_kb");
  if (!hit) return null;
  const base = t("accessRules.advanced.askDocsScopeNotInKb");
  const gaps = formatCoverageGaps(hit);
  if (gaps.length === 0) return base;
  return `${base} ${t("accessRules.advanced.askDocsScopeGaps", { items: gaps.join(", ") })}`;
}

export function extractAskDocsWarnings(payload: unknown): AskDocsCoverageWarning[] | undefined {
  if (!payload || typeof payload !== "object") return undefined;
  const warnings = (payload as { warnings?: unknown }).warnings;
  if (!Array.isArray(warnings)) return undefined;
  return warnings.filter(
    (w): w is AskDocsCoverageWarning =>
      !!w && typeof w === "object" && typeof (w as AskDocsCoverageWarning).code === "string"
  ).map((w) => {
    const raw = w as AskDocsCoverageWarning & Record<string, unknown>;
    const folders = Array.isArray(raw.missing_folder_paths)
      ? raw.missing_folder_paths.filter((p): p is string => typeof p === "string")
      : undefined;
    const docs = Array.isArray(raw.missing_document_ids)
      ? raw.missing_document_ids.filter((id): id is string => typeof id === "string")
      : undefined;
    return {
      code: raw.code,
      message: typeof raw.message === "string" ? raw.message : "",
      missing_folder_paths: folders,
      missing_document_ids: docs,
    };
  });
}
