import { describe, expect, it } from "vitest";
import { turnFromQATurn } from "./streamEvents";

describe("turnFromQATurn", () => {
  it("preserves no_hits for follow-up templates without inventing evidence", () => {
    const turn = turnFromQATurn({
      id: "t1",
      sessionId: "s1",
      sequence: 1,
      question: "q",
      answer: "",
      refused: false,
      resultStatus: "no_hits",
      hits: [{ chunkId: "c", text: "should clear", score: 0.1 }],
      createdAt: "2026-08-03T00:00:00Z",
    });
    expect(turn.phase).toBe("done");
    expect(turn.refused).toBe(false);
    expect(turn.resultStatus).toBe("no_hits");
    expect(turn.results).toEqual([]);
  });

  it("maps refused to empty evidence", () => {
    const turn = turnFromQATurn({
      id: "t2",
      sessionId: "s1",
      sequence: 2,
      question: "q",
      answer: "does not contain an answer",
      refused: true,
      resultStatus: "refused",
      hits: [{ chunkId: "c", text: "noise", score: 0.2 }],
      createdAt: "2026-08-03T00:00:00Z",
    });
    expect(turn.phase).toBe("refused");
    expect(turn.refused).toBe(true);
    expect(turn.resultStatus).toBe("refused");
    expect(turn.results).toEqual([]);
  });

  it("keeps answered hits and resultStatus", () => {
    const turn = turnFromQATurn(
      {
        id: "t3",
        sessionId: "s1",
        sequence: 3,
        question: "q",
        answer: "ok [1]",
        refused: false,
        resultStatus: "answered",
        hits: [
          {
            chunkId: "c",
            text: "cap",
            score: 0.9,
            sourceName: "Memo.pdf",
          },
        ],
        createdAt: "2026-08-03T00:00:00Z",
      },
      1,
    );
    expect(turn.phase).toBe("done");
    expect(turn.resultStatus).toBe("answered");
    expect(turn.results).toHaveLength(1);
    expect(turn.activeCite).toBe(1);
  });
});
