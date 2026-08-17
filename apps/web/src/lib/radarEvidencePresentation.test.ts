// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  coalesceSecurityEvents,
  evidenceEmptyPrimaryKey,
  gateTimelineI18nKey,
  gateTimelineSummary,
  isRadarGateHoldItem,
  radarRowIdentities,
  topPagesSpanMultipleDocuments,
} from "./radarEvidencePresentation";
describe("radarRowIdentities", () => {
  it("keeps a named actor on warm cards", () => {
    expect(
      radarRowIdentities({
        product: "buying_window",
        actor: "张姐",
      }),
    ).toEqual({ primary: "张姐", shareContact: null });
  });

  it("does not treat a contact name as the person held at the gate", () => {
    expect(
      radarRowIdentities({
        product: "diligence_gate",
        actor: "张姐",
      }),
    ).toEqual({ primary: null, shareContact: "张姐" });
  });

  it("keeps an email-shaped actor on a waiting-to-enter card", () => {
    expect(
      radarRowIdentities({
        product: "diligence_gate",
        actor: "yqx-401@126.com",
      }),
    ).toEqual({ primary: "yqx-401@126.com", shareContact: null });
  });

  it("names a distinct share-contact email beside the person held", () => {
    expect(
      radarRowIdentities({
        product: "diligence_gate",
        actor: "yqx-401@126.com",
        contactEmail: "zhang@share.example",
      }),
    ).toEqual({
      primary: "yqx-401@126.com",
      shareContact: "zhang@share.example",
    });
  });
});

describe("isRadarGateHoldItem", () => {
  it("is true only for a waiting-to-enter hold", () => {
    expect(
      isRadarGateHoldItem({ product: "diligence_gate", verb: "review" }),
    ).toBe(true);
    expect(
      isRadarGateHoldItem({ product: "diligence_gate", verb: "approve" }),
    ).toBe(false);
    expect(isRadarGateHoldItem({ product: "diligence_gate" })).toBe(false);
    expect(
      isRadarGateHoldItem({ product: "leak_watch", verb: "review" }),
    ).toBe(false);
  });
});

const promptEvents = [
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

const holdEvents = promptEvents.map((e) => ({
  ...e,
  eventType: "not_in_allow_list",
  reason: undefined,
}));

describe("coalesceSecurityEvents", () => {
  it("merges identical gate failures and keeps newest lastAt", () => {
    const groups = coalesceSecurityEvents(promptEvents);
    expect(groups).toHaveLength(1);
    expect(groups[0]?.count).toBe(4);
    expect(groups[0]?.lastAt).toBe("2026-08-11T10:23:33Z");
    expect(groups[0]?.firstAt).toBe("2026-08-11T10:14:46Z");
  });

  it("keeps distinct reasons separate", () => {
    const groups = coalesceSecurityEvents([
      ...promptEvents.slice(0, 1),
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
  it("splits holds before and after the access request", () => {
    const summary = gateTimelineSummary(holdEvents, "2026-08-11T10:15:22Z");
    expect(summary).toEqual({
      kind: "before_and_after",
      before: 1,
      after: 3,
      total: 4,
    });
  });

  it("handles request-only-after pattern", () => {
    const summary = gateTimelineSummary(holdEvents.slice(0, 3), "2026-08-11T10:15:22Z");
    expect(summary).toEqual({ kind: "after_only", after: 3, total: 3 });
  });

  it("ignores empty-form prompts when counting holds", () => {
    expect(gateTimelineSummary(promptEvents, "2026-08-11T10:15:22Z")).toBeNull();
    const mixed = [
      ...promptEvents,
      {
        eventType: "not_in_allow_list",
        createdAt: "2026-08-11T10:23:40Z",
      },
    ];
    expect(gateTimelineSummary(mixed, "2026-08-11T10:15:22Z")).toEqual({
      kind: "after_only",
      after: 1,
      total: 1,
    });
  });
});

describe("gateTimelineI18nKey", () => {
  it("uses pending copy when the request is still waiting", () => {
    expect(
      gateTimelineI18nKey({ kind: "after_only", after: 1, total: 1 }, "pending"),
    ).toBe("radar.evidenceRail.gateTimeline.afterOnlyPending");
    expect(
      gateTimelineI18nKey(
        { kind: "before_and_after", before: 1, after: 2, total: 3 },
        "pending",
      ),
    ).toBe("radar.evidenceRail.gateTimeline.beforeAndAfterPending");
    expect(
      gateTimelineI18nKey({ kind: "events_only", total: 2 }, "pending"),
    ).toBe("radar.evidenceRail.gateTimeline.eventsOnlyPending");
    expect(
      gateTimelineI18nKey({ kind: "before_only", before: 2, total: 2 }, "pending"),
    ).toBe("radar.evidenceRail.gateTimeline.beforeOnlyPending");
  });

  it("does not claim pending or a fault when status is unknown", () => {
    expect(
      gateTimelineI18nKey({ kind: "after_only", after: 1, total: 1 }, "approved"),
    ).toBe("radar.evidenceRail.gateTimeline.afterOnly");
    expect(gateTimelineI18nKey({ kind: "events_only", total: 2 })).toBe(
      "radar.evidenceRail.gateTimeline.eventsOnly",
    );
    expect(
      gateTimelineI18nKey(
        { kind: "before_and_after", before: 1, after: 2, total: 3 },
        "approved",
      ),
    ).toBe("radar.evidenceRail.gateTimeline.beforeAndAfter");
    expect(
      gateTimelineI18nKey({ kind: "before_only", before: 2, total: 2 }),
    ).toBe("radar.evidenceRail.gateTimeline.beforeOnly");
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
