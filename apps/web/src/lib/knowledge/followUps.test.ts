import { describe, expect, it } from "vitest";
import { buildRoomFollowUps, followUpNeedsNarrowing } from "./followUps";

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

  it("anchors suggestions to a room source name", () => {
    const tips = buildRoomFollowUps({
      refused: false,
      resultStatus: "answered",
      hits: [{ sourceName: "NDA.pdf" }],
    });
    expect(tips).toHaveLength(3);
    expect(tips[0].params?.sourceName).toBe("NDA.pdf");
    expect(tips.every((t) => t.messageKey.startsWith("knowledge.followUp."))).toBe(true);
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
