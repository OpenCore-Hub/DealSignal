import { describe, expect, it } from "vitest";
import { knowledgeErrorMessage } from "./errors";

describe("knowledgeErrorMessage", () => {
  const t = (key: string) => `i18n:${key}`;

  it("maps gate / quota codes", () => {
    expect(knowledgeErrorMessage(t, "knowledge_query_busy")).toBe(
      "i18n:knowledge.errors.knowledge_query_busy",
    );
    expect(knowledgeErrorMessage(t, "knowledge_query_rate_limited")).toBe(
      "i18n:knowledge.errors.knowledge_query_rate_limited",
    );
    expect(knowledgeErrorMessage(t, "knowledge_query_quota_exceeded")).toBe(
      "i18n:knowledge.errors.knowledge_query_quota_exceeded",
    );
  });

  it("falls back for unknown codes", () => {
    expect(knowledgeErrorMessage(t, "nope")).toBe("i18n:knowledge.queryFailed");
    expect(knowledgeErrorMessage(t, null)).toBe("i18n:knowledge.queryFailed");
  });
});
