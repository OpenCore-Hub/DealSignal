import { describe, expect, it } from "vitest";
import enDocuments from "@/i18n/locales/en/documents.json";
import zhDocuments from "@/i18n/locales/zh-CN/documents.json";
import enLinkShare from "@/i18n/locales/en/linkShare.json";
import zhLinkShare from "@/i18n/locales/zh-CN/linkShare.json";

const DOCUMENT_VIEWER_KEYS = [
  "sidebarQA",
  "qaModeAI",
  "qaModeOwner",
  "qaAIPlaceholder",
  "qaOwnerPlaceholder",
  "qaEmptyUnified",
  "qaNoEvidence",
  "qaSuggestAskHost",
  "qaSwitchToAskHost",
] as const;

const LINK_SHARE_ADVANCED_KEYS = [
  "visitorAsk",
  "visitorAskDescription",
  "askDocs",
  "askDocsDescription",
  "askHost",
  "askHostDescription",
  "knowledgeBaseRequired",
  "openKnowledgeBase",
  "askDocsScopeNotInKb",
  "askDocsScopeGaps",
] as const;

describe("Visitor Ask i18n parity", () => {
  it("keeps documents viewer Ask keys in en and zh-CN", () => {
    for (const key of DOCUMENT_VIEWER_KEYS) {
      expect(enDocuments.viewer[key], `en missing ${key}`).toBeTruthy();
      expect(zhDocuments.viewer[key], `zh-CN missing ${key}`).toBeTruthy();
    }
  });

  it("keeps linkShare advanced Visitor Ask keys in en and zh-CN", () => {
    for (const key of LINK_SHARE_ADVANCED_KEYS) {
      expect(enLinkShare.accessRules.advanced[key], `en missing ${key}`).toBeTruthy();
      expect(zhLinkShare.accessRules.advanced[key], `zh-CN missing ${key}`).toBeTruthy();
    }
  });

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

  it("separates Ask Host management copy from Ask Docs audit and Signal (B7)", () => {
    expect(enLinkShare.management.questionsTitle).toMatch(/Ask Host/i);
    expect(zhLinkShare.management.questionsTitle).toContain("问发起方");
    expect(enLinkShare.management.questionsDescription).toMatch(/not Ask Docs audit/i);
    expect(enLinkShare.management.questionsDescription).toMatch(/not the Signal inbox/i);
    expect(zhLinkShare.management.questionsDescription).toContain("问文档审计");
    expect(zhLinkShare.management.questionsDescription).toContain("信号");

    expect(enLinkShare.analytics.qaRecords).toMatch(/Ask Host/i);
    expect(zhLinkShare.analytics.qaRecords).toContain("问发起方");

    expect(enLinkShare.askDocsAudit.description).toMatch(/Not the Ask Host inbox/i);
    expect(enLinkShare.askDocsAudit.description).toMatch(/not the Signal inbox/i);
    expect(zhLinkShare.askDocsAudit.description).toContain("问发起方");
    expect(zhLinkShare.askDocsAudit.description).toContain("信号");
  });
});
