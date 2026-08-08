/** @vitest-environment jsdom */
import { describe, expect, it } from "vitest";
import {
  countRadarFilters,
  defaultOutcomeForProduct,
  filterRadarItems,
  filterServerStrands,
  flatRadarOrder,
  groupIntoStrands,
  isEditableKeyboardTarget,
  outcomesForProduct,
  parseRadarCircle,
  parseRadarFilter,
  radarHeadlineKey,
  radarWhyNowFallbackKey,
  radarWhyNowKey,
  withMailtoHrefs,
  type RadarStrand,
  type RadarWorkItem,
} from "./radarQueue";

const baseItem = (over: Partial<RadarWorkItem>): RadarWorkItem => ({
  id: "a1",
  product: "buying_window",
  headline: "Reach out",
  subtitle: "Hot",
  verb: "email",
  priority: "high",
  slaDueAt: "2026-06-21T18:00:00Z",
  createdAt: "2026-06-20T18:00:00Z",
  dealKey: "link:l1",
  dealName: "Deck link",
  actionId: "a1",
  contactEmail: "a@example.com",
  documentTitle: "Deck",
  ...over,
});

describe("radarQueue", () => {
  it("parses product filters and legacy aliases", () => {
    expect(parseRadarFilter("commitment_ask")).toBe("commitment_ask");
    expect(parseRadarFilter("follow_up")).toBe("buying_window");
    expect(parseRadarFilter("ask")).toBe("commitment_ask");
    expect(parseRadarFilter("approve")).toBe("diligence_gate");
    expect(parseRadarFilter("risk")).toBe("leak_watch");
    expect(parseRadarFilter("nope")).toBe("all");
  });

  it("filters by product", () => {
    const items = [
      baseItem({ id: "1", product: "buying_window" }),
      baseItem({ id: "2", product: "diligence_gate", verb: "approve" }),
    ];
    expect(filterRadarItems(items, "diligence_gate")).toHaveLength(1);
    expect(countRadarFilters(items).all).toBe(2);
    expect(countRadarFilters(items).buying_window).toBe(1);
  });

  it("builds mailto hrefs for email verbs", () => {
    const [item] = withMailtoHrefs([baseItem({})], {
      subject: (d) => `Follow-up: ${d}`,
      body: ({ email }) => `Hi ${email}`,
    });
    expect(item.mailtoHref).toContain("mailto:a@example.com");
    expect(item.mailtoHref).toContain("Follow-up");
  });

  it("groups items into deal strands", () => {
    const strands = groupIntoStrands([
      baseItem({ id: "1", dealKey: "link:a", dealName: "A" }),
      baseItem({ id: "2", dealKey: "link:b", dealName: "B" }),
      baseItem({ id: "3", dealKey: "link:a", dealName: "A" }),
    ]);
    expect(strands).toHaveLength(2);
    expect(strands[0].items).toHaveLength(2);
    expect(strands[1].dealKey).toBe("link:b");
  });

  it("filters server strands without regrouping order", () => {
    const strands: RadarStrand[] = [
      {
        dealKey: "link:a",
        dealName: "A",
        items: [
          baseItem({ id: "1", product: "buying_window", dealKey: "link:a" }),
          baseItem({
            id: "2",
            product: "diligence_gate",
            verb: "approve",
            dealKey: "link:a",
          }),
        ],
      },
      {
        dealKey: "link:b",
        dealName: "B",
        items: [
          baseItem({ id: "3", product: "leak_watch", verb: "review", dealKey: "link:b" }),
        ],
      },
    ];
    const filtered = filterServerStrands(strands, "diligence_gate", "1");
    expect(filtered).toHaveLength(1);
    expect(filtered[0].dealKey).toBe("link:a");
    expect(filtered[0].items.map((i) => i.id)).toEqual(["2"]);
    expect(flatRadarOrder(baseItem({ id: "n" }), filtered).map((i) => i.id)).toEqual([
      "n",
      "2",
    ]);
  });

  it("parses circle lens and default outcomes", () => {
    expect(parseRadarCircle("sales")).toBe("sales");
    expect(parseRadarCircle("nope")).toBe("founder");
    expect(defaultOutcomeForProduct("diligence_gate")).toBe("approved");
    expect(outcomesForProduct("leak_watch")).toEqual([
      "acted",
      "false_positive",
    ]);
  });

  it("builds scenario-specific whyNow and headline i18n keys", () => {
    expect(
      radarWhyNowKey({
        scenario: "sales-dataroom",
        whyNowCode: "buying_window",
      }),
    ).toBe("radar.scenario.sales-dataroom.whyNow.buying_window");
    expect(
      radarWhyNowKey({
        scenario: "startup-fundraising",
        whyNowCode: "diligence_gate",
      }),
    ).toBe("radar.scenario.startup-fundraising.whyNow.diligence_gate");
    expect(radarWhyNowFallbackKey({ whyNowCode: "buying_window" })).toBe(
      "radar.whyNow.buying_window",
    );
    expect(radarWhyNowKey({ whyNowCode: "buying_window" })).toBe(
      "radar.whyNow.buying_window",
    );
    expect(
      radarHeadlineKey({
        scenario: "sales-dataroom",
        headlineCode: "follow_warm_buyer",
      }),
    ).toBe("radar.scenario.sales-dataroom.headline.follow_warm_buyer");
    expect(
      radarHeadlineKey({
        scenario: "startup-fundraising",
        headlineCode: "unlock_investor_gate",
      }),
    ).toBe("radar.scenario.startup-fundraising.headline.unlock_investor_gate");
    expect(
      radarHeadlineKey({
        scenario: "real-estate-transaction",
        headlineCode: "unlock_counterparty_gate",
      }),
    ).toBe(
      "radar.scenario.real-estate-transaction.headline.unlock_counterparty_gate",
    );
    expect(
      radarHeadlineKey({
        scenario: "project-management",
        headlineCode: "answer_project_ask",
      }),
    ).toBe("radar.scenario.project-management.headline.answer_project_ask");
    expect(radarHeadlineKey({ headlineCode: "x" })).toBe("");
  });

  it("detects editable keyboard targets for shortcut gating", () => {
    const input = document.createElement("input");
    const div = document.createElement("div");
    expect(isEditableKeyboardTarget(input)).toBe(true);
    expect(isEditableKeyboardTarget(div)).toBe(false);
  });
});
