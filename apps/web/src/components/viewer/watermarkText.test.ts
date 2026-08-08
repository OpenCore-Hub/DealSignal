import { describe, expect, it } from "vitest";
import {
  buildDynamicWatermarkText,
  formatWatermarkTimestamp,
  normalizeWatermarkTimestamp,
  parseWatermarkText,
} from "./watermarkText";

describe("formatWatermarkTimestamp", () => {
  it("formats world-unified UTC (not local timezone)", () => {
    expect(formatWatermarkTimestamp(new Date("2026-08-08T01:23:45.678Z"))).toBe(
      "2026-08-08 01:23:45 UTC",
    );
  });
});

describe("normalizeWatermarkTimestamp", () => {
  it("keeps UTC stamps and upgrades legacy RFC3339 Z", () => {
    expect(normalizeWatermarkTimestamp("2026-08-08 01:23:45 UTC")).toBe(
      "2026-08-08 01:23:45 UTC",
    );
    expect(normalizeWatermarkTimestamp("2026-08-08T01:23:45Z")).toBe(
      "2026-08-08 01:23:45 UTC",
    );
    expect(normalizeWatermarkTimestamp("not-a-time")).toBeNull();
  });
});

describe("parseWatermarkText", () => {
  it("parses server UTC watermark payloads", () => {
    expect(
      parseWatermarkText(
        "visitor@example.com | 2026-08-08 01:00:00 UTC | IP:abcd1234",
      ),
    ).toEqual({
      email: "visitor@example.com",
      ip: "abcd1234",
      viewedAt: "2026-08-08 01:00:00 UTC",
    });
  });

  it("parses legacy RFC3339 Z payloads into UTC stamps", () => {
    expect(
      parseWatermarkText("visitor@example.com | 2026-08-08T01:00:00Z | IP:abcd1234"),
    ).toEqual({
      email: "visitor@example.com",
      ip: "abcd1234",
      viewedAt: "2026-08-08 01:00:00 UTC",
    });
  });

  it("rejects non-dynamic payloads", () => {
    expect(parseWatermarkText("CONFIDENTIAL")).toBeNull();
    expect(parseWatermarkText("a | b")).toBeNull();
    expect(parseWatermarkText("")).toBeNull();
  });
});

describe("buildDynamicWatermarkText", () => {
  const now = new Date("2026-08-08T12:34:56.000Z");
  const opts = {
    fallback: "CONFIDENTIAL",
    previewIp: "preview",
  };

  it("composes structured email/ip with world-unified UTC", () => {
    expect(
      buildDynamicWatermarkText(
        { email: "visitor@example.com", ip: "abcd1234" },
        now,
        opts,
      ),
    ).toBe("visitor@example.com | 2026-08-08 12:34:56 UTC | IP:abcd1234");
  });

  it("keeps the server-issued UTC stamp (world clock) over client now", () => {
    expect(
      buildDynamicWatermarkText(
        {
          watermarkText:
            "visitor@example.com | 2026-08-08 01:00:00 UTC | IP:abcd1234",
        },
        now,
        opts,
      ),
    ).toBe("visitor@example.com | 2026-08-08 01:00:00 UTC | IP:abcd1234");
  });

  it("uses login account email for owner viewer preview", () => {
    expect(
      buildDynamicWatermarkText({ email: "owner@example.com" }, now, opts),
    ).toBe("owner@example.com | 2026-08-08 12:34:56 UTC | IP:preview");
  });

  it("does not forge an identity when login email is unavailable", () => {
    expect(buildDynamicWatermarkText({}, now, opts)).toBe("CONFIDENTIAL");
  });

  it("returns fallback when watermark is undefined", () => {
    expect(buildDynamicWatermarkText(undefined, now, opts)).toBe("CONFIDENTIAL");
  });
});
