// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  coalesceSecurityEvents,
  evidenceEmptyPrimaryKey,
  gateTimelineSummary,
  topPagesSpanMultipleDocuments,
} from "./radarEvidencePresentation";

const events = [
  {
    eventType: "security_gate_failed",
    reason: "email_code_required",
    createdAt: "2026-08-11T10:23:33Z",
  },
  {
    eventType: "security_gate_failed",
    reason: "email_code_required",
    createdAt: "2026-08-11T10:23:27Z",
  },
  {
    eventType: "security_gate_failed",
    reason: "email_code_required",
    createdAt: "2026-08-11T10:23:23Z",
  },
  {
    eventType: "security_gate_failed",
    reason: "email_code_required",
    createdAt: "2026-08-11T10:14:46Z",
  },
];

describe("coalesceSecurityEvents", () => {
  it("merges identical gate failures and keeps newest lastAt", () => {
    const groups = coalesceSecurityEvents(events);
    expect(groups).toHaveLength(1);
    expect(groups[0]?.count).toBe(4);
    expect(groups[0]?.lastAt).toBe("2026-08-11T10:23:33Z");
    expect(groups[0]?.firstAt).toBe("2026-08-11T10:14:46Z");
  });

  it("keeps distinct reasons separate", () => {
    const groups = coalesceSecurityEvents([
      ...events.slice(0, 1),
      {
        eventType: "security_gate_failed",
        reason: "nda_required",
        createdAt: "2026-08-11T10:20:00Z",
      },
    ]);
    expect(groups).toHaveLength(2);
  });
});

describe("evidenceEmptyPrimaryKey", () => {
  it("hides empty copy when metrics are active", () => {
    expect(
      evidenceEmptyPrimaryKey("leak_watch", {
        metricsActive: true,
        hasSecurityEvents: false,
      }),
    ).toBeNull();
  });

  it("uses product empty copy for leak without metrics or events", () => {
    expect(
      evidenceEmptyPrimaryKey("leak_watch", {
        metricsActive: false,
        hasSecurityEvents: false,
      }),
    ).toBe("radar.evidenceRail.emptyPrimary.leak_watch");
  });

  it("prefers security events over empty metrics copy for risk products", () => {
    expect(
      evidenceEmptyPrimaryKey("leak_watch", {
        metricsActive: false,
        hasSecurityEvents: true,
      }),
    ).toBeNull();
  });
});

describe("gateTimelineSummary", () => {
  it("splits hits before and after the access request", () => {
    const summary = gateTimelineSummary(events, "2026-08-11T10:15:22Z");
    expect(summary).toEqual({
      kind: "before_and_after",
      before: 1,
      after: 3,
      total: 4,
    });
  });

  it("handles request-only-after pattern", () => {
    const summary = gateTimelineSummary(events.slice(0, 3), "2026-08-11T10:15:22Z");
    expect(summary).toEqual({ kind: "after_only", after: 3, total: 3 });
  });
});

describe("topPagesSpanMultipleDocuments", () => {
  it("is false for a single document or missing ids", () => {
    expect(topPagesSpanMultipleDocuments([{ documentId: "doc-1" }, { documentId: "doc-1" }])).toBe(
      false,
    );
    expect(topPagesSpanMultipleDocuments([{ pageNumber: 1 } as { documentId?: string }])).toBe(
      false,
    );
  });

  it("is true when pages come from more than one document", () => {
    expect(
      topPagesSpanMultipleDocuments([{ documentId: "doc-xlsx" }, { documentId: "doc-pdf" }]),
    ).toBe(true);
  });
});
