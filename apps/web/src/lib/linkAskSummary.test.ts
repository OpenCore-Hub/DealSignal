import { describe, expect, it } from "vitest";
import { formatAskDeflectionRate, hasAskActivity } from "./linkAskSummary";

describe("linkAskSummary", () => {
  it("formats deflection rate as whole percent", () => {
    expect(formatAskDeflectionRate(0.605)).toBe("61%");
    expect(formatAskDeflectionRate(undefined)).toBe("—");
  });

  it("detects ask activity", () => {
    expect(hasAskActivity(undefined)).toBe(false);
    expect(hasAskActivity({ total_turns: 0, ai_answered: 0, ai_refused: 0, host_pending: 0, host_answered: 0 })).toBe(false);
    expect(hasAskActivity({ total_turns: 1, ai_answered: 1, ai_refused: 0, host_pending: 0, host_answered: 0 })).toBe(true);
  });
});
