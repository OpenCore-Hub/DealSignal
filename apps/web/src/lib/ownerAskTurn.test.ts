import { describe, expect, it, vi, beforeEach } from "vitest";
import {
  ownerAskTurnToVisitorQuestion,
  ownerAskTurnsToVisitorQuestions,
  answerOwnerAskQuestion,
} from "./ownerAskTurn";
import { api } from "@/lib/api";
import type { OwnerAskTurn, VisitorQuestion } from "@/types";

vi.mock("@/lib/api", () => ({
  api: {
    answerAskTurn: vi.fn(),
  },
}));

describe("ownerAskTurnToVisitorQuestion", () => {
  it("uses host_question_id for answer API and maps answered status", () => {
    const turn: OwnerAskTurn = {
      id: "turn-1",
      session_id: "sess-1",
      link_id: "link-1",
      visitor_id: "visitor-1",
      question: "Timeline?",
      lane: "host",
      status: "host_answered",
      host_question_id: "legacy-q-1",
      host_answer: "Next week",
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-02T00:00:00Z",
    };
    const q = ownerAskTurnToVisitorQuestion(turn);
    expect(q.id).toBe("legacy-q-1");
    expect(q.ask_turn_id).toBe("turn-1");
    expect(q.status).toBe("answered");
    expect(q.answer).toBe("Next week");
  });
});

describe("ownerAskTurnsToVisitorQuestions", () => {
  it("maps each turn", () => {
    const turns: OwnerAskTurn[] = [
      {
        id: "turn-1",
        session_id: "sess-1",
        link_id: "link-1",
        visitor_id: "visitor-1",
        question: "Hello",
        lane: "host",
        status: "host_pending",
        host_question_id: "q-1",
        created_at: "2026-01-01T00:00:00Z",
        updated_at: "2026-01-01T00:00:00Z",
      },
    ];
    expect(ownerAskTurnsToVisitorQuestions(turns)).toHaveLength(1);
    expect(ownerAskTurnsToVisitorQuestions(turns)[0].ask_turn_id).toBe("turn-1");
  });
});

describe("answerOwnerAskQuestion", () => {
  beforeEach(() => {
    vi.mocked(api.answerAskTurn).mockReset();
  });

  it("uses answerAskTurn with ask_turn_id", async () => {
    const question: VisitorQuestion = {
      id: "q-1",
      ask_turn_id: "turn-1",
      link_id: "link-1",
      visitor_id: "visitor-1",
      question: "Hello",
      status: "pending",
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    };
    vi.mocked(api.answerAskTurn).mockResolvedValue({
      data: {
        id: "turn-1",
        session_id: "sess-1",
        link_id: "link-1",
        visitor_id: "visitor-1",
        question: "Hello",
        lane: "host",
        status: "host_answered",
        host_question_id: "q-1",
        host_answer: "Reply",
        created_at: "2026-01-01T00:00:00Z",
        updated_at: "2026-01-02T00:00:00Z",
      },
    });

    const updated = await answerOwnerAskQuestion(question, "Reply");
    expect(api.answerAskTurn).toHaveBeenCalledWith("link-1", "turn-1", "Reply");
    expect(updated.status).toBe("answered");
    expect(updated.answer).toBe("Reply");
  });

  it("falls back to question id as turn id when ask_turn_id is absent", async () => {
    const question: VisitorQuestion = {
      id: "turn-legacy",
      link_id: "link-1",
      visitor_id: "visitor-1",
      question: "Hello",
      status: "pending",
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    };
    vi.mocked(api.answerAskTurn).mockResolvedValue({
      data: {
        id: "turn-legacy",
        session_id: "sess-1",
        link_id: "link-1",
        visitor_id: "visitor-1",
        question: "Hello",
        lane: "host",
        status: "host_answered",
        host_answer: "Reply",
        created_at: "2026-01-01T00:00:00Z",
        updated_at: "2026-01-02T00:00:00Z",
      },
    });

    await answerOwnerAskQuestion(question, "Reply");
    expect(api.answerAskTurn).toHaveBeenCalledWith("link-1", "turn-legacy", "Reply");
  });
});
