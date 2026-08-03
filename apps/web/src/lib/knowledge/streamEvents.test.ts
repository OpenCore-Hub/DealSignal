import { describe, expect, it } from "vitest";
import {
  createKnowledgeTurn,
  reduceKnowledgeStream,
  turnFromQATurn,
} from "./streamEvents";

describe("reduceKnowledgeStream", () => {
  it("never keeps evidence after a refuse done event", () => {
    let turn = createKnowledgeTurn("q");
    turn = reduceKnowledgeStream(turn, { type: "phase", phase: "retrieving" });
    turn = reduceKnowledgeStream(turn, {
      type: "sources",
      results: [{ chunkId: "c", text: "noise", score: 0.2 }],
      grounded: true,
    });
    turn = reduceKnowledgeStream(turn, {
      type: "done",
      answer: "does not contain an answer",
      refused: true,
      results: [{ chunkId: "c", text: "noise", score: 0.2 }],
    });
    expect(turn.phase).toBe("refused");
    expect(turn.refused).toBe(true);
    expect(turn.results).toEqual([]);
  });

  it("ignores ungrounded sources frames", () => {
    let turn = createKnowledgeTurn("q");
    turn = reduceKnowledgeStream(turn, {
      type: "sources",
      results: [{ chunkId: "c", text: "x", score: 0.1 }],
      grounded: false,
    });
    expect(turn.results).toEqual([]);
  });

  it("grows answer from token* then reconciles done", () => {
    let turn = createKnowledgeTurn("q");
    turn = reduceKnowledgeStream(turn, { type: "phase", phase: "retrieving" });
    turn = reduceKnowledgeStream(turn, { type: "phase", phase: "generating" });
    turn = reduceKnowledgeStream(turn, {
      type: "sources",
      results: [{ chunkId: "c", text: "cap", score: 0.9, sourceName: "Memo.pdf" }],
      grounded: true,
    });
    turn = reduceKnowledgeStream(turn, { type: "token", text: "Grounded " });
    turn = reduceKnowledgeStream(turn, { type: "token", text: "answer" });
    expect(turn.phase).toBe("generating");
    expect(turn.answer).toBe("Grounded answer");
    turn = reduceKnowledgeStream(turn, {
      type: "done",
      answer: "Grounded answer for: q",
      refused: false,
      resultStatus: "answered",
      results: [{ chunkId: "c", text: "cap", score: 0.9, sourceName: "Memo.pdf" }],
    });
    expect(turn.phase).toBe("done");
    expect(turn.answer).toBe("Grounded answer for: q");
    expect(turn.results).toHaveLength(1);
  });
});

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
