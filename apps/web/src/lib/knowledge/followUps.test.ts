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
    expect(tips.every((t) => t.kind === "narrow")).toBe(true);
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

  it("suggests narrowing for RAG context-missing mixed answers (not split chips)", () => {
    const answer =
      "提供的上下文未包含2025年GMV年增长数据。材料中可见「Managed Ad Spend」约 4.8 亿元人民币，但缺少 GMV 基数或同比口径，无法据此计算年增长。";
    expect(
      followUpNeedsNarrowing({
        refused: false,
        resultStatus: "answered",
        question: "年增长GMV多少？",
        answer,
        hits: [
          { sourceName: "00_财务口径统一说明.pdf" },
          { sourceName: "01_商业计划书_Pitch_Deck_v2026_财务口径已修订.pdf" },
        ],
        unresolved: ["提供的上下文未包含2025年GMV年增长数据。"],
      }),
    ).toBe(true);
    const tips = buildRoomFollowUps({
      refused: false,
      resultStatus: "answered",
      question: "年增长GMV多少？",
      answer,
      hits: [
        { sourceName: "00_财务口径统一说明.pdf" },
        { sourceName: "01_商业计划书_Pitch_Deck_v2026_财务口径已修订.pdf" },
      ],
      unresolved: ["提供的上下文未包含2025年GMV年增长数据。"],
    });
    expect(tips.map((t) => t.id)).toEqual(["narrow-scope", "name-clause"]);
    expect(tips.every((t) => t.kind === "narrow")).toBe(true);
  });

  it("uses slot0 verify from a grounded claim, not liability templates", () => {
    const tips = buildRoomFollowUps({
      refused: false,
      resultStatus: "answered",
      question: "valuation cap?",
      answer: "The cap is $10M.",
      hits: [{ sourceName: "NDA.pdf", chunkId: "c1", text: "cap $10M" }],
      claims: [{ text: "The cap is $10M", hitIds: ["c1"], confidence: "grounded" }],
    });
    expect(tips[0]?.id).toBe("gap-verify-claim");
    expect(tips[0]?.kind).toBe("verify");
    expect(tips[0]?.params?.preview).toContain("10M");
    expect(tips.some((t) => t.id === "liability-in-source")).toBe(false);
  });

  it("puts unresolved into slot0 as conflict", () => {
    const tips = buildRoomFollowUps({
      refused: false,
      resultStatus: "answered",
      question: "liability cap?",
      answer: "The exception is not stated.",
      hits: [{ sourceName: "SPA.pdf" }],
      unresolved: ["The SPA does not state the liability cap exception."],
    });
    expect(tips[0]?.kind).toBe("conflict");
    expect(tips[0]?.id).toMatch(/^gap-unresolved-/);
    expect(tips[0]?.params?.prompt).toMatch(/liability cap/i);
  });

  it("rewrites uncovered pack items against this-turn anchor", () => {
    const tips = buildRoomFollowUps({
      refused: false,
      resultStatus: "answered",
      question: "What is the gross margin?",
      answer: "Gross margin is 62%.",
      hits: [{ sourceName: "Unit_Economics.xlsx", chunkId: "h1", text: "62%" }],
      claims: [{ text: "Gross margin is 62%", hitIds: ["h1"], confidence: "grounded" }],
      packItems: [
        {
          id: "option_pool",
          prompt: "How is the employee option pool sized in this room’s financing docs?",
          keywords: ["option", "pool", "期权池"],
          covered: false,
        },
      ],
    });
    const cover = tips.find((t) => t.kind === "cover");
    expect(cover).toBeTruthy();
    expect(cover?.params?.anchor).toMatch(/62|margin/i);
    expect(cover?.params?.topic).toMatch(/option|pool|期权/i);
    expect(cover?.params?.topic).not.toMatch(/option pool sized/i);
  });

  it("covers top-1 and top-2 as a cross-file extra when the claim is on-topic", () => {
    const tips = buildRoomFollowUps({
      refused: false,
      resultStatus: "answered",
      question: "purchase price?",
      answer: "The purchase price is ten million.",
      hits: [
        { sourceName: "SPA.pdf", chunkId: "c1", text: "ten million" },
        { sourceName: "Disclosure.xlsx" },
      ],
      claims: [
        { text: "The purchase price is ten million", hitIds: ["c1"], confidence: "grounded" },
      ],
    });
    expect(tips[0]?.kind).toBe("verify");
    expect(tips.some((t) => t.id === "liability-in-source")).toBe(false);
    const cross = tips.find((t) => t.id === "gap-cross-file");
    expect(cross?.kind).toBe("conflict");
    expect(cross?.params).toEqual({
      sourceA: "SPA.pdf",
      sourceB: "Disclosure.xlsx",
    });
  });

  it("narrows when no grounded claim covers the asked topic", () => {
    const tips = buildRoomFollowUps({
      refused: false,
      resultStatus: "answered",
      question: "purchase price?",
      answer: "See the documents.",
      hits: [
        { sourceName: "SPA.pdf" },
        { sourceName: "Disclosure.xlsx" },
      ],
    });
    expect(tips.map((t) => t.id)).toEqual(["narrow-scope", "name-clause"]);
  });

  it("falls back to narrowing when hits lack source names and no grounded claim", () => {
    const tips = buildRoomFollowUps({
      refused: false,
      resultStatus: "answered",
      question: "valuation cap",
      answer: "ten million",
      hits: [{ sourceName: "" }],
    });
    expect(tips.map((t) => t.id)).toEqual(["narrow-scope", "name-clause"]);
  });

  it("ignores weak claims even when they have hit ids", () => {
    const tips = buildRoomFollowUps({
      refused: false,
      resultStatus: "answered",
      question: "valuation cap",
      answer: "see the memo",
      hits: [{ sourceName: "Memo.pdf", chunkId: "c1", text: "maybe 10M" }],
      claims: [{ text: "The cap is maybe 10M", hitIds: ["c1"], confidence: "weak" }],
    });
    expect(tips.map((t) => t.id)).toEqual(["narrow-scope", "name-clause"]);
  });

  it("narrows when the only grounded claim is off-topic", () => {
    const tips = buildRoomFollowUps({
      refused: false,
      resultStatus: "answered",
      question: "年增长GMV多少？",
      answer: "材料可见 Managed Ad Spend 约 4.8 亿元，但未给出 GMV 年增长。",
      hits: [{ sourceName: "口径.pdf", chunkId: "h1", text: "4.8 亿" }],
      claims: [
        { text: "Managed Ad Spend 约 4.8 亿元", hitIds: ["h1"], confidence: "grounded" },
      ],
    });
    expect(tips.every((t) => t.kind === "narrow")).toBe(true);
    expect(tips.some((t) => t.id === "gap-verify-claim")).toBe(false);
    expect(JSON.stringify(tips)).not.toMatch(/4\.8|Ad Spend/i);
  });

  it("verifies 毛利率 on first paint without packItems (no YAML cover)", () => {
    const tips = buildRoomFollowUps({
      refused: false,
      resultStatus: "answered",
      question: "毛利率是多少？",
      answer: "毛利率为 62%。",
      hits: [{ sourceName: "单元经济.xlsx", chunkId: "h1", text: "毛利率 62%" }],
      claims: [{ text: "毛利率为 62%", hitIds: ["h1"], confidence: "grounded" }],
    });
    expect(tips[0]?.kind).toBe("verify");
    expect(tips[0]?.params?.preview ?? tips[0]?.params?.anchor ?? "").toMatch(
      /62|毛利率/,
    );
    expect(tips.some((t) => t.kind === "cover")).toBe(false);
    expect(JSON.stringify(tips)).not.toMatch(/期权池规模如何约定|option pool sized/i);
  });

  it("does not stack a second conflict extra on unresolved slot0", () => {
    const tips = buildRoomFollowUps({
      refused: false,
      resultStatus: "answered",
      question: "liability cap?",
      answer: "The exception is not stated.",
      hits: [{ sourceName: "NDA.pdf" }, { sourceName: "SPA.pdf" }],
      unresolved: ["The SPA does not state the liability cap exception."],
    });
    expect(tips[0]?.kind).toBe("conflict");
    expect(tips.filter((t) => t.kind === "conflict")).toHaveLength(1);
  });

  it("does not use YAML prompt as cover topic when keywords are missing", () => {
    const tips = buildRoomFollowUps({
      refused: false,
      resultStatus: "answered",
      question: "What is the gross margin?",
      answer: "Gross margin is 62%.",
      hits: [{ sourceName: "Unit_Economics.xlsx", chunkId: "h1", text: "62%" }],
      claims: [{ text: "Gross margin is 62%", hitIds: ["h1"], confidence: "grounded" }],
      packItems: [
        {
          id: "option_pool",
          prompt: "How is the employee option pool sized in this room’s financing docs?",
          keywords: [],
          covered: false,
        },
      ],
    });
    expect(tips.some((t) => t.kind === "cover")).toBe(false);
    expect(tips.some((t) => (t.params?.topic ?? "").includes("option pool sized"))).toBe(
      false,
    );
  });
});
