import { describe, it, expect } from "vitest";
import {
  formatFileSize,
  formatDuration,
  formatRelativeTime,
  getInitials,
} from "./formatters";

describe("formatFileSize", () => {
  it("returns 0 B for zero bytes", () => {
    expect(formatFileSize(0)).toBe("0 B");
  });

  it("formats bytes", () => {
    expect(formatFileSize(512)).toBe("512 B");
  });

  it("formats kilobytes", () => {
    expect(formatFileSize(1536)).toBe("1.5 KB");
  });

  it("formats megabytes", () => {
    expect(formatFileSize(2 * 1024 * 1024)).toBe("2 MB");
  });
});

describe("formatDuration", () => {
  it("returns - for zero or negative seconds", () => {
    expect(formatDuration(0)).toBe("-");
    expect(formatDuration(-10)).toBe("-");
  });

  it("formats seconds only", () => {
    expect(formatDuration(45)).toBe("45s");
  });

  it("formats minutes and seconds", () => {
    expect(formatDuration(125)).toBe("2m 5s");
  });
});

describe("formatRelativeTime", () => {
  it("returns - for undefined date", () => {
    expect(formatRelativeTime(undefined)).toBe("-");
  });

  it("returns 刚刚 for recent timestamps", () => {
    const now = new Date().toISOString();
    expect(formatRelativeTime(now)).toBe("刚刚");
  });

  it("returns minutes ago", () => {
    const date = new Date(Date.now() - 3 * 60 * 1000).toISOString();
    expect(formatRelativeTime(date)).toBe("3 分钟前");
  });

  it("returns days ago", () => {
    const date = new Date(Date.now() - 2 * 24 * 60 * 60 * 1000).toISOString();
    expect(formatRelativeTime(date)).toBe("2 天前");
  });
});

describe("getInitials", () => {
  it("returns uppercase initials", () => {
    expect(getInitials("john doe")).toBe("JD");
  });

  it("limits to two characters", () => {
    expect(getInitials("one two three")).toBe("OT");
  });
});
