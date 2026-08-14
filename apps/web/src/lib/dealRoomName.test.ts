import { describe, expect, it } from "vitest";
import {
  dealRoomSlugFromName,
  latinSlugFromName,
  normalizeDealRoomName,
  validateDealRoomName,
} from "./dealRoomName";

describe("normalizeDealRoomName", () => {
  it("trims and collapses whitespace, including tab and newline", () => {
    expect(normalizeDealRoomName("  Acme\tSeries \n A  ")).toBe("Acme Series A");
    expect(normalizeDealRoomName("创业\u3000融资")).toBe("创业 融资");
    expect(normalizeDealRoomName("Acme\u0085Series")).toBe("Acme Series");
  });

  it("strips format controls and NFC-composes", () => {
    expect(normalizeDealRoomName("创\u200B业融资")).toBe("创业融资");
    expect(normalizeDealRoomName("e\u0301clat")).toBe("éclat");
  });
});

describe("validateDealRoomName", () => {
  it("requires a non-empty name", () => {
    expect(validateDealRoomName("")).toBe("empty");
    expect(validateDealRoomName("   ")).toBe("empty");
    expect(validateDealRoomName("\t\n")).toBe("empty");
  });

  it("rejects names that are too short or too long", () => {
    expect(validateDealRoomName("A")).toBe("short");
    expect(validateDealRoomName("创")).toBe("short");
    expect(validateDealRoomName("A".repeat(81))).toBe("long");
    expect(validateDealRoomName("融".repeat(81))).toBe("long");
    expect(validateDealRoomName("x".repeat(4097))).toBe("long");
  });

  it("rejects punctuation-only names and angle brackets", () => {
    expect(validateDealRoomName("--")).toBe("format");
    expect(validateDealRoomName("...")).toBe("format");
    expect(validateDealRoomName("A < B")).toBe("format");
    expect(validateDealRoomName("A > B")).toBe("format");
    expect(validateDealRoomName("A ＜ B")).toBe("format");
    expect(validateDealRoomName("A ＞ B")).toBe("format");
  });

  it("accepts bilingual titles", () => {
    expect(validateDealRoomName("创业融资")).toBeNull();
    expect(validateDealRoomName("《投资备忘录》")).toBeNull();
    expect(validateDealRoomName("Acme Series A (Q2)")).toBeNull();
    expect(validateDealRoomName("M&A — Project Phoenix")).toBeNull();
    expect(validateDealRoomName("红杉 x Acme")).toBeNull();
    expect(validateDealRoomName("Q2.2026")).toBeNull();
    expect(validateDealRoomName("Seed-Round")).toBeNull();
    expect(validateDealRoomName("100% Club")).toBeNull();
  });
});

describe("dealRoomSlugFromName", () => {
  it("uses a latin slug when the name has two or more ascii letters or digits", () => {
    expect(latinSlugFromName("Seed-Round")).toBe("seed-round");
    expect(dealRoomSlugFromName("Seed-Round")).toBe("seed-round");
    expect(dealRoomSlugFromName("Acme Series A")).toBe("acme-series-a");
  });

  it("does not fall back to a template slug for CJK-only names", () => {
    const slug = dealRoomSlugFromName("创业融资");
    expect(slug).toMatch(/^room-[0-9a-f]{10}$/);
    expect(dealRoomSlugFromName("创业融资")).not.toBe(slug);
  });
});
