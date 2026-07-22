import { describe, it, expect } from "vitest";
import {
  isAskDocsKnowledgeBaseReady,
  shouldBlockAskDocsForKnowledgeBase,
} from "./visitorAskKbGate";

describe("visitorAskKbGate", () => {
  it("treats ready and stale as Ask Docs capable", () => {
    expect(isAskDocsKnowledgeBaseReady("ready")).toBe(true);
    expect(isAskDocsKnowledgeBaseReady("stale")).toBe(true);
    expect(isAskDocsKnowledgeBaseReady("none")).toBe(false);
    expect(isAskDocsKnowledgeBaseReady("building")).toBe(false);
    expect(isAskDocsKnowledgeBaseReady("failed")).toBe(false);
  });

  it("blocks Ask Docs on deal-room links when KB is not ready/stale", () => {
    expect(shouldBlockAskDocsForKnowledgeBase(true, "none")).toBe(true);
    expect(shouldBlockAskDocsForKnowledgeBase(true, "ready")).toBe(false);
    expect(shouldBlockAskDocsForKnowledgeBase(false, "none")).toBe(false);
    expect(shouldBlockAskDocsForKnowledgeBase(true, undefined)).toBe(false);
  });
});
