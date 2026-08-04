import { describe, expect, it } from "vitest";
import { resolveCiteOpenOutcome } from "./citeOutcome";
import type { KnowledgeTurn } from "@/lib/knowledge/streamEvents";
import type { DealRoomKnowledgeQATurn } from "@/types";

describe("resolveCiteOpenOutcome", () => {
  it("prefers live refused over audit hits", () => {
    const live = {
      refused: true,
      results: [{ id: "h1" }],
    } as unknown as KnowledgeTurn;
    const turns = [
      {
        refused: false,
        hits: [{ chunkId: "c1" }],
      },
    ] as unknown as DealRoomKnowledgeQATurn[];
    expect(resolveCiteOpenOutcome(live, turns)).toBe("refused");
  });

  it("uses last audit turn when no live turn", () => {
    const turns = [
      { refused: false, hits: [] },
      { refused: false, hits: [{ chunkId: "c1" }] },
    ] as unknown as DealRoomKnowledgeQATurn[];
    expect(resolveCiteOpenOutcome(null, turns)).toBe("grounded");
  });

  it("returns unknown when empty", () => {
    expect(resolveCiteOpenOutcome(null, [])).toBe("unknown");
  });
});
