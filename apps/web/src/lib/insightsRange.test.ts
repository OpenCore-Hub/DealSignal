import { describe, expect, it } from "vitest";
import {
  inclusiveUtcDayCount,
  isInsightsCustomActive,
  isInsightsPresetActive,
  validateInsightsCustomRange,
  utcTodayISO,
} from "./insightsRange";

describe("insightsRange", () => {
  it("counts inclusive UTC days", () => {
    expect(inclusiveUtcDayCount("2026-07-01", "2026-07-14")).toBe(14);
    expect(inclusiveUtcDayCount("2026-08-08", "2026-08-08")).toBe(1);
  });

  it("rejects inverted or invalid ranges", () => {
    expect(validateInsightsCustomRange("2026-08-08", "2026-08-01")).toBe("order");
    expect(validateInsightsCustomRange("08-01-2026", "2026-08-08")).toBe("invalid");
    expect(validateInsightsCustomRange("", "2026-08-08")).toBe("incomplete");
  });

  it("enforces 90-day max", () => {
    expect(validateInsightsCustomRange("2026-01-01", "2026-08-08")).toBe("tooLong");
    expect(validateInsightsCustomRange("2026-05-11", "2026-08-08")).toBeNull();
  });

  it("formats UTC today", () => {
    expect(utcTodayISO(new Date("2026-08-08T15:00:00Z"))).toBe("2026-08-08");
  });
});

describe("insights range chip active state", () => {
  it("does not keep a preset highlighted while the custom panel is open", () => {
    const preset30 = { kind: "preset" as const, days: 30 as const };
    expect(isInsightsPresetActive(preset30, true, 30)).toBe(false);
    expect(isInsightsCustomActive(preset30, true)).toBe(true);
  });

  it("highlights only the selected preset when custom is closed", () => {
    const preset30 = { kind: "preset" as const, days: 30 as const };
    expect(isInsightsPresetActive(preset30, false, 30)).toBe(true);
    expect(isInsightsPresetActive(preset30, false, 7)).toBe(false);
    expect(isInsightsCustomActive(preset30, false)).toBe(false);
  });

  it("keeps custom highlighted after apply", () => {
    const custom = { kind: "custom" as const, from: "2026-07-10", to: "2026-08-08" };
    expect(isInsightsCustomActive(custom, false)).toBe(true);
    expect(isInsightsPresetActive(custom, false, 30)).toBe(false);
  });
});
