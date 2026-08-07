// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { useOwnerAskCitationNavigation } from "./ownerAskCitation";

const navigate = vi.fn();

vi.mock("react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router")>();
  return {
    ...actual,
    useNavigate: () => navigate,
    useParams: () => ({ workspaceSlug: "acme-capital" }),
  };
});

describe("useOwnerAskCitationNavigation", () => {
  beforeEach(() => {
    navigate.mockReset();
  });

  it("navigates to authenticated viewer at cited page", () => {
    const { result } = renderHook(() => useOwnerAskCitationNavigation("room_1"), {
      wrapper: MemoryRouter,
    });

    result.current({
      chunkId: "chunk-1",
      documentId: "doc_1",
      text: "Revenue increased 12% YoY.",
      score: 0.9,
      viewerPage: 3,
    });

    expect(navigate).toHaveBeenCalledWith(
      "/viewer/doc_1?page=3&roomId=room_1&ws=acme-capital",
    );
  });

  it("ignores hits without documentId", () => {
    const { result } = renderHook(() => useOwnerAskCitationNavigation("room_1"), {
      wrapper: MemoryRouter,
    });

    result.current({
      chunkId: "chunk-1",
      text: "No doc",
      score: 0.5,
    });

    expect(navigate).not.toHaveBeenCalled();
  });
});
