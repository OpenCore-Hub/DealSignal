/** @vitest-environment node */
import { describe, expect, it } from "vitest";
import { buildRadarStressFeed, RADAR_STRESS_PRODUCTS } from "./radarStressFeed";
import { buildMockRadarEvidencePack } from "./data";

describe("buildRadarStressFeed", () => {
  it("emits balanced counts across all six products", () => {
    const feed = buildRadarStressFeed({ perProduct: 10 });
    expect(feed.counts.all).toBe(60);
    for (const p of RADAR_STRESS_PRODUCTS) {
      expect(feed.counts[p]).toBe(10);
    }
    expect(feed.strands?.length).toBeGreaterThan(1);
    expect(feed.noiseHints?.[0]?.product).toBe("leak_watch");
    expect(feed.nextUp).toBeTruthy();
  });

  it("builds diligence evidence with request + coalescable gate hits", () => {
    const feed = buildRadarStressFeed({ perProduct: 1 });
    const gate = feed.items.find((i) => i.product === "diligence_gate");
    expect(gate).toBeTruthy();
    const pack = buildMockRadarEvidencePack(gate!);
    expect(pack.accessRequest?.email).toBeTruthy();
    expect(pack.metrics.opens24h).toBe(0);
    expect(pack.securityEvents?.length).toBe(4);
  });
});
