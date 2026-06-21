import { describe, it, expect } from "vitest";
import {
  calculateUniqueVisitors,
  calculateHeatDistribution,
  isOverdue,
  daysOverdue,
  confidenceLabel,
} from "./calculations";

describe("calculateUniqueVisitors", () => {
  it("counts unique emails", () => {
    const logs = [
      { visitorEmail: "a@example.com" },
      { visitorEmail: "b@example.com" },
      { visitorEmail: "a@example.com" },
    ];
    expect(calculateUniqueVisitors(logs)).toBe(2);
  });

  it("returns 0 for empty logs", () => {
    expect(calculateUniqueVisitors([])).toBe(0);
  });
});

describe("calculateHeatDistribution", () => {
  it("groups contacts by heat level", () => {
    const contacts = [
      { heatLevel: "hot" as const },
      { heatLevel: "hot" as const },
      { heatLevel: "warm" as const },
    ];
    expect(calculateHeatDistribution(contacts)).toEqual({ hot: 2, warm: 1, cold: 0 });
  });
});

describe("isOverdue", () => {
  it("returns true for past dates", () => {
    expect(isOverdue("2000-01-01T00:00:00Z")).toBe(true);
  });

  it("returns false for future dates", () => {
    expect(isOverdue("2099-01-01T00:00:00Z")).toBe(false);
  });
});

describe("daysOverdue", () => {
  it("returns 0 for future dates", () => {
    expect(daysOverdue("2099-01-01T00:00:00Z")).toBe(0);
  });

  it("returns positive days for past dates", () => {
    const days = daysOverdue("2000-01-01T00:00:00Z");
    expect(days).toBeGreaterThan(0);
  });
});

describe("confidenceLabel", () => {
  it("returns high confidence key for large samples", () => {
    expect(confidenceLabel(50)).toBe("common:confidence.high");
  });

  it("returns medium confidence key for moderate samples", () => {
    expect(confidenceLabel(10)).toBe("common:confidence.medium");
  });

  it("returns low confidence key for small samples", () => {
    expect(confidenceLabel(3)).toBe("common:confidence.low");
  });
});
