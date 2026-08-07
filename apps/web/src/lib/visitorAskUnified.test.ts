import { describe, expect, it } from "vitest";
import { visitorAskUnifiedEnabled } from "./visitorAskUnified";

describe("visitorAskUnifiedEnabled", () => {
  it("requires qa enabled", () => {
    expect(visitorAskUnifiedEnabled({ qaEnabled: true, visitorAskUnified: true })).toBe(true);
    expect(visitorAskUnifiedEnabled({ qaEnabled: true, visitorAskUnified: false })).toBe(false);
    expect(visitorAskUnifiedEnabled({ qaEnabled: false, visitorAskUnified: true })).toBe(false);
  });

  it("defaults deal-room links to unified when flag omitted", () => {
    expect(
      visitorAskUnifiedEnabled({ qaEnabled: true, dealRoomId: "room-1" }),
    ).toBe(true);
    expect(visitorAskUnifiedEnabled({ qaEnabled: true })).toBe(false);
  });
});
