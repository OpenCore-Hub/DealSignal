// @vitest-environment jsdom
import { describe, expect, it, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { usePublicPageThumbnailUrls } from "./usePublicPageThumbnailUrls";
import { api } from "@/lib/api";

vi.mock("@/lib/api", () => ({
  api: {
    getPublicPageSignedUrl: vi.fn(),
  },
}));

describe("usePublicPageThumbnailUrls", () => {
  beforeEach(() => {
    vi.mocked(api.getPublicPageSignedUrl).mockReset();
    vi.mocked(api.getPublicPageSignedUrl).mockImplementation(async (_doc, _token, page) => ({
      page_number: page,
      image_url: `https://example.test/page-${page}.png`,
      expires_at: "2099-01-01T00:00:00Z",
      width: 612,
      height: 792,
    }));
  });

  it("seeds the current page URL without refetching it", async () => {
    const { result } = renderHook(() =>
      usePublicPageThumbnailUrls({
        documentId: "doc-1",
        publicToken: "tok-1",
        pageNumbers: [1, 2],
        currentPage: 1,
        seedUrls: { 1: "https://example.test/page-1-seeded.png" },
      }),
    );

    expect(result.current[1]).toBe("https://example.test/page-1-seeded.png");

    await waitFor(() => {
      expect(result.current[2]).toBe("https://example.test/page-2.png");
    });

    expect(api.getPublicPageSignedUrl).not.toHaveBeenCalledWith(
      "doc-1",
      "tok-1",
      1,
      undefined,
    );
  });

  it("loads all pages even when currentPage changes during fetch", async () => {
    let releasePage2!: () => void;
    const page2Gate = new Promise<void>((resolve) => {
      releasePage2 = resolve;
    });

    vi.mocked(api.getPublicPageSignedUrl).mockImplementation(async (_doc, _token, page) => {
      if (page === 2) await page2Gate;
      return {
        page_number: page,
        image_url: `https://example.test/page-${page}.png`,
        expires_at: "2099-01-01T00:00:00Z",
        width: 612,
        height: 792,
      };
    });

    const { result, rerender } = renderHook(
      (props: { currentPage: number }) =>
        usePublicPageThumbnailUrls({
          documentId: "doc-1",
          publicToken: "tok-1",
          pageNumbers: [1, 2, 3, 4, 5, 6],
          currentPage: props.currentPage,
          seedUrls: { 1: "https://example.test/page-1-seeded.png" },
        }),
      { initialProps: { currentPage: 1 } },
    );

    await act(async () => {
      rerender({ currentPage: 5 });
      releasePage2();
    });

    await waitFor(() => {
      expect(result.current[2]).toBe("https://example.test/page-2.png");
      expect(result.current[3]).toBe("https://example.test/page-3.png");
      expect(result.current[4]).toBe("https://example.test/page-4.png");
      expect(result.current[5]).toBe("https://example.test/page-5.png");
      expect(result.current[6]).toBe("https://example.test/page-6.png");
    });
  });

  it("deduplicates concurrent requests for the same page", async () => {
    renderHook(() =>
      usePublicPageThumbnailUrls({
        documentId: "doc-1",
        publicToken: "tok-1",
        pageNumbers: [2, 2, 2],
        currentPage: 2,
      }),
    );

    await waitFor(() => {
      expect(api.getPublicPageSignedUrl).toHaveBeenCalledTimes(1);
    });
  });
});
