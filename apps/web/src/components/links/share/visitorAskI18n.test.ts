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
});
