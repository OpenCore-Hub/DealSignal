/** @vitest-environment node */
import { describe, expect, it } from "vitest";
import {
  countRadarFilters,
  filterRadarItems,
  flatRadarOrder,
  groupIntoStrands,
  type RadarProduct,
  type RadarWorkItem,
} from "./radarQueue";

const PRODUCTS: RadarProduct[] = [
  "buying_window",
  "diligence_gate",
  "commitment_ask",
  "leak_watch",
  "access_decay",
  "abuse_guard",
];

function item(product: RadarProduct, i: number): RadarWorkItem {
  return {
    id: `${product}-${i}`,
    product,
    headline: `${product} ${i}`,
    subtitle: "",
    verb: "review",
    priority: "high",
    slaDueAt: "2026-08-09T00:00:00Z",
    createdAt: "2026-08-08T00:00:00Z",
    dealKey: `deal:${i % 25}`,
    dealName: `Deal ${i % 25}`,
    actionId: `${product}-${i}`,
  };
}

describe("radarQueue six-product stress", () => {
  it("counts and filters stay consistent under 1200 mixed items", () => {
    const items: RadarWorkItem[] = [];
    for (let i = 0; i < 1200; i += 1) {
      items.push(item(PRODUCTS[i % PRODUCTS.length], i));
    }
    const counts = countRadarFilters(items);
    expect(counts.all).toBe(1200);
    for (const p of PRODUCTS) {
      expect(counts[p]).toBe(200);
      expect(filterRadarItems(items, p)).toHaveLength(200);
    }
    const strands = groupIntoStrands(items);
    expect(strands.length).toBe(25);
    const flat = flatRadarOrder(undefined, strands);
    expect(flat).toHaveLength(1200);
    // Deterministic strand order by first-seen dealKey.
    expect(strands[0].dealKey).toBe("deal:0");
  });

  it("empty product buckets stay zero (boundary)", () => {
    const onlyGate = [item("diligence_gate", 0), item("diligence_gate", 1)];
    const counts = countRadarFilters(onlyGate);
    expect(counts.diligence_gate).toBe(2);
    expect(counts.buying_window).toBe(0);
    expect(counts.abuse_guard).toBe(0);
    expect(filterRadarItems(onlyGate, "abuse_guard")).toHaveLength(0);
  });
});
