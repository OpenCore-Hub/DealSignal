// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  computeMockLinkHeat,
  countLinksByHeatLevel,
  mockHeatInputFromLink,
  syncMockLinkHeatLevels,
} from "./mockHeat";
import type { Link } from "@/types";

function link(partial: Partial<Link> & Pick<Link, "id" | "accessCount">): Link {
  return {
    documentIds: [],
    folderPaths: [],
    shortUrl: "https://example.com/l/x",
    heatLevel: "cold",
    createdAt: "2026-01-01T00:00:00Z",
    isActive: true,
    isBundle: false,
    ...partial,
  } as Link;
}

describe("mockHeat", () => {
  it("does not invent forwardSignals (marker-only)", () => {
    const input = mockHeatInputFromLink({ accessCount: 20, avgDurationSeconds: 120 });
    expect(input.forwardSignals).toBe(0);
  });

  it("counts Chinese key-page titles using the same keyword rules", () => {
    const input = mockHeatInputFromLink(
      { accessCount: 4, avgDurationSeconds: 60 },
      [
        {
          pageNumber: 2,
          title: "财务模型",
          viewCount: 4,
          avgDurationSeconds: 20,
          exitRate: 0,
        },
      ],
    );
    expect(input.keyPageViews).toBeGreaterThan(0);
  });

  it("uses founder thresholds (hot≥75, warm≥40) via computeHeatScore", () => {
    const cold = computeMockLinkHeat({ accessCount: 0, avgDurationSeconds: 0 });
    expect(cold.level).toBe("cold");
    expect(cold.score).toBe(0);

    const engaged = computeMockLinkHeat({ accessCount: 12, avgDurationSeconds: 180 });
    expect(engaged.score).toBeGreaterThanOrEqual(40);
    expect(["warm", "hot"]).toContain(engaged.level);
  });

  it("syncs link.heatLevel from computed score, not alerts", () => {
    const links = [
      link({ id: "a", accessCount: 40, avgDurationSeconds: 200, heatLevel: "cold" }),
      link({ id: "b", accessCount: 0, avgDurationSeconds: 0, heatLevel: "hot" }),
    ];
    syncMockLinkHeatLevels(links);
    expect(links[0]!.heatLevel).not.toBe("cold");
    expect(links[1]!.heatLevel).toBe("cold");
    const counts = countLinksByHeatLevel(links);
    expect(counts.cold).toBe(1);
    expect(counts.hot + counts.warm).toBe(1);
  });

  it("never applies constant score inflation", () => {
    const a = computeMockLinkHeat({ accessCount: 5, avgDurationSeconds: 60 });
    const b = computeMockLinkHeat({ accessCount: 5, avgDurationSeconds: 60 });
    expect(a.score).toBe(b.score);
    expect(a.breakdown.opens).toBe(Math.min(5, 10) * 3);
  });

  it("dashboard-style tier counts ignore heatAlerts length", async () => {
    const { getMockDashboardStats, mockHeatAlerts, mockLinks } = await import("./data");
    const stats = getMockDashboardStats();
    const fromLinks = countLinksByHeatLevel(mockLinks);
    expect(stats.hotCount).toBe(fromLinks.hot);
    expect(stats.warmCount).toBe(fromLinks.warm);
    expect(stats.coldCount).toBe(fromLinks.cold);
    // Alerts are visitor-scoped — must not equal / inflate link tiers.
    expect(mockHeatAlerts.filter((a) => a.heatLevel === "hot").length).toBeGreaterThan(
      0,
    );
    expect(stats.hotCount).not.toBe(
      mockHeatAlerts.filter((a) => a.heatLevel === "hot").length,
    );
  });
});
