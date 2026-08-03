// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { SetupAccessRedirectOverlay } from "./SetupAccessRedirectToast";

describe("SetupAccessRedirectOverlay", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("auto-redirects after 5 seconds", () => {
    const onRedirect = vi.fn();
    render(
      <SetupAccessRedirectOverlay
        open
        title="先设置安全规则"
        goNowLabel="立即前往"
        onRedirect={onRedirect}
      />
    );

    expect(screen.getByTestId("setup-access-redirect-toast")).toBeInTheDocument();
    expect(screen.getByText("先设置安全规则")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "立即前往" })).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(5000);
    });

    expect(onRedirect).toHaveBeenCalledTimes(1);
  });

  it("redirects immediately when Go now is clicked", () => {
    const onRedirect = vi.fn();
    render(
      <SetupAccessRedirectOverlay
        open
        title="先设置安全规则"
        goNowLabel="立即前往"
        onRedirect={onRedirect}
      />
    );

    fireEvent.click(screen.getByRole("button", { name: "立即前往" }));
    expect(onRedirect).toHaveBeenCalledTimes(1);
  });

  it("renders nothing when closed", () => {
    render(
      <SetupAccessRedirectOverlay
        open={false}
        title="先设置安全规则"
        goNowLabel="立即前往"
        onRedirect={vi.fn()}
      />
    );
    expect(screen.queryByTestId("setup-access-redirect-toast")).not.toBeInTheDocument();
  });
});
