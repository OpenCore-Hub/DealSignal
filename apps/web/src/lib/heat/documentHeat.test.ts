import { describe, it, expect } from "vitest";
import { documentHeatFromLinks, overlayDocumentNativeHeat } from "./documentHeat";
import type { HeatLevel, Link } from "@/types";

function link(partial: Partial<Link> & Pick<Link, "id" | "heatLevel">): Link {
  return {
    documentId: "doc_1",
    documentIds: ["doc_1"],
    folderPaths: [],
    documentTitle: "Deck",
    shortUrl: "https://example.com/l/x",
    accessCount: 0,
    createdAt: "2026-06-01T00:00:00Z",
    isBundle: false,
    documents: [],
    ...partial,
  };
}

describe("documentHeatFromLinks", () => {
  it("returns cold when there are no links", () => {
    expect(documentHeatFromLinks([])).toBe("cold");
  });

  it("uses the hottest link heat.Compute level", () => {
    expect(
      documentHeatFromLinks([
        link({ id: "l1", heatLevel: "cold" }),
        link({ id: "l2", heatLevel: "warm" }),
        link({ id: "l3", heatLevel: "hot" }),
      ]),
    ).toBe("hot");
  });

  it("does not use accessCount thresholds", () => {
    expect(
      documentHeatFromLinks([
        link({ id: "l1", heatLevel: "cold", accessCount: 100 }),
      ]),
    ).toBe("cold");
  });
});

describe("overlayDocumentNativeHeat", () => {
  function row(partial: { id: string; heatLevel?: HeatLevel; totalViews?: number; links?: string[] }) {
    return {
      id: partial.id,
      heatLevel: partial.heatLevel ?? "cold",
      totalViews: partial.totalViews ?? 0,
      links: partial.links ?? [],
    };
  }

  it("is a no-op when scores are empty", () => {
    const rows = [row({ id: "doc_1", heatLevel: "warm", totalViews: 4 })];
    expect(overlayDocumentNativeHeat(rows, [])).toBe(rows);
  });

  it("overlays heat and page views without attaching shares", () => {
    const rows = [row({ id: "doc_1", heatLevel: "cold", totalViews: 0, links: [] })];
    const out = overlayDocumentNativeHeat(rows, [
      { id: "doc_1", views: 12, heatLevel: "hot" },
    ]);
    expect(out[0]).toEqual({
      id: "doc_1",
      heatLevel: "hot",
      totalViews: 12,
      links: [],
    });
  });

  it("keeps library-link fallback when the file has no native score", () => {
    const rows = [row({ id: "doc_1", heatLevel: "warm", totalViews: 5 })];
    const out = overlayDocumentNativeHeat(rows, [
      { id: "doc_other", views: 99, heatLevel: "hot" },
    ]);
    expect(out[0]).toEqual(rows[0]);
  });
});
