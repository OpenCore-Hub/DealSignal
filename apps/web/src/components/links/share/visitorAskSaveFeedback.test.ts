import { describe, expect, it } from "vitest";
import {
  askDocsCoverageWarningMessage,
  extractAskDocsWarnings,
  visitorAskSaveErrorMessage,
  type AskDocsCoverageWarning,
} from "./visitorAskSaveFeedback";

const t = (key: string, options?: Record<string, unknown>) => {
  if (options && "items" in options) {
    return `i18n:${key}:${String(options.items)}`;
  }
  return `i18n:${key}`;
};

describe("visitorAskSaveFeedback", () => {
  it("maps knowledge_base_required to KB gate copy", () => {
    expect(visitorAskSaveErrorMessage({ code: "knowledge_base_required" }, t)).toBe(
      "i18n:accessRules.advanced.knowledgeBaseRequired"
    );
  });

  it("returns null for unrelated API errors", () => {
    expect(visitorAskSaveErrorMessage({ code: "duplicate_name" }, t)).toBeNull();
    expect(visitorAskSaveErrorMessage(null, t)).toBeNull();
  });

  it("maps ask_docs_scope_not_in_kb warning to soft coverage copy", () => {
    const warnings: AskDocsCoverageWarning[] = [
      {
        code: "ask_docs_scope_not_in_kb",
        message: "server text",
      },
    ];
    expect(askDocsCoverageWarningMessage(warnings, t)).toBe(
      "i18n:accessRules.advanced.askDocsScopeNotInKb"
    );
  });

  it("appends missing folder paths and document ids to the coverage warning", () => {
    const warnings: AskDocsCoverageWarning[] = [
      {
        code: "ask_docs_scope_not_in_kb",
        message: "server text",
        missing_folder_paths: ["Legal", "Finance/Models"],
        missing_document_ids: ["doc-abc"],
      },
    ];
    const msg = askDocsCoverageWarningMessage(warnings, t);
    expect(msg).toContain("i18n:accessRules.advanced.askDocsScopeNotInKb");
    expect(msg).toContain("i18n:accessRules.advanced.askDocsScopeGaps:");
    expect(msg).toContain("Legal");
    expect(msg).toContain("Finance/Models");
    expect(msg).toContain("doc-abc");
  });

  it("extracts coverage gap fields from save payloads", () => {
    const warnings = extractAskDocsWarnings({
      warnings: [
        {
          code: "ask_docs_scope_not_in_kb",
          message: "x",
          missing_folder_paths: ["Legal"],
          missing_document_ids: ["doc-1"],
        },
      ],
    });
    expect(warnings).toEqual([
      {
        code: "ask_docs_scope_not_in_kb",
        message: "x",
        missing_folder_paths: ["Legal"],
        missing_document_ids: ["doc-1"],
      },
    ]);
  });

  it("ignores unrelated warnings", () => {
    expect(
      askDocsCoverageWarningMessage([{ code: "other", message: "x" }], t)
    ).toBeNull();
    expect(askDocsCoverageWarningMessage(undefined, t)).toBeNull();
  });
});
