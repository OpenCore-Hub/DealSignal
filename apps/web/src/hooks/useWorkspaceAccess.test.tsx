// @vitest-environment jsdom
vi.unmock("@/hooks/useWorkspaceAccess");

import type { ReactNode } from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { useWorkspaceAccess } from "./useWorkspaceAccess";

const { getWorkspacesMock } = vi.hoisted(() => ({
  getWorkspacesMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getWorkspaces: getWorkspacesMock,
  },
}));

function wrap(slug: string) {
  return ({ children }: { children: ReactNode }) => (
    <MemoryRouter initialEntries={[`/${slug}/dashboard`]}>
      <Routes>
        <Route path="/:workspaceSlug/*" element={<>{children}</>} />
      </Routes>
    </MemoryRouter>
  );
}

describe("useWorkspaceAccess", () => {
  beforeEach(() => {
    getWorkspacesMock.mockReset();
  });

  it("marks guest as read-only", async () => {
    getWorkspacesMock.mockResolvedValue({
      data: [{ id: "1", slug: "kendiyang", name: "Kendi", role: "guest" }],
    });
    const { result } = renderHook(() => useWorkspaceAccess("kendiyang"), {
      wrapper: wrap("kendiyang"),
    });
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.isGuest).toBe(true);
    expect(result.current.canWrite).toBe(false);
    expect(result.current.canManage).toBe(false);
    expect(result.current.canRead).toBe(true);
  });

  it("allows member write but not manage", async () => {
    getWorkspacesMock.mockResolvedValue({
      data: [{ id: "1", slug: "kendiyang", name: "Kendi", role: "member" }],
    });
    const { result } = renderHook(() => useWorkspaceAccess("kendiyang"), {
      wrapper: wrap("kendiyang"),
    });
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.canWrite).toBe(true);
    expect(result.current.canManage).toBe(false);
  });
});
