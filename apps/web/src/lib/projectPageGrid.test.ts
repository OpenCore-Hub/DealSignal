import { describe, expect, it } from "vitest";
import {
  PAGE_GRID_VIRTUALIZE_THRESHOLD,
  buildPageGridItems,
  columnCountForWidth,
  estimatePageCardRowHeight,
  pageGridRowCount,
  shouldVirtualizePageGrid,
} from "./projectPageGrid";
import type { PageAnalytics } from "@/types";

const analytics: PageAnalytics[] = [
  { pageNumber: 2, viewCount: 5, avgDurationSeconds: 12, exitRate: 0.2 },
  { pageNumber: 1, viewCount: 1, avgDurationSeconds: 3, exitRate: 0.1 },
];

describe("projectPageGrid", () => {
  it("builds O(1)-indexed items for every page", () => {
    const items = buildPageGridItems(3, analytics);
    expect(items).toHaveLength(3);
    expect(items[0]).toMatchObject({ pageNumber: 1, viewCount: 1 });
    expect(items[1]).toMatchObject({ pageNumber: 2, viewCount: 5, exitRate: 0.2 });
    expect(items[2]).toMatchObject({ pageNumber: 3, viewCount: 0, avgDurationSeconds: 0 });
  });

  it("maps widths to the same column breakpoints as the CSS grid", () => {
    expect(columnCountForWidth(375)).toBe(2);
    expect(columnCountForWidth(640)).toBe(3);
    expect(columnCountForWidth(768)).toBe(4);
    expect(columnCountForWidth(1024)).toBe(5);
    expect(columnCountForWidth(1280)).toBe(6);
  });

  it("virtualizes only above the threshold", () => {
    expect(shouldVirtualizePageGrid(PAGE_GRID_VIRTUALIZE_THRESHOLD)).toBe(false);
    expect(shouldVirtualizePageGrid(PAGE_GRID_VIRTUALIZE_THRESHOLD + 1)).toBe(true);
  });

  it("estimates positive row height and row counts", () => {
    expect(estimatePageCardRowHeight(720, 4)).toBeGreaterThan(100);
    expect(pageGridRowCount(50, 5)).toBe(10);
    expect(pageGridRowCount(0, 5)).toBe(0);
  });
});
