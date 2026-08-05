// @vitest-environment jsdom
import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { createTestI18n } from "@/i18n/test-utils";
import { DocumentAnalytics } from "./DocumentAnalytics";
import type { PageAnalytics } from "@/types";

function pages(count: number, engaged: (n: number) => boolean = () => true): PageAnalytics[] {
  return Array.from({ length: count }, (_, i) => {
    const pageNumber = i + 1;
    const on = engaged(pageNumber);
    return {
      pageNumber,
      viewCount: on ? 1 : 0,
      avgDurationSeconds: on ? 10 + pageNumber : 0,
      exitRate: 0,
    };
  });
}

async function renderAnalytics(
  analytics: PageAnalytics[],
  variant: "overview" | "detail" = "detail",
  onOpenPage?: (pageNumber: number) => void,
) {
  const i18n = await createTestI18n({
    documents: {
      "analytics.title": "Time on page",
      "analytics.emptyTitle": "Empty",
      "analytics.emptyDescription": "No data",
      "analytics.pageTooltip": "Page {{pageNumber}} · {{seconds}}s",
      "analytics.bucketTooltip": "Pages {{start}}–{{end}} · peak {{seconds}}s",
      "analytics.firstPage": "Page 1",
      "analytics.lastPage": "Page {{count}}",
      "analytics.bucketedHint": "{{count}} ranges",
      "analytics.hiddenZeros": "{{count}} hidden",
      "analytics.filterAll": "All pages",
      "analytics.filterEngaged": "With dwell",
      "analytics.filterEmptyTitle": "No dwell",
      "analytics.filterEmptyDescription": "Switch filter",
      "analytics.expandPerPage": "Expand {{count}} pages",
      "analytics.collapsePerPage": "Show ranges",
      "analytics.focusRange": "Pages {{start}}–{{end}}",
      "analytics.clearFocus": "All pages",
      "analytics.drillHint": "Click a range to inspect pages inside it",
      "analytics.drillBucket": "Inspect pages {{start}}–{{end}}",
      "analytics.openPageHint": "Click a page to open Content",
      "analytics.openPage": "Open page {{pageNumber}}",
      "analytics.topPages": "Longest dwell",
      "analytics.topPageLabel": "Page {{pageNumber}}",
      "analytics.topPageSeconds": "{{seconds}}s",
    },
  });
  return render(
    <I18nextProvider i18n={i18n}>
      <DocumentAnalytics analytics={analytics} variant={variant} onOpenPage={onOpenPage} />
    </I18nextProvider>,
  );
}

describe("DocumentAnalytics", () => {
  it("overview buckets dense documents and shows top pages", async () => {
    await renderAnalytics(pages(120), "overview");
    expect(screen.getByTestId("document-analytics")).toHaveAttribute("data-strategy", "bucketed");
    expect(screen.getByText("Longest dwell")).toBeInTheDocument();
  });

  it("detail defaults to engaged filter when most pages are zero", async () => {
    await renderAnalytics(pages(80, (n) => n <= 4), "detail");
    expect(screen.getByTestId("document-analytics")).toHaveAttribute("data-strategy", "engaged");
    expect(screen.getByText("76 hidden")).toBeInTheDocument();
  });

  it("detail expands bucketed long docs to per-page scroll", async () => {
    await renderAnalytics(pages(160), "detail");
    const root = screen.getByTestId("document-analytics");
    expect(root).toHaveAttribute("data-strategy", "bucketed");
    fireEvent.click(screen.getByTestId("document-analytics-expand"));
    expect(root).toHaveAttribute("data-strategy", "scroll");
    expect(screen.getByText("Show ranges")).toBeInTheDocument();
  });

  it("clicking a bucket drills into that page range", async () => {
    await renderAnalytics(pages(160), "detail");
    const root = screen.getByTestId("document-analytics");
    expect(root).toHaveAttribute("data-strategy", "bucketed");
    expect(screen.getByText("Click a range to inspect pages inside it")).toBeInTheDocument();

    const bucket = screen.getAllByTestId(/document-analytics-bucket-/).at(0);
    expect(bucket).toBeTruthy();
    fireEvent.click(bucket!);

    expect(root).toHaveAttribute("data-focus-start");
    expect(root).toHaveAttribute("data-focus-end");
    expect(root.getAttribute("data-strategy")).not.toBe("bucketed");
    expect(screen.getByTestId("document-analytics-clear-focus")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("document-analytics-clear-focus"));
    expect(root).not.toHaveAttribute("data-focus-start");
    expect(root).toHaveAttribute("data-strategy", "bucketed");
  });

  it("opens a page from the longest-dwell list when onOpenPage is provided", async () => {
    const onOpenPage = vi.fn();
    await renderAnalytics(pages(20), "overview", onOpenPage);
    const top = screen.getByTestId("document-analytics-top-page-20");
    fireEvent.click(top);
    expect(onOpenPage).toHaveBeenCalledWith(20);
  });

  it("opens Content directly when a single-page bucket is clicked", async () => {
    const onOpenPage = vi.fn();
    // 49 pages → bucket size 2; final bucket is page 49 alone (start===end).
    await renderAnalytics(pages(49), "overview", onOpenPage);
    expect(screen.getByTestId("document-analytics")).toHaveAttribute(
      "data-strategy",
      "bucketed",
    );
    fireEvent.click(screen.getByTestId("document-analytics-bucket-49-49"));
    expect(onOpenPage).toHaveBeenCalledWith(49);
  });
});
