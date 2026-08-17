import { describe, expect, it } from "vitest";
import { KEY_PAGE_ENGAGED_MIN_SECONDS, isKeyPageEngaged } from "./keyPageEngage";

describe("isKeyPageEngaged", () => {
  it("treats the 3s gate as engaged", () => {
    expect(isKeyPageEngaged(KEY_PAGE_ENGAGED_MIN_SECONDS)).toBe(true);
    expect(isKeyPageEngaged(15)).toBe(true);
  });

  it("treats shorter dwell as skim", () => {
    expect(isKeyPageEngaged(2)).toBe(false);
    expect(isKeyPageEngaged(0)).toBe(false);
  });

  it("does not treat non-finite dwell as engaged", () => {
    expect(isKeyPageEngaged(Number.NaN)).toBe(false);
  });
});
