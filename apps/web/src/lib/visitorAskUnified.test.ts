import { describe, expect, it } from "vitest";
import { visitorAskUnifiedEnabled } from "./visitorAskUnified";

describe("visitorAskUnifiedEnabled", () => {
  it("requires qa and server flag", () => {
    expect(visitorAskUnifiedEnabled({ qaEnabled: true, visitorAskUnified: true })).toBe(true);
    expect(visitorAskUnifiedEnabled({ qaEnabled: true, visitorAskUnified: false })).toBe(false);
    expect(visitorAskUnifiedEnabled({ qaEnabled: false, visitorAskUnified: true })).toBe(false);
  });
});
