// @vitest-environment jsdom
import { describe, expect, it, vi, beforeEach, beforeAll } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { createTestI18n } from "@/i18n/test-utils";
import enDocuments from "@/i18n/locales/en/documents.json";
import { PublicViewerThumbnailRail } from "./PublicViewerThumbnailRail";

describe("PublicViewerThumbnailRail", () => {
  let i18n: Awaited<ReturnType<typeof createTestI18n>>;

  beforeAll(async () => {
    i18n = await createTestI18n({
      documents: enDocuments as unknown as Record<string, string>,
    });
  });

  beforeEach(() => {
    vi.clearAllMocks();
  });

  function renderRail(props: React.ComponentProps<typeof PublicViewerThumbnailRail>) {
    return render(
      <I18nextProvider i18n={i18n}>
        <PublicViewerThumbnailRail {...props} />
      </I18nextProvider>,
    );
  }

  it("renders page preview images when thumbnail URLs are available", () => {
    const { container } = renderRail({
      pages: [{ pageNumber: 1 }, { pageNumber: 2 }],
      currentPage: 1,
      thumbnailUrls: {
        1: "https://example.test/page-1.png",
        2: "https://example.test/page-2.png",
      },
      onSelect: vi.fn(),
    });

    const images = container.querySelectorAll("img");
    expect(images).toHaveLength(2);
    expect(images[0]).toHaveAttribute("src", "https://example.test/page-1.png");
    expect(images[1]).toHaveAttribute("src", "https://example.test/page-2.png");
  });

  it("shows loading placeholders without inline page numbers inside thumbnails", () => {
    const { container } = renderRail({
      pages: [{ pageNumber: 3 }],
      currentPage: 3,
      onSelect: vi.fn(),
    });

    expect(container.querySelector("img")).toBeNull();
    expect(screen.getByRole("button", { name: "Page 3" })).toBeInTheDocument();
  });

  it("calls onSelect when a thumbnail is clicked", async () => {
    const onSelect = vi.fn();
    renderRail({
      pages: [{ pageNumber: 1 }, { pageNumber: 2 }],
      currentPage: 1,
      thumbnailUrls: { 1: "https://example.test/page-1.png" },
      onSelect,
    });

    screen.getByRole("button", { name: "Page 2" }).click();
    await waitFor(() => {
      expect(onSelect).toHaveBeenCalledWith(2);
    });
  });

  it("uses the page aspect ratio when dimensions are available", () => {
    const { container } = renderRail({
      pages: [{ pageNumber: 1, width: 420, height: 297 }],
      currentPage: 1,
      onSelect: vi.fn(),
    });

    const frame = container.querySelector('[data-testid="public-thumbnail-frame"]');
    expect(frame).toHaveStyle({ aspectRatio: "420 / 297" });
  });
});
