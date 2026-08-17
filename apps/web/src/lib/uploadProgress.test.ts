import { describe, expect, it } from "vitest";
import {
  UPLOAD_TRANSFER_MAX,
  ingestionBarPercent,
  transferBarPercent,
} from "./uploadProgress";

describe("transferBarPercent", () => {
  it("maps loaded/total onto the transfer slice, not the full bar", () => {
    expect(transferBarPercent(0, 1000)).toBe(0);
    expect(transferBarPercent(500, 1000)).toBe(20);
    expect(transferBarPercent(1000, 1000)).toBe(UPLOAD_TRANSFER_MAX);
  });

  it("returns null when total is unknown so callers do not invent a percent", () => {
    expect(transferBarPercent(100, 0)).toBeNull();
    expect(transferBarPercent(100, Number.NaN)).toBeNull();
    expect(transferBarPercent(Number.NaN, 0)).toBeNull();
  });
});

describe("ingestionBarPercent", () => {
  it("uses the same 25 / 50 / 100 steps as the documents list", () => {
    expect(ingestionBarPercent(25, 0)).toBe(25);
    expect(ingestionBarPercent(50, 0)).toBe(50);
    expect(ingestionBarPercent(100, 0)).toBe(100);
  });

  it("uses status when the payload omits progress (same steps as documentProgress)", () => {
    expect(ingestionBarPercent(undefined, 0, "processing")).toBe(50);
    expect(ingestionBarPercent(undefined, 0, "ready")).toBe(100);
  });

  it("does not keep a high transfer leftover (stuck-at-90)", () => {
    expect(ingestionBarPercent(50, 90, "processing")).toBe(50);
    expect(ingestionBarPercent(undefined, 90, "processing")).toBe(50);
  });

  it("can keep a real transfer floor that is still below the processing step", () => {
    expect(ingestionBarPercent(25, 40)).toBe(40);
    expect(ingestionBarPercent(50, 40)).toBe(50);
  });
});
