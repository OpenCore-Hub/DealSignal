import { describe, expect, it } from "vitest";
import { usageAtCap } from "./planQuota";

describe("usageAtCap", () => {
  it("treats non-positive limits as unlimited", () => {
    expect(usageAtCap(100, 0)).toBe(false);
    expect(usageAtCap(100, -1)).toBe(false);
  });

  it("flags when used reaches a finite cap", () => {
    expect(usageAtCap(1, 1)).toBe(true);
    expect(usageAtCap(0, 1)).toBe(false);
    expect(usageAtCap(2, 3)).toBe(false);
  });
});
