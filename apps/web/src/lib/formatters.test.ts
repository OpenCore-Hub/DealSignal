import { describe, it, expect, beforeAll } from "vitest";
import i18next from "i18next";
import { initReactI18next } from "react-i18next";
import zhCNFormatters from "../i18n/locales/zh-CN/formatters.json";
import {
  formatFileSize,
  formatDuration,
  formatRelativeTime,
  formatCompactNumber,
  formatCountWithPlural,
  getInitials,
} from "./formatters";

beforeAll(async () => {
  await i18next.use(initReactI18next).init({
    lng: "zh-CN",
    fallbackLng: "zh-CN",
    resources: {
      "zh-CN": { formatters: zhCNFormatters },
    },
  });
});

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
    expect(formatDuration(45)).toBe("45 秒");
  });

  it("formats minutes and seconds", () => {
    expect(formatDuration(125)).toBe("2 分 5 秒");
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

describe("formatCompactNumber", () => {
  it("formats small numbers as-is", () => {
    expect(formatCompactNumber(42, "en")).toBe("42");
  });

  it("formats thousands with K", () => {
    expect(formatCompactNumber(1500, "en")).toBe("1.5K");
  });

  it("formats millions with M", () => {
    expect(formatCompactNumber(2500000, "en")).toBe("2.5M");
  });

  it("formats chinese compact numbers", () => {
    expect(formatCompactNumber(15000, "zh-CN")).toMatch(/万/);
  });
});

describe("formatCountWithPlural", () => {
  const t = (key: string, options?: { count?: number }) => {
    const count = options?.count ?? 0;
    if (key === "views") return count === 1 ? `${count} view` : `${count} views`;
    if (key === "links") return count === 1 ? `${count} link` : `${count} links`;
    return String(count);
  };

  it("preserves singular suffix for count of 1", () => {
    expect(formatCountWithPlural(t, "views", 1, "en")).toBe("1 view");
  });

  it("uses compact notation while keeping plural suffix", () => {
    expect(formatCountWithPlural(t, "views", 1500, "en")).toBe("1.5K views");
  });

  it("uses compact notation for millions", () => {
    expect(formatCountWithPlural(t, "views", 2500000, "en")).toBe("2.5M views");
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
