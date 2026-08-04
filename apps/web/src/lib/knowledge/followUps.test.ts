import { describe, expect, it } from "vitest";
import {
  buildRoomFollowUps,
  coverageSourceNames,
  followUpNeedsNarrowing,
} from "./followUps";

describe("coverageSourceNames", () => {
  it("dedupes and preserves retrieval order", () => {
    expect(
      coverageSourceNames([
        { sourceName: "A.pdf" },
        { sourceName: "a.pdf" },
        { sourceName: "B.pdf" },
        { sourceName: "C.pdf" },
        { sourceName: "D.pdf" },
      ]),
    ).toEqual(["A.pdf", "B.pdf", "C.pdf"]);
  });
});

describe("buildRoomFollowUps", () => {
  it("suggests narrowing when the last turn refused", () => {
    const tips = buildRoomFollowUps({
      refused: true,
      resultStatus: "refused",
      hits: [],
    });
    expect(tips.map((t) => t.id)).toEqual(["narrow-scope", "name-clause"]);
  });

  it("suggests narrowing for no_hits even when refused=false", () => {
    expect(
      followUpNeedsNarrowing({
        refused: false,
        resultStatus: "no_hits",
        hits: [{ sourceName: "Memo.pdf" }],
      }),
    ).toBe(true);
    const tips = buildRoomFollowUps({
      refused: false,
      resultStatus: "no_hits",
      hits: [{ sourceName: "Memo.pdf" }],
    });
    expect(tips.map((t) => t.id)).toEqual(["narrow-scope", "name-clause"]);
  });

  it("suggests narrowing for error turns", () => {
    const tips = buildRoomFollowUps({
      refused: false,
      resultStatus: "error",
      hits: [],
    });
    expect(tips.map((t) => t.id)).toEqual(["narrow-scope", "name-clause"]);
  });

  it("suggests narrowing when answer text is soft-refusal even if refused=false", () => {
    expect(
      followUpNeedsNarrowing({
        refused: false,
        resultStatus: "answered",
        hits: [{ sourceName: "NDA.pdf" }],
        answer:
          "根据您提供的上下文，文档属于单向保密协议。因此，无法根据现有上下文回答该问题。",
      }),
    ).toBe(true);
    const tips = buildRoomFollowUps({
      refused: false,
      resultStatus: "answered",
      hits: [{ sourceName: "NDA.pdf" }],
      answer: "因此，无法根据现有上下文回答该问题。",
    });
    expect(tips.map((t) => t.id)).toEqual(["narrow-scope", "name-clause"]);
  });

  it("anchors three chips to a single room source name", () => {
    const tips = buildRoomFollowUps({
      refused: false,
      resultStatus: "answered",
      hits: [{ sourceName: "NDA.pdf" }],
    });
    expect(tips).toHaveLength(3);
    expect(tips[0].params?.sourceName).toBe("NDA.pdf");
    expect(tips.every((t) => t.messageKey.startsWith("knowledge.followUp."))).toBe(true);
  });

  it("covers top-1, top-2, and cross-file when multiple sources hit", () => {
    const tips = buildRoomFollowUps({
      refused: false,
      resultStatus: "answered",
      hits: [
        { sourceName: "SPA.pdf" },
        { sourceName: "SPA.pdf" },
        { sourceName: "Disclosure.xlsx" },
        { sourceName: "Memo.docx" },
      ],
    });
    expect(tips.map((t) => t.id)).toEqual([
      "liability-in-source",
      "exceptions-in-second-source",
      "cross-file-consistency",
    ]);
    expect(tips[0]?.params?.sourceName).toBe("SPA.pdf");
    expect(tips[1]?.params?.sourceName).toBe("Disclosure.xlsx");
    expect(tips[2]?.params).toEqual({
      sourceA: "SPA.pdf",
      sourceB: "Disclosure.xlsx",
    });
  });

  it("falls back to clause prompts when hits lack source names", () => {
    const tips = buildRoomFollowUps({
      refused: false,
      resultStatus: "answered",
      hits: [{ sourceName: "" }],
    });
    expect(tips.map((t) => t.id)).toEqual(["specific-clause", "party-obligations"]);
  });
});
