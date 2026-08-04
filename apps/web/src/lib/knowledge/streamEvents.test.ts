import { describe, expect, it } from "vitest";
import {
  createKnowledgeTurn,
  reduceKnowledgeStream,
  shouldShowEvidence,
  turnFromQATurn,
  turnRetrieveDisclosure,
} from "./streamEvents";

describe("reduceKnowledgeStream", () => {
  it("never keeps evidence after a no_hits done event (P4)", () => {
    let turn = createKnowledgeTurn("cap?");
    turn = reduceKnowledgeStream(turn, {
      type: "sources",
      results: [{ chunkId: "c", text: "noise", score: 0.2, sourceName: "SAFE.pdf" }],
      grounded: true,
    });
    turn = reduceKnowledgeStream(turn, {
      type: "done",
      answer: "",
      refused: false,
      resultStatus: "no_hits",
      results: [{ chunkId: "c", text: "noise", score: 0.2, sourceName: "SAFE.pdf" }],
      refusal: { kind: "no_hits", hadHits: true, hitCount: 1 },
    });
    expect(turn.results).toHaveLength(0);
    expect(shouldShowEvidence(turn)).toBe(false);
    expect(turn.refusal?.kind).toBe("no_hits");
  });

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

  it("carries conflict set on done without dropping evidence", () => {
    let turn = createKnowledgeTurn("valuation cap?");
    turn = reduceKnowledgeStream(turn, {
      type: "sources",
      results: [
        { chunkId: "c1", text: "cap 10m", score: 0.9, sourceName: "A.pdf" },
        { chunkId: "c2", text: "cap 8m", score: 0.8, sourceName: "B.pdf" },
      ],
      grounded: true,
    });
    turn = reduceKnowledgeStream(turn, {
      type: "done",
      answer: "Documents disagree",
      refused: false,
      resultStatus: "answered",
      results: [
        { chunkId: "c1", text: "cap 10m", score: 0.9, sourceName: "A.pdf" },
        { chunkId: "c2", text: "cap 8m", score: 0.8, sourceName: "B.pdf" },
      ],
      conflicts: [
        {
          id: "conflict-numeric-1",
          kind: "numeric",
          topic: "valuation",
          sides: [
            { sourceName: "A.pdf", hitId: "c1", value: "10000000", excerpt: "cap 10m" },
            { sourceName: "B.pdf", hitId: "c2", value: "8000000", excerpt: "cap 8m" },
          ],
        },
      ],
    });
    expect(turn.conflicts).toHaveLength(1);
    expect(turn.conflicts?.[0]?.sides).toHaveLength(2);
    expect(turn.results).toHaveLength(2);
  });

  it("keeps partial judgment on answered done events", () => {
    let turn = createKnowledgeTurn("What is the fee?");
    turn = reduceKnowledgeStream(turn, {
      type: "done",
      answer: "The fee looks like 2%.",
      refused: false,
      resultStatus: "answered",
      results: [{ chunkId: "c1", text: "fee", score: 0.5, sourceName: "SPA.pdf" }],
      judgment: {
        kind: "partial",
        reason: "weak_only",
        groundedClaims: 0,
        weakClaims: 1,
      },
    });
    expect(turn.judgment?.kind).toBe("partial");
    expect(turn.judgment?.reason).toBe("weak_only");
  });

  it("keeps typed refusal on refused done events", () => {
    let turn = createKnowledgeTurn("What is the fee?");
    turn = reduceKnowledgeStream(turn, {
      type: "done",
      answer: "The provided context does not contain an answer",
      refused: true,
      resultStatus: "refused",
      results: [],
      refusal: { kind: "ungrounded", hadHits: true, hitCount: 2 },
    });
    expect(turn.refusal?.kind).toBe("ungrounded");
    expect(turn.refusal?.hadHits).toBe(true);
    expect(turn.results).toHaveLength(0);
  });

  it("maps no_hits QA turns with refusal and clears hits (P4)", () => {
    const turn = turnFromQATurn({
      id: "t-gap",
      sessionId: "s1",
      sequence: 1,
      question: "cap?",
      refused: false,
      resultStatus: "no_hits",
      hits: [{ chunkId: "c1", text: "cap language", score: 0.4, sourceName: "SAFE.pdf" }],
      createdAt: new Date().toISOString(),
      refusal: { kind: "no_hits", hadHits: true, hitCount: 1 },
    });
    expect(turn.resultStatus).toBe("no_hits");
    expect(turn.refusal?.kind).toBe("no_hits");
    expect(turn.refusal?.hadHits).toBe(true);
    expect(turn.results).toHaveLength(0);
    expect(shouldShowEvidence(turn)).toBe(false);
  });

  it("keeps multiHop audit on done for definition/attachment hops", () => {
    let turn = createKnowledgeTurn("What is MAE?");
    turn = reduceKnowledgeStream(turn, {
      type: "done",
      answer: "As defined in Exhibit A",
      refused: false,
      resultStatus: "answered",
      results: [{ chunkId: "c1", text: "MAE", score: 0.9, sourceName: "SPA.pdf" }],
      multiHop: {
        applied: true,
        queries: [
          {
            kind: "definition",
            query: 'definition of "Material Adverse Effect"',
            anchor: "Material Adverse Effect",
          },
        ],
        addedHitIds: ["c9"],
      },
    });
    expect(turn.multiHop?.applied).toBe(true);
    expect(turn.multiHop?.queries?.[0]?.kind).toBe("definition");
    expect(turn.multiHop?.addedHitIds).toEqual(["c9"]);
  });

  it("applies rewrite audit on generating so disclosure is live during tokens", () => {
    let turn = createKnowledgeTurn("他们免费吗？");
    turn = reduceKnowledgeStream(turn, { type: "phase", phase: "retrieving" });
    turn = reduceKnowledgeStream(turn, {
      type: "phase",
      phase: "generating",
      rewriteApplied: true,
      retrieveQuery: "Acme SAFE valuation cap",
    });
    expect(turnRetrieveDisclosure(turn)).toBe("Acme SAFE valuation cap");
    turn = reduceKnowledgeStream(turn, { type: "token", text: "见原文" });
    expect(turnRetrieveDisclosure(turn)).toBe("Acme SAFE valuation cap");
    turn = reduceKnowledgeStream(turn, {
      type: "done",
      answer: "见原文",
      refused: false,
      resultStatus: "answered",
      results: [],
      rewriteApplied: true,
      retrieveQuery: "Acme SAFE valuation cap",
    });
    expect(turn.phase).toBe("done");
    expect(turnRetrieveDisclosure(turn)).toBe("Acme SAFE valuation cap");
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
      hits: [],
      createdAt: "2026-08-03T00:00:00Z",
    });
    expect(turn.phase).toBe("done");
    expect(turn.refused).toBe(false);
    expect(turn.resultStatus).toBe("no_hits");
    expect(turn.results).toEqual([]);
    expect(turn.refusal?.kind).toBe("no_hits");
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

  it("preserves rewrite audit fields for disclosure", () => {
    const turn = turnFromQATurn({
      id: "t4",
      sessionId: "s1",
      sequence: 4,
      question: "他们免费吗？",
      answer: "见原文",
      refused: false,
      resultStatus: "answered",
      hits: [],
      retrieveQuery: "Acme NDA.pdf 是否免费？",
      rewriteApplied: true,
      createdAt: "2026-08-03T00:00:00Z",
    });
    expect(turn.rewriteApplied).toBe(true);
    expect(turn.retrieveQuery).toBe("Acme NDA.pdf 是否免费？");
    expect(turnRetrieveDisclosure(turn)).toBe("Acme NDA.pdf 是否免费？");
  });

  it("hides disclosure when rewrite matches the display question", () => {
    expect(
      turnRetrieveDisclosure({
        ...createKnowledgeTurn("same"),
        rewriteApplied: true,
        retrieveQuery: "same",
      }),
    ).toBeNull();
    expect(
      turnRetrieveDisclosure({
        ...createKnowledgeTurn("q"),
        rewriteApplied: false,
        retrieveQuery: "other",
      }),
    ).toBeNull();
  });
});
