import { describe, expect, it } from "vitest";
import enLinkShare from "@/i18n/locales/en/linkShare.json";
import zhLinkShare from "@/i18n/locales/zh-CN/linkShare.json";

describe("Share link Ask Host i18n (deal-room owner surfaces)", () => {
  it("keeps askSecurityEvents owner panel keys in en and zh-CN", () => {
    expect(enLinkShare.askSecurityEvents.title).toBeTruthy();
    expect(zhLinkShare.askSecurityEvents.title).toBeTruthy();
    expect(enLinkShare.askSecurityEvents.description).toBeTruthy();
    expect(zhLinkShare.askSecurityEvents.description).toBeTruthy();
    for (const key of [
      "rate_limit_exceeded",
      "scope_violation",
      "blocked_email",
      "blocked_domain",
      "not_in_allow_list",
    ] as const) {
      expect(enLinkShare.askSecurityEvents.eventTypes[key], `en missing ${key}`).toBeTruthy();
      expect(zhLinkShare.askSecurityEvents.eventTypes[key], `zh-CN missing ${key}`).toBeTruthy();
    }
  });

  it("separates Ask management copy from Signal inbox (B7)", () => {
    expect(enLinkShare.management.questionsTitle).toMatch(/Ask inbox/i);
    expect(zhLinkShare.management.questionsTitle).toContain("Ask");
    expect(enLinkShare.management.questionsDescription).toMatch(/not the Signal inbox/i);
    expect(zhLinkShare.management.questionsDescription).toContain("信号");

    expect(enLinkShare.analytics.qaRecords).toMatch(/Ask activity/i);
    expect(zhLinkShare.analytics.qaRecords).toContain("Ask");
  });
});
