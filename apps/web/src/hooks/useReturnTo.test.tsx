// @vitest-environment jsdom
import type { ReactNode } from "react";
import { describe, expect, it } from "vitest";
import { renderHook } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { useReturnTo, type ReturnToState } from "./useReturnTo";

function wrapper(initialEntries: { pathname: string; state?: ReturnToState }[]) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <MemoryRouter initialEntries={initialEntries}>{children}</MemoryRouter>;
  };
}

describe("useReturnTo", () => {
  it("uses default values when no state is present", () => {
    const { result } = renderHook(() => useReturnTo("/default", "Back to list"), {
      wrapper: wrapper([{ pathname: "/detail" }]),
    });
    expect(result.current).toEqual({ to: "/default", label: "Back to list" });
  });

  it("uses returnTo and returnLabel from location state", () => {
    const { result } = renderHook(() => useReturnTo("/default", "Back to list"), {
      wrapper: wrapper([{ pathname: "/detail", state: { returnTo: "/deal-rooms/1", returnLabel: "Back to deal room" } }]),
    });
    expect(result.current).toEqual({ to: "/deal-rooms/1", label: "Back to deal room" });
  });

  it("uses default label when returnLabel is missing", () => {
    const { result } = renderHook(() => useReturnTo("/default", "Back to list"), {
      wrapper: wrapper([{ pathname: "/detail", state: { returnTo: "/dashboard" } }]),
    });
    expect(result.current).toEqual({ to: "/dashboard", label: "Back to list" });
  });

  it("ignores empty returnTo and falls back to default", () => {
    const { result } = renderHook(() => useReturnTo("/default", "Back to list"), {
      wrapper: wrapper([{ pathname: "/detail", state: { returnTo: "", returnLabel: "" } }]),
    });
    expect(result.current).toEqual({ to: "/default", label: "Back to list" });
  });
});
