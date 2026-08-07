import { describe, expect, it } from "vitest";
import {
  publicAskTurnToKnowledgeTurn,
  turnNeedsAIStream,
} from "./turnModel";
import type { PublicAskTurn } from "@/types";

describe("turnNeedsAIStream", () => {
  it("detects streaming AI turns", () => {
    const turn: PublicAskTurn = {
      id: "t1",
      session_id: "s1",
      question: "q",
      lane: "ai",
      status: "ai_streaming",
      created_at: "",
      updated_at: "",
    };
    expect(turnNeedsAIStream(turn)).toBe(true);
  });
});

describe("publicAskTurnToKnowledgeTurn", () => {
  it("maps completed AI payload", () => {
    const turn: PublicAskTurn = {
      id: "t1",
      session_id: "s1",
      question: "Revenue?",
      lane: "ai",
      status: "ai_answered",
      ai_payload: {
        answer: "12% growth",
        refused: false,
        resultStatus: "answered",
        hits: [{ chunkId: "c1", text: "hit", score: 0.9 }],
      },
      created_at: "",
      updated_at: "",
    };
    const kt = publicAskTurnToKnowledgeTurn(turn);
    expect(kt?.phase).toBe("done");
    expect(kt?.answer).toBe("12% growth");
    expect(kt?.results).toHaveLength(1);
  });
});
