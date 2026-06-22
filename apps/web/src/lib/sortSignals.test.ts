import { describe, it, expect } from "vitest";
import { sortSignals } from "./sortSignals";
import type { Signal } from "@/types";

function makeSignal(type: Signal["type"], priority: Signal["priority"], createdAt: string): Signal {
  return {
    id: `${type}-${priority}-${createdAt}`,
    type,
    priority,
    createdAt,
    title: "signal.title",
    description: "signal.description",
    explanation: "signal.explanation",
    suggestion: "signal.suggestion",
  };
}

describe("sortSignals", () => {
  it("orders by type hot > risk > warm > cold", () => {
    const signals: Signal[] = [
      makeSignal("cold", "high", "2026-06-20T00:00:00Z"),
      makeSignal("warm", "high", "2026-06-20T00:00:00Z"),
      makeSignal("hot", "low", "2026-06-20T00:00:00Z"),
      makeSignal("risk", "high", "2026-06-20T00:00:00Z"),
    ];
    expect(sortSignals(signals).map((s) => s.type)).toEqual(["hot", "risk", "warm", "cold"]);
  });

  it("orders by priority when type is equal", () => {
    const signals: Signal[] = [
      makeSignal("hot", "low", "2026-06-20T00:00:00Z"),
      makeSignal("hot", "high", "2026-06-20T00:00:00Z"),
      makeSignal("hot", "medium", "2026-06-20T00:00:00Z"),
    ];
    expect(sortSignals(signals).map((s) => s.priority)).toEqual(["high", "medium", "low"]);
  });

  it("orders by recency when type and priority are equal", () => {
    const signals: Signal[] = [
      makeSignal("hot", "high", "2026-06-18T00:00:00Z"),
      makeSignal("hot", "high", "2026-06-20T00:00:00Z"),
      makeSignal("hot", "high", "2026-06-19T00:00:00Z"),
    ];
    expect(sortSignals(signals).map((s) => s.createdAt)).toEqual([
      "2026-06-20T00:00:00Z",
      "2026-06-19T00:00:00Z",
      "2026-06-18T00:00:00Z",
    ]);
  });

  it("does not mutate the original array", () => {
    const signals: Signal[] = [makeSignal("cold", "high", "2026-06-20T00:00:00Z"), makeSignal("hot", "high", "2026-06-20T00:00:00Z")];
    sortSignals(signals);
    expect(signals.map((s) => s.type)).toEqual(["cold", "hot"]);
  });
});
