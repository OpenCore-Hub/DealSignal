import { describe, expect, it } from "vitest";
import {
  filterPromotableDeskTexts,
  isPromotableDeskFollowUpText,
  isUngroundedKnowledgeAnswer,
  looksLikeNonRoomFactMeta,
  looksLikeOutOfRoomGeneralKnowledge,
} from "./trustGates";

describe("trustGates", () => {
  it("detects soft Chinese refusals", () => {
    expect(
      isUngroundedKnowledgeAnswer(
        "根据您提供的上下文，文档属于单向保密协议。因此，无法根据现有上下文回答该问题。",
      ),
    ).toBe(true);
    expect(
      looksLikeNonRoomFactMeta("现有材料不足以确定该问题。"),
    ).toBe(true);
    expect(
      looksLikeNonRoomFactMeta("提供的上下文未包含2025年GMV年增长数据。"),
    ).toBe(true);
    expect(
      isUngroundedKnowledgeAnswer(
        "提供的上下文未包含2025年GMV年增长数据。材料中可见 Managed Ad Spend 约 4.8 亿元。",
      ),
    ).toBe(false);
  });

  it("blocks industry trivia from desk promotion", () => {
    expect(
      looksLikeOutOfRoomGeneralKnowledge(
        "The EBITDA multiple for this sector is typically 12x.",
      ),
    ).toBe(true);
    expect(isPromotableDeskFollowUpText("同行一般怎么谈对赌条款？")).toBe(
      false,
    );
    expect(
      filterPromotableDeskTexts([
        "因此，无法根据现有上下文回答该问题。",
        "EBITDA multiple is typically 12x in this sector.",
        "该模式仅约束接收方保守披露方的信息，对接收方明显不利。",
      ]),
    ).toEqual([
      "该模式仅约束接收方保守披露方的信息，对接收方明显不利。",
    ]);
  });

  it("keeps room-local gaps", () => {
    expect(
      isPromotableDeskFollowUpText(
        "The exclusivity period under the letter of intent is ninety days.",
      ),
    ).toBe(true);
  });
});
