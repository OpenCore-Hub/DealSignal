// @vitest-environment jsdom
import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { createTestI18n } from "@/i18n/test-utils";
import { DocumentPageGrid } from "./DocumentPageGrid";
import { PAGE_GRID_VIRTUALIZE_THRESHOLD, buildPageGridItems } from "@/lib/projectPageGrid";
import type { PageAnalytics } from "@/types";

vi.mock("./PageCard", () => ({
  PageCard: ({ pageNumber }: { pageNumber: number }) => (
    <div data-testid={`page-card-${pageNumber}`}>page {pageNumber}</div>
  ),
}));

function analytics(count: number): PageAnalytics[] {
  return Array.from({ length: count }, (_, i) => ({
    pageNumber: i + 1,
    viewCount: 1,
    avgDurationSeconds: 2,
    exitRate: 0,
  }));
}

async function renderGrid(pageCount: number) {
  const i18n = await createTestI18n({
    documents: {
      "content.virtualizedHint": "{{count}} pages · scroll to load more thumbnails",
    },
  });
  const items = buildPageGridItems(pageCount, analytics(pageCount));
  return render(
    <I18nextProvider i18n={i18n}>
      <DocumentPageGrid
        items={items}
        documentId="doc-1"
        selectedPage={null}
        onSelectPage={() => undefined}
      />
    </I18nextProvider>,
  );
}

describe("DocumentPageGrid", () => {
  beforeEach(() => {
    Object.defineProperty(HTMLElement.prototype, "clientWidth", {
      configurable: true,
      get() {
        return 1280;
      },
    });
    Object.defineProperty(HTMLElement.prototype, "clientHeight", {
      configurable: true,
      get() {
        return 800;
      },
    });
    Object.defineProperty(HTMLElement.prototype, "offsetHeight", {
      configurable: true,
      get() {
        return 220;
      },
    });
  });

  it("renders all cards without virtualization under the threshold", async () => {
    await renderGrid(PAGE_GRID_VIRTUALIZE_THRESHOLD);
    const root = screen.getByTestId("document-page-grid");
    expect(root).toHaveAttribute("data-virtualized", "false");
    expect(screen.getAllByTestId(/page-card-/)).toHaveLength(PAGE_GRID_VIRTUALIZE_THRESHOLD);
    expect(screen.queryByTestId("document-page-grid-hint")).not.toBeInTheDocument();
  });

  it("virtualizes long documents and mounts only a window of cards", async () => {
    const count = PAGE_GRID_VIRTUALIZE_THRESHOLD + 80;
    await renderGrid(count);
    const root = screen.getByTestId("document-page-grid");
    expect(root).toHaveAttribute("data-virtualized", "true");
    expect(screen.getByTestId("document-page-grid-hint")).toHaveTextContent(
      `${count} pages · scroll to load more thumbnails`,
    );
    const mounted = screen.getAllByTestId(/page-card-/).length;
    expect(mounted).toBeGreaterThan(0);
    expect(mounted).toBeLessThan(count);
  });
});
