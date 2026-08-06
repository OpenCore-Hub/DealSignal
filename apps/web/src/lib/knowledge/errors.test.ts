import { describe, expect, it } from "vitest";
import { isKnowledgeAskGateReject, knowledgeErrorMessage } from "./errors";

describe("knowledgeErrorMessage", () => {
  const t = (key: string) => `i18n:${key}`;

  it("maps gate / quota / audit codes", () => {
    expect(knowledgeErrorMessage(t, "knowledge_query_busy")).toBe(
      "i18n:knowledge.errors.knowledge_query_busy",
    );
    expect(knowledgeErrorMessage(t, "knowledge_query_rate_limited")).toBe(
      "i18n:knowledge.errors.knowledge_query_rate_limited",
    );
    expect(knowledgeErrorMessage(t, "knowledge_query_quota_exceeded")).toBe(
      "i18n:knowledge.errors.knowledge_query_quota_exceeded",
    );
    expect(knowledgeErrorMessage(t, "knowledge_query_quota_unavailable")).toBe(
      "i18n:knowledge.errors.knowledge_query_quota_unavailable",
    );
    expect(knowledgeErrorMessage(t, "client_cancelled")).toBe(
      "i18n:knowledge.errors.client_cancelled",
    );
    expect(knowledgeErrorMessage(t, "query_timeout")).toBe(
      "i18n:knowledge.errors.query_timeout",
    );
    expect(knowledgeErrorMessage(t, "answer_requires_session")).toBe(
      "i18n:knowledge.errors.answer_requires_session",
    );
    expect(knowledgeErrorMessage(t, "knowledge_corpus_not_ready")).toBe(
      "i18n:knowledge.errors.knowledge_corpus_not_ready",
    );
    expect(knowledgeErrorMessage(t, "rate_limit_exceeded")).toBe(
      "i18n:knowledge.errors.knowledge_query_rate_limited",
    );
    expect(knowledgeErrorMessage(t, "stream_incomplete")).toBe(
      "i18n:knowledge.errors.stream_incomplete",
    );
  });

  it("falls back for unknown codes", () => {
    expect(knowledgeErrorMessage(t, "nope")).toBe("i18n:knowledge.queryFailed");
    expect(knowledgeErrorMessage(t, null)).toBe("i18n:knowledge.queryFailed");
  });
});

describe("isKnowledgeAskGateReject", () => {
  it("treats 429/409 and known gate codes as non-hydrating", () => {
    expect(isKnowledgeAskGateReject(429, "knowledge_query_busy")).toBe(true);
    expect(isKnowledgeAskGateReject(409, "knowledge_corpus_not_ready")).toBe(true);
    expect(isKnowledgeAskGateReject(500, "internal_error")).toBe(false);
    expect(isKnowledgeAskGateReject(200, "")).toBe(false);
  });
});
