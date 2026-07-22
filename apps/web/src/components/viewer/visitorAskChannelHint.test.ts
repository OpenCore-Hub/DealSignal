// @vitest-environment node
import { describe, it, expect } from "vitest";
import { suggestAskHostFromDraft } from "./visitorAskChannelHint";

describe("suggestAskHostFromDraft", () => {
  it("suggests Ask Host for missing-document intents (zh)", () => {
    expect(suggestAskHostFromDraft("能否提供完整财报？")).toBe(true);
    expect(suggestAskHostFromDraft("这里好像缺少组织架构图")).toBe(true);
  });

  it("suggests Ask Host for missing-document intents (en)", () => {
    expect(suggestAskHostFromDraft("Can you provide the full financials?")).toBe(true);
    expect(suggestAskHostFromDraft("Is the cap table missing from this link?")).toBe(true);
  });

  it("does not suggest for ordinary document questions", () => {
    expect(suggestAskHostFromDraft("What is the Series A valuation?")).toBe(false);
    expect(suggestAskHostFromDraft("估值假设是什么？")).toBe(false);
    expect(suggestAskHostFromDraft("")).toBe(false);
  });
});
