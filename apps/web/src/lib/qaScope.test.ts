import { describe, expect, it } from "vitest";
import { qaEnabledForLink, qaEnabledForLinkId } from "./qaScope";

describe("qaScope", () => {
  it("enables visitor Ask only for deal-room share links", () => {
    expect(qaEnabledForLink(false)).toBe(false);
    expect(qaEnabledForLink(true)).toBe(true);
  });

  it("derives scope from deal room id", () => {
    expect(qaEnabledForLinkId(undefined)).toBe(false);
    expect(qaEnabledForLinkId("")).toBe(false);
    expect(qaEnabledForLinkId("room-1")).toBe(true);
  });
});
