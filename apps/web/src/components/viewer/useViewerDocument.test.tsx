// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import type { ReactNode } from "react";
import { useViewerDocument } from "./useViewerDocument";
import { useAIStore } from "@/stores/aiStore";
import type { Document, PageAnalytics } from "@/types";

const mockDocument: Document = {
  id: "doc-001",
  title: "Q3 Pitch",
  sourceType: "pdf",
  fileName: "Q3 Pitch.pdf",
  fileType: "pdf",
  fileSize: 1024 * 1024,
  pageCount: 3,
  status: "ready",
  createdAt: "2026-06-21T00:00:00Z",
  updatedAt: "2026-06-21T00:00:00Z",
};

const mockAnalytics: PageAnalytics[] = [
  { pageNumber: 1, title: "Cover", viewCount: 10, avgDurationSeconds: 5, exitRate: 0.05 },
];

const makePages = (count: number) =>
  Array.from({ length: count }, (_, i) => ({
    pageNumber: i + 1,
    width: 612,
    height: 792,
  }));

const { apiMock } = vi.hoisted(() => ({
  apiMock: {
    getDocumentById: vi.fn(() => Promise.resolve(mockDocument)),
    getPageAnalytics: vi.fn(() => Promise.resolve({ data: mockAnalytics })),
    getDocumentPages: vi.fn((id: string) =>
      Promise.resolve({ documentId: id, pages: makePages(3), total: 3 })
    ),
    getPageSignedUrl: vi.fn(() =>
      Promise.resolve({ page_number: 1, image_url: "https://example.test/page.png", expires_at: "", width: 612, height: 792 })
    ),
    getPublicDocumentPages: vi.fn((id: string) =>
      Promise.resolve({ documentId: id, pages: makePages(3), total: 3 })
    ),
    getPublicPageSignedUrl: vi.fn(() =>
      Promise.resolve({ page_number: 1, image_url: "https://example.test/public.png", expires_at: "", width: 612, height: 792 })
    ),
    recordViewerEvent: vi.fn(() => Promise.resolve(undefined)),
    recordPublicEvent: vi.fn(() => Promise.resolve(undefined)),
  },
}));

vi.mock("@/lib/api", () => ({ api: apiMock }));

function wrapper(route = "/viewer/doc-001") {
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <MemoryRouter initialEntries={[route]}>
        <Routes>
          <Route path="/viewer/:documentId" element={children as React.ReactElement} />
        </Routes>
      </MemoryRouter>
    );
  };
}

describe("useViewerDocument", () => {
  beforeEach(() => {
    useAIStore.getState().reset();
    vi.clearAllMocks();
  });

  it("loads authenticated document, analytics, and pages", async () => {
    const { result } = renderHook(() => useViewerDocument(), { wrapper: wrapper() });

    expect(result.current.loading).toBe(true);

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.doc).toEqual(mockDocument);
    expect(result.current.pages).toHaveLength(3);
    expect(result.current.analytics).toEqual(mockAnalytics);
    expect(result.current.documentId).toBe("doc-001");
    expect(apiMock.getDocumentById).toHaveBeenCalledWith("doc-001");
    expect(apiMock.getDocumentPages).toHaveBeenCalledWith("doc-001");
  });

  it("loads public document and pages when token is provided", async () => {
    const { result } = renderHook(() => useViewerDocument({ publicToken: "token-123", publicDocument: mockDocument }), {
      wrapper: wrapper("/viewer/doc-001"),
    });

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.doc).toEqual(mockDocument);
    expect(result.current.pages).toHaveLength(3);
    expect(apiMock.getPublicDocumentPages).toHaveBeenCalledWith("doc-001", "token-123", undefined);
  });

  it("exposes error state when loading fails", async () => {
    apiMock.getDocumentById.mockRejectedValueOnce(new Error("network error"));
    const { result } = renderHook(() => useViewerDocument(), { wrapper: wrapper() });

    await waitFor(() => expect(result.current.error).toBe("network error"));
    expect(result.current.doc).toBeNull();
  });

  it("synchronizes page with AI highlight", async () => {
    const { result } = renderHook(() => useViewerDocument(), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => {
      useAIStore.getState().setHighlight(null, 2);
    });

    await waitFor(() => expect(result.current.page).toBe(2));
  });

  it("fetches signed URL for current page", async () => {
    const { result } = renderHook(() => useViewerDocument(), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.loading).toBe(false));

    await waitFor(() => expect(result.current.imageUrl).toBe("https://example.test/page.png"));
    expect(apiMock.getPageSignedUrl).toHaveBeenCalledWith("doc-001", 1);
  });

  it("initializes page from ?page query param", async () => {
    const { result } = renderHook(() => useViewerDocument(), {
      wrapper: wrapper("/viewer/doc-001?page=2"),
    });

    expect(result.current.page).toBe(2);
    await waitFor(() => expect(result.current.loading).toBe(false));
    await waitFor(() =>
      expect(apiMock.getPageSignedUrl).toHaveBeenCalledWith("doc-001", 2)
    );
  });
});
