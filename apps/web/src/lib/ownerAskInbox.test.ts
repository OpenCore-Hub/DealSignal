import { describe, expect, it } from "vitest";
import {
  attachOwnerAskRepeatCounts,
  countSimilarAskQuestions,
  countOwnerAskPendingAttention,
  countUnreadOwnerAskTurns,
  matchesOwnerAskInboxFilter,
  ownerAskTurnIsFormalDegraded,
  moveOwnerAskPinnedFAQ,
  normalizeAskQuestionKey,
  ownerAskFaqReorderEnabled,
  ownerAskInboxUsesPinnedFAQApi,
  ownerAskInboxQuery,
  parseOwnerAskInboxView,
  ownerAskTurnCanPinFAQ,
  ownerAskTurnCanUnpinFAQ,
  ownerAskTurnNeedsHostReply,
  ownerAskTurnSuggestPinFAQ,
  sortOwnerAskPinnedFAQs,
} from "./ownerAskInbox";
import type { OwnerAskTurn } from "@/types";

describe("parseOwnerAskInboxView", () => {
  it("accepts known inbox views", () => {
    expect(parseOwnerAskInboxView("formal_queue")).toBe("formal_queue");
  });

  it("rejects unknown values", () => {
    expect(parseOwnerAskInboxView("legacy_questions")).toBeUndefined();
    expect(parseOwnerAskInboxView(null)).toBeUndefined();
  });
});

describe("ownerAskInboxQuery", () => {
  it("maps needs_host to host pending", () => {
    expect(ownerAskInboxQuery("needs_host")).toEqual({
      lane: "host",
      status: "host_pending",
    });
  });

  it("maps formal_queue to formal_queue status", () => {
    expect(ownerAskInboxQuery("formal_queue")).toEqual({ status: "formal_queue" });
  });

  it("maps ai_handled to ai answered", () => {
    expect(ownerAskInboxQuery("ai_handled")).toEqual({
      lane: "ai",
      status: "ai_answered",
    });
  });

  it("uses pinned FAQ API for pinned_faq view", () => {
    expect(ownerAskInboxUsesPinnedFAQApi("pinned_faq")).toBe(true);
    expect(ownerAskInboxUsesPinnedFAQApi("all")).toBe(false);
  });
});

describe("countUnreadOwnerAskTurns", () => {
  it("counts host_pending and host_escalated turns", () => {
    const turns = [
      { status: "host_pending" },
      { status: "host_escalated" },
      { status: "host_answered" },
      { status: "ai_answered" },
    ] as OwnerAskTurn[];
    expect(countUnreadOwnerAskTurns(turns)).toBe(2);
  });
});

describe("countOwnerAskPendingAttention", () => {
  it("sums needs-host replies and active formal queue", () => {
    const host = [
      { lane: "host", status: "host_pending" },
      { lane: "host", status: "host_answered" },
    ] as OwnerAskTurn[];
    const formal = [
      { formal_status: "pending_review", route_reason: "policy_formal" },
      { formal_status: "published", route_reason: "policy_formal" },
    ] as OwnerAskTurn[];
    expect(countOwnerAskPendingAttention(host, formal)).toBe(2);
  });
});

describe("ownerAskTurnIsFormalDegraded", () => {
  it("detects formal_not_entitled route reason", () => {
    expect(
      ownerAskTurnIsFormalDegraded({
        route_reason: "formal_not_entitled",
      } as OwnerAskTurn),
    ).toBe(true);
    expect(
      ownerAskTurnIsFormalDegraded({
        route_reason: "policy_formal",
      } as OwnerAskTurn),
    ).toBe(false);
  });
});

describe("matchesOwnerAskInboxFilter", () => {
  it("includes hybrid host_pending in needs_host view", () => {
    const turn = {
      lane: "hybrid",
      status: "host_pending",
    } as OwnerAskTurn;
    expect(matchesOwnerAskInboxFilter(turn, "host", "host_pending")).toBe(true);
  });

  it("includes hybrid host_escalated in needs_host view", () => {
    const turn = {
      lane: "hybrid",
      status: "host_escalated",
    } as OwnerAskTurn;
    expect(matchesOwnerAskInboxFilter(turn, "host", "host_pending")).toBe(true);
  });

  it("excludes formal queue from needs_host and includes in formal_queue", () => {
    const formalTurn = {
      lane: "host",
      status: "host_pending",
      route_reason: "policy_formal",
      formal_status: "pending_review",
    } as OwnerAskTurn;
    expect(matchesOwnerAskInboxFilter(formalTurn, "host", "host_pending")).toBe(false);
    expect(matchesOwnerAskInboxFilter(formalTurn, "", "formal_queue")).toBe(true);
  });

  it("excludes published formal from formal_queue tab", () => {
    const publishedFormal = {
      lane: "host",
      status: "host_answered",
      route_reason: "policy_formal",
      formal_status: "published",
      host_answer: "Guidance is $42M ARR.",
    } as OwnerAskTurn;
    expect(matchesOwnerAskInboxFilter(publishedFormal, "", "formal_queue")).toBe(false);
  });

  it("excludes hybrid pending from ai_handled view", () => {
    const turn = {
      lane: "hybrid",
      status: "host_pending",
    } as OwnerAskTurn;
    expect(matchesOwnerAskInboxFilter(turn, "ai", "ai_answered")).toBe(false);
  });

  it("excludes FAQ replay from needs_host and ai_handled", () => {
    const turn = {
      lane: "ai",
      status: "ai_answered",
      route_reason: "pinned_faq",
    } as OwnerAskTurn;
    expect(matchesOwnerAskInboxFilter(turn, "ai", "ai_answered")).toBe(false);
    expect(matchesOwnerAskInboxFilter(turn, "host", "host_pending")).toBe(false);
    expect(matchesOwnerAskInboxFilter(turn, "", "")).toBe(true);
  });
});

describe("ownerAskTurnNeedsHostReply", () => {
  it("detects host pending turns", () => {
    const turn = {
      lane: "host",
      status: "host_pending",
    } as OwnerAskTurn;
    expect(ownerAskTurnNeedsHostReply(turn)).toBe(true);
  });

  it("detects hybrid escalated turns", () => {
    const turn = {
      lane: "hybrid",
      status: "host_escalated",
    } as OwnerAskTurn;
    expect(ownerAskTurnNeedsHostReply(turn)).toBe(true);
  });
});

describe("ownerAskTurnCanPinFAQ", () => {
  it("allows ai answered turns with answer text", () => {
    const turn = {
      status: "ai_answered",
      ai_payload: { answer: "yes", refused: false, resultStatus: "answered" },
    } as OwnerAskTurn;
    expect(ownerAskTurnCanPinFAQ(turn)).toBe(true);
  });

  it("rejects already pinned turns", () => {
    const turn = {
      status: "ai_answered",
      pinned_faq_at: "2026-01-01T00:00:00Z",
      ai_payload: { answer: "yes", refused: false, resultStatus: "answered" },
    } as OwnerAskTurn;
    expect(ownerAskTurnCanPinFAQ(turn)).toBe(false);
  });

  it("rejects refused AI answers even when answer text is present", () => {
    const turn = {
      status: "ai_answered",
      ai_payload: { answer: "cannot share", refused: true, resultStatus: "refused" },
    } as OwnerAskTurn;
    expect(ownerAskTurnCanPinFAQ(turn)).toBe(false);
  });

  it("allows host-answered hybrid turns whose AI payload was refused", () => {
    const turn = {
      status: "host_answered",
      host_answer: "暂不公开",
      ai_payload: { answer: "", refused: true, resultStatus: "refused" },
    } as OwnerAskTurn;
    expect(ownerAskTurnCanPinFAQ(turn)).toBe(true);
  });
});

describe("ownerAskTurnCanUnpinFAQ", () => {
  it("detects pinned turns", () => {
    const turn = {
      pinned_faq_at: "2026-01-01T00:00:00Z",
    } as OwnerAskTurn;
    expect(ownerAskTurnCanUnpinFAQ(turn)).toBe(true);
  });
});

describe("sortOwnerAskPinnedFAQs", () => {
  it("orders by pinned_faq_sort ascending", () => {
    const turns = [
      { id: "b", pinned_faq_at: "2026-01-02T00:00:00Z", pinned_faq_sort: 1 },
      { id: "a", pinned_faq_at: "2026-01-01T00:00:00Z", pinned_faq_sort: 0 },
    ] as OwnerAskTurn[];
    expect(sortOwnerAskPinnedFAQs(turns).map((t) => t.id)).toEqual(["a", "b"]);
  });
});

describe("moveOwnerAskPinnedFAQ", () => {
  it("swaps adjacent pinned FAQs", () => {
    const turns = [
      { id: "a", pinned_faq_at: "2026-01-01T00:00:00Z", pinned_faq_sort: 0 },
      { id: "b", pinned_faq_at: "2026-01-02T00:00:00Z", pinned_faq_sort: 1 },
    ] as OwnerAskTurn[];
    const moved = moveOwnerAskPinnedFAQ(turns, "b", "up");
    expect(moved.map((t) => t.id)).toEqual(["b", "a"]);
    expect(moved.map((t) => t.pinned_faq_sort)).toEqual([0, 1]);
  });
});

describe("ownerAskFaqReorderEnabled", () => {
  it("enables reorder on link pinned_faq view", () => {
    expect(ownerAskFaqReorderEnabled("pinned_faq", { type: "link" })).toBe(true);
  });

  it("requires single-link filter in room scope", () => {
    expect(ownerAskFaqReorderEnabled("pinned_faq", { type: "room", linkFilter: "all" })).toBe(
      false,
    );
    expect(ownerAskFaqReorderEnabled("pinned_faq", { type: "room", linkFilter: "link_1" })).toBe(
      true,
    );
  });
});

describe("normalizeAskQuestionKey", () => {
  it("normalizes casing and punctuation", () => {
    expect(normalizeAskQuestionKey("  What is NDA?  ")).toBe("what is nda");
  });
});

describe("countSimilarAskQuestions", () => {
  it("counts same-link repeats only", () => {
    const turns = [
      { id: "1", link_id: "l1", question: "What is NDA?" },
      { id: "2", link_id: "l1", question: "what is nda" },
      { id: "3", link_id: "l2", question: "what is nda" },
    ] as OwnerAskTurn[];
    expect(countSimilarAskQuestions(turns, turns[0])).toBe(2);
  });
});

describe("attachOwnerAskRepeatCounts", () => {
  it("sets repeat_count per link and normalized question", () => {
    const turns = [
      { id: "1", link_id: "l1", question: "What is NDA?" },
      { id: "2", link_id: "l1", question: "what is nda" },
      { id: "3", link_id: "l2", question: "what is nda" },
    ] as OwnerAskTurn[];
    const out = attachOwnerAskRepeatCounts(turns);
    expect(out[0].repeat_count).toBe(2);
    expect(out[2].repeat_count).toBe(1);
  });
});

describe("ownerAskTurnSuggestPinFAQ", () => {
  it("suggests pin when repeat_count meets threshold", () => {
    const turn = {
      id: "1",
      link_id: "l1",
      question: "pricing?",
      status: "ai_answered",
      repeat_count: 3,
      ai_payload: { answer: "x", refused: false, resultStatus: "answered" },
    } as OwnerAskTurn;
    expect(ownerAskTurnSuggestPinFAQ(turn)).toBe(true);
  });

  it("does not suggest when repeat_count is below threshold", () => {
    const turn = {
      id: "1",
      link_id: "l1",
      question: "pricing?",
      status: "ai_answered",
      repeat_count: 2,
      ai_payload: { answer: "x", refused: false, resultStatus: "answered" },
    } as OwnerAskTurn;
    expect(ownerAskTurnSuggestPinFAQ(turn)).toBe(false);
  });
});
