import { describe, expect, it } from "vitest";
import {
  filterLinksForShareView,
  hasActiveShareListFilters,
  parseLinkCreatedWithin,
} from "./shareLinksFilter";
import type { Link } from "@/types";

function link(partial: Partial<Link> & Pick<Link, "id" | "createdAt">): Link {
  return {
    documentId: "doc_1",
    documentIds: ["doc_1"],
    folderPaths: [],
    documentTitle: "Pitch Deck",
    shortUrl: "https://example.test/l/abc",
    accessCount: 0,
    heatLevel: "cold",
    isBundle: false,
    documents: [],
    ...partial,
  };
}

describe("shareLinksFilter", () => {
  const now = Date.parse("2026-08-11T12:00:00Z");
  const rows = [
    link({
      id: "hour",
      createdAt: "2026-08-11T06:00:00Z",
      documentTitle: "Fresh Deck",
      shortUrl: "https://example.test/l/hour",
    }),
    link({
      id: "new",
      createdAt: "2026-08-09T12:00:00Z",
      documentTitle: "Alpha Deck",
      shortUrl: "https://example.test/l/new",
    }),
    link({
      id: "mid",
      createdAt: "2026-07-20T12:00:00Z",
      documentTitle: "Beta Model",
      shortUrl: "https://example.test/l/mid",
      name: "partner-share",
    }),
    link({
      id: "old",
      createdAt: "2026-04-01T12:00:00Z",
      documentTitle: "Gamma Report",
      shortUrl: "https://example.test/l/old",
    }),
  ];

  it("parses create-time tokens and rejects legacy bare numbers", () => {
    expect(parseLinkCreatedWithin("7d")).toBe("7d");
    expect(parseLinkCreatedWithin("7")).toBe("all");
    expect(parseLinkCreatedWithin(null)).toBe("all");
  });

  it("detects active list filters", () => {
    expect(hasActiveShareListFilters({ searchQuery: "", createdWithin: "all" })).toBe(false);
    expect(hasActiveShareListFilters({ searchQuery: "deck", createdWithin: "all" })).toBe(true);
    expect(hasActiveShareListFilters({ searchQuery: "  ", createdWithin: "24h" })).toBe(true);
  });

  it("matches secondary titles on a bundle share", () => {
    const bundle = link({
      id: "bundle",
      createdAt: "2026-08-11T06:00:00Z",
      documentTitle: "Model.xlsx",
      isBundle: true,
      documents: [
        { id: "doc_xlsx", title: "Model.xlsx", sourceType: "xlsx", pageCount: 3, status: "ready" },
        { id: "doc_pdf", title: "Memo.pdf", sourceType: "pdf", pageCount: 16, status: "ready" },
      ],
    });
    expect(filterLinksForShareView([bundle], { searchQuery: "memo" }).map((l) => l.id)).toEqual([
      "bundle",
    ]);
  });

  it("filters by search query across title, url, and name", () => {
    expect(filterLinksForShareView(rows, { searchQuery: "beta", nowMs: now }).map((l) => l.id)).toEqual([
      "mid",
    ]);
    expect(
      filterLinksForShareView(rows, { searchQuery: "partner-share", nowMs: now }).map((l) => l.id),
    ).toEqual(["mid"]);
    expect(filterLinksForShareView(rows, { searchQuery: "/l/old", nowMs: now }).map((l) => l.id)).toEqual([
      "old",
    ]);
  });

  it("filters by rolling create-time windows", () => {
    expect(
      filterLinksForShareView(rows, { createdWithin: "24h", nowMs: now }).map((l) => l.id),
    ).toEqual(["hour"]);
    expect(
      filterLinksForShareView(rows, { createdWithin: "7d", nowMs: now }).map((l) => l.id),
    ).toEqual(["hour", "new"]);
    expect(
      filterLinksForShareView(rows, { createdWithin: "30d", nowMs: now }).map((l) => l.id),
    ).toEqual(["hour", "new", "mid"]);
    expect(
      filterLinksForShareView(rows, { createdWithin: "all", nowMs: now }).map((l) => l.id),
    ).toEqual(["hour", "new", "mid", "old"]);
  });

  it("applies search and create-time together", () => {
    expect(
      filterLinksForShareView(rows, {
        searchQuery: "deck",
        createdWithin: "7d",
        nowMs: now,
      }).map((l) => l.id),
    ).toEqual(["hour", "new"]);
  });
});
