import { describe, expect, it } from "vitest";
import { documentAvgDurationSeconds } from "./documentAvgDuration";
import type { Link, PageAnalytics } from "@/types";

function link(partial: Partial<Link> & Pick<Link, "id">): Link {
  return {
    documentIds: [],
    folderPaths: [],
    documentTitle: "Doc",
    shortUrl: "https://example.com/l/x",
    accessCount: 0,
    heatLevel: "cold",
    createdAt: "2026-08-01T00:00:00Z",
    isBundle: false,
    documents: [],
    ...partial,
  };
}

const bundleDocs = [
  { id: "doc_xlsx", title: "Model.xlsx", sourceType: "xlsx" as const, pageCount: 3, status: "ready" as const },
  { id: "doc_pdf", title: "Memo.pdf", sourceType: "pdf" as const, pageCount: 16, status: "ready" as const },
];

describe("documentAvgDurationSeconds", () => {
  it("keeps the mean of link averages for solo shares", () => {
    expect(
      documentAvgDurationSeconds([
        link({ id: "a", avgDurationSeconds: 10 }),
        link({ id: "b", avgDurationSeconds: 30 }),
      ]),
    ).toBe(20);
  });

  it("uses attributed page analytics when pages have views", () => {
    const pages: PageAnalytics[] = [
      { pageNumber: 1, viewCount: 10, avgDurationSeconds: 99, exitRate: 0 },
    ];
    expect(
      documentAvgDurationSeconds(
        [link({ id: "solo", avgDurationSeconds: 12 })],
        pages,
      ),
    ).toBe(99);
  });

  it("uses page analytics when there are no library shares", () => {
    const pages: PageAnalytics[] = [
      { pageNumber: 1, viewCount: 3, avgDurationSeconds: 20, exitRate: 0 },
    ];
    expect(documentAvgDurationSeconds([], pages)).toBe(20);
  });

  it("uses attributed page analytics when the document is only in bundles", () => {
    const pages: PageAnalytics[] = [
      { pageNumber: 8, viewCount: 2, avgDurationSeconds: 30, exitRate: 0 },
      { pageNumber: 16, viewCount: 2, avgDurationSeconds: 50, exitRate: 0 },
    ];
    expect(
      documentAvgDurationSeconds(
        [
          link({
            id: "bundle",
            isBundle: true,
            avgDurationSeconds: 12,
            documents: bundleDocs,
          }),
        ],
        pages,
      ),
    ).toBe(40);
  });

  it("uses attributed page analytics when a solo share is mixed with a bundle", () => {
    const pages: PageAnalytics[] = [
      { pageNumber: 8, viewCount: 2, avgDurationSeconds: 30, exitRate: 0 },
      { pageNumber: 16, viewCount: 2, avgDurationSeconds: 50, exitRate: 0 },
    ];
    expect(
      documentAvgDurationSeconds(
        [
          link({ id: "solo", avgDurationSeconds: 12 }),
          link({
            id: "bundle",
            isBundle: true,
            avgDurationSeconds: 99,
            documents: bundleDocs,
          }),
        ],
        pages,
      ),
    ).toBe(40);
  });

  it("does not leak bundle-wide link duration when page analytics are empty", () => {
    expect(
      documentAvgDurationSeconds(
        [
          link({
            id: "bundle",
            isBundle: true,
            avgDurationSeconds: 18,
            documents: bundleDocs,
          }),
        ],
        [{ pageNumber: 1, viewCount: 0, avgDurationSeconds: 0, exitRate: 0 }],
      ),
    ).toBe(0);
  });
});
