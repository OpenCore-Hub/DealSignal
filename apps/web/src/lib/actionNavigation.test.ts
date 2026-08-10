import { describe, expect, it } from "vitest";
import {
  actionNavigatePath,
  formalAskSuggestionPath,
  isFormalAskReviewAction,
} from "./actionNavigation";

describe("actionNavigatePath", () => {
  it("routes document share access requests to Document Library Share only", () => {
    expect(
      actionNavigatePath("acme", {
        sourceType: "link_access_request",
        sourceId: "link-doc",
        actionType: "approve",
      }),
    ).toBe("/acme/documents?tab=shared&linkId=link-doc");
  });

  it("routes deal-room share access requests to Deal Room Access only", () => {
    expect(
      actionNavigatePath("acme", {
        sourceType: "deal_room_link_access_request",
        sourceId: "link-room",
        targetId: "room-1",
        actionType: "approve",
      }),
    ).toBe("/acme/deal-rooms/room-1?tab=access&linkId=link-room");
  });

  it("refuses deal-room share navigation without targetId (no document fallback)", () => {
    expect(
      actionNavigatePath("acme", {
        sourceType: "deal_room_link_access_request",
        sourceId: "link-room",
        actionType: "approve",
      }),
    ).toBeNull();
  });

  it("routes room membership / NDA todos by room id to Access", () => {
    expect(
      actionNavigatePath("acme", {
        sourceType: "room_access_request",
        sourceId: "room-1",
        actionType: "approve",
      }),
    ).toBe("/acme/deal-rooms/room-1?tab=access");

    expect(
      actionNavigatePath("acme", {
        sourceType: "room_nda",
        sourceId: "member-1",
        targetId: "room-1",
        actionType: "sign",
      }),
    ).toBe("/acme/deal-rooms/room-1?tab=access");

    // Legacy room-keyed NDA rows (pre member-keying).
    expect(
      actionNavigatePath("acme", {
        sourceType: "room_nda",
        sourceId: "room-1",
        actionType: "sign",
      }),
    ).toBe("/acme/deal-rooms/room-1?tab=access");
  });

  it("does not send document share todos into deal rooms", () => {
    const path = actionNavigatePath("acme", {
      sourceType: "link_access_request",
      sourceId: "link-doc",
      targetId: "room-should-be-ignored",
      actionType: "approve",
    });
    expect(path).toContain("/documents?");
    expect(path).not.toContain("/deal-rooms/");
  });

  it("routes deal-room visitor Ask todos to QA inbox with link filter", () => {
    expect(
      actionNavigatePath("acme", {
        sourceType: "deal_room_link_question",
        sourceId: "question-1",
        targetId: "room-1/link-room",
        actionType: "answer",
      }),
    ).toBe("/acme/deal-rooms/room-1?tab=qa&linkId=link-room");
  });

  it("routes formal review todos to the formal queue inbox tab", () => {
    expect(
      actionNavigatePath("acme", {
        sourceType: "deal_room_link_question",
        sourceId: "turn-formal-1",
        targetId: "room-1/link-room",
        actionType: "review",
      }),
    ).toBe("/acme/deal-rooms/room-1?tab=qa&linkId=link-room&askInbox=formal_queue");
  });

  it("routes document-library Ask todos to link detail Ask inbox", () => {
    expect(
      actionNavigatePath("acme", {
        sourceType: "link_question",
        sourceId: "turn-1",
        targetId: "link-lib",
        actionType: "answer",
      }),
    ).toBe("/acme/links/link-lib?askInbox=needs_host");
  });

  it("routes document-library formal review todos to formal queue", () => {
    expect(
      actionNavigatePath("acme", {
        sourceType: "link_question",
        sourceId: "turn-formal",
        targetId: "link-lib",
        actionType: "review",
      }),
    ).toBe("/acme/links/link-lib?askInbox=formal_queue");
  });

  it("refuses link_question rows without targetId", () => {
    expect(
      actionNavigatePath("acme", {
        sourceType: "link_question",
        sourceId: "question-legacy",
        actionType: "answer",
      }),
    ).toBeNull();
  });

  it("does not route deal-room Ask todos to document share surfaces", () => {
    const path = actionNavigatePath("acme", {
      sourceType: "deal_room_link_question",
      sourceId: "question-1",
      targetId: "room-1/link-room",
      actionType: "answer",
    });
    expect(path).toContain("/deal-rooms/room-1?tab=qa");
    expect(path).not.toContain("/documents");
    expect(path).not.toMatch(/\/links\//);
  });
});

describe("isFormalAskReviewAction", () => {
  it("matches formal review todos on deal-room and library Ask", () => {
    expect(
      isFormalAskReviewAction({ sourceType: "deal_room_link_question", actionType: "review" }),
    ).toBe(true);
    expect(
      isFormalAskReviewAction({ sourceType: "deal_room_link_question", actionType: "answer" }),
    ).toBe(false);
    expect(isFormalAskReviewAction({ sourceType: "link_question", actionType: "review" })).toBe(
      true,
    );
  });
});

describe("formalAskSuggestionPath", () => {
  it("routes deal-room Formal Ask suggestions to room QA formal queue", () => {
    expect(
      formalAskSuggestionPath("acme", { linkId: "link-room", dealRoomId: "room-1" }),
    ).toBe("/acme/deal-rooms/room-1?tab=qa&linkId=link-room&askInbox=formal_queue");
  });

  it("routes library Formal Ask suggestions to link Ask formal queue", () => {
    expect(formalAskSuggestionPath("acme", { linkId: "link-lib" })).toBe(
      "/acme/links/link-lib?askInbox=formal_queue",
    );
  });

  it("refuses empty linkId", () => {
    expect(formalAskSuggestionPath("acme", { linkId: "" })).toBeNull();
  });
});
