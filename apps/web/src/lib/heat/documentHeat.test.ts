import { describe, it, expect } from "vitest";
import { documentHeatFromLinks } from "./documentHeat";
import type { Link } from "@/types";

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
