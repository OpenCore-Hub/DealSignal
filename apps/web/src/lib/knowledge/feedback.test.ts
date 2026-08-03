import { describe, expect, it } from "vitest";
import { asFeedbackKind, FEEDBACK_KINDS, isFeedbackKind } from "./feedback";

describe("feedback kinds", () => {
  it("accepts the three Phase C kinds only", () => {
    expect(FEEDBACK_KINDS).toEqual(["helpful", "wrong_citation", "not_answering"]);
    expect(isFeedbackKind("helpful")).toBe(true);
    expect(isFeedbackKind("thumbs_up")).toBe(false);
    expect(asFeedbackKind("wrong_citation")).toBe("wrong_citation");
    expect(asFeedbackKind("nope")).toBeUndefined();
  });
});
