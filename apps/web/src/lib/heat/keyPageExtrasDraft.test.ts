import { describe, expect, it } from "vitest";
import {
  draftsFromExtras,
  editorCategoriesForCircle,
  extrasFromDrafts,
  parseKeywordDraft,
} from "./keyPageExtrasDraft";

describe("keyPageExtrasDraft", () => {
  it("parses mixed separators", () => {
    expect(parseKeywordDraft("watermark, 股权\nNDA")).toEqual(["watermark", "股权", "NDA"]);
  });

  it("lists circle categories plus custom and leftover extras", () => {
    const cats = editorCategoriesForCircle("founder", {
      pricing: ["quote"],
      custom: ["watermark"],
    });
    expect(cats).toContain("financials");
    expect(cats).toContain("pricing");
    expect(cats[cats.length - 1]).toBe("custom");
  });

  it("round-trips drafts and extras", () => {
    const extras = { financials: ["cap table"], custom: ["watermark", "股权"] };
    const drafts = draftsFromExtras(["financials", "team", "custom"], extras);
    expect(drafts.financials).toBe("cap table");
    expect(drafts.team).toBe("");
    expect(extrasFromDrafts(drafts)).toEqual(extras);
  });
});
