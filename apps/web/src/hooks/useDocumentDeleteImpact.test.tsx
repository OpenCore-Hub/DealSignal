// @vitest-environment jsdom
import { describe, expect, it, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { loadDocumentDeleteImpact, useDocumentDeleteImpact } from "./useDocumentDeleteImpact";

const { getDocumentDeleteImpactMock } = vi.hoisted(() => ({
  getDocumentDeleteImpactMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getDocumentDeleteImpact: getDocumentDeleteImpactMock,
  },
}));

describe("loadDocumentDeleteImpact", () => {
  beforeEach(() => {
    getDocumentDeleteImpactMock.mockReset();
  });

  it("maps API counts", async () => {
    getDocumentDeleteImpactMock.mockResolvedValue({
      active_link_count: 3,
      deal_room_count: 1,
    });
    await expect(loadDocumentDeleteImpact("doc-1", 9)).resolves.toEqual({
      activeLinkCount: 3,
      revokedLinkCount: 3,
      dealRoomCount: 1,
    });
  });

  it("uses revoked_link_count when the API distinguishes membership from revoke", async () => {
    getDocumentDeleteImpactMock.mockResolvedValue({
      active_link_count: 2,
      revoked_link_count: 0,
      deal_room_count: 0,
    });
    await expect(loadDocumentDeleteImpact("doc-1", 9)).resolves.toEqual({
      activeLinkCount: 2,
      revokedLinkCount: 0,
      dealRoomCount: 0,
    });
  });

  it("falls back to row link count when API fails", async () => {
    getDocumentDeleteImpactMock.mockRejectedValue(new Error("down"));
    await expect(loadDocumentDeleteImpact("doc-1", 4)).resolves.toEqual({
      activeLinkCount: 4,
      revokedLinkCount: 4,
      dealRoomCount: 0,
    });
  });
});

describe("useDocumentDeleteImpact", () => {
  beforeEach(() => {
    getDocumentDeleteImpactMock.mockReset();
    getDocumentDeleteImpactMock.mockResolvedValue({
      active_link_count: 2,
      deal_room_count: 0,
    });
  });

  it("loads impact for archive/delete confirm dialogs", async () => {
    const doc = { id: "doc-1", links: [{ id: "l1" }, { id: "l2" }] };
    const { result } = renderHook(() => useDocumentDeleteImpact(doc));
    await waitFor(() => {
      expect(result.current).toEqual({
        activeLinkCount: 2,
        revokedLinkCount: 2,
        dealRoomCount: 0,
      });
    });
    expect(getDocumentDeleteImpactMock).toHaveBeenCalledWith("doc-1");
  });

  it("clears when doc is null", async () => {
    const { result, rerender } = renderHook(
      ({ doc }) => useDocumentDeleteImpact(doc),
      { initialProps: { doc: { id: "doc-1", links: [] as { id: string }[] } } },
    );
    await waitFor(() => expect(result.current).not.toBeNull());
    rerender({ doc: null as unknown as { id: string; links: { id: string }[] } });
    await waitFor(() => expect(result.current).toBeNull());
  });
});
