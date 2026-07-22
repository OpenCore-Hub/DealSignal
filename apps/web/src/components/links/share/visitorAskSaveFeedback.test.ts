import { describe, expect, it } from "vitest";
import {
  askDocsCoverageWarningMessage,
  visitorAskSaveErrorMessage,
  type AskDocsCoverageWarning,
} from "./visitorAskSaveFeedback";

const t = (key: string) => `i18n:${key}`;

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
        missing_folder_paths: ["Legal"],
      },
    ];
    expect(askDocsCoverageWarningMessage(warnings, t)).toBe(
      "i18n:accessRules.advanced.askDocsScopeNotInKb"
    );
  });

  it("ignores unrelated warnings", () => {
    expect(
      askDocsCoverageWarningMessage([{ code: "other", message: "x" }], t)
    ).toBeNull();
    expect(askDocsCoverageWarningMessage(undefined, t)).toBeNull();
  });
});
