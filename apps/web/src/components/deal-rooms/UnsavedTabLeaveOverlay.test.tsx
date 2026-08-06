// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { ComponentProps } from "react";
import { render, screen, fireEvent, act } from "@testing-library/react";
import {
  UnsavedTabLeaveOverlay,
  UNSAVED_TAB_LEAVE_SECONDS,
} from "./UnsavedTabLeaveOverlay";

function renderOpen(overrides?: Partial<ComponentProps<typeof UnsavedTabLeaveOverlay>>) {
  const onStay = vi.fn();
  const onLeave = vi.fn();
  render(
    <UnsavedTabLeaveOverlay
      open
      title="Unsaved access policy"
      description="Edits will be discarded."
      countdownLabel={(seconds) => `Leaving in ${seconds}s…`}
      stayLabel="Stay"
      leaveNowLabel="Leave now"
      onStay={onStay}
      onLeave={onLeave}
      {...overrides}
    />,
  );
  return { onStay, onLeave };
}

describe("UnsavedTabLeaveOverlay", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders nothing when closed", () => {
    render(
      <UnsavedTabLeaveOverlay
        open={false}
        title="Unsaved"
        countdownLabel={(s) => `${s}`}
        stayLabel="Stay"
        leaveNowLabel="Leave now"
        onStay={vi.fn()}
        onLeave={vi.fn()}
      />,
    );
    expect(screen.queryByTestId("unsaved-tab-leave-overlay")).not.toBeInTheDocument();
  });

  it("auto-leaves after the countdown", () => {
    const { onLeave } = renderOpen();

    expect(screen.getByTestId("unsaved-tab-leave-overlay")).toBeInTheDocument();
    expect(screen.getByTestId("unsaved-tab-leave-seconds")).toHaveTextContent(
      String(UNSAVED_TAB_LEAVE_SECONDS),
    );

    act(() => {
      vi.advanceTimersByTime(UNSAVED_TAB_LEAVE_SECONDS * 1000);
    });

    expect(onLeave).toHaveBeenCalledTimes(1);
  });

  it("leaves immediately when Leave now is clicked", () => {
    const { onLeave } = renderOpen();
    fireEvent.click(screen.getByTestId("unsaved-tab-leave-now"));
    expect(onLeave).toHaveBeenCalledTimes(1);
  });

  it("cancels when Stay is clicked", () => {
    const { onStay, onLeave } = renderOpen();
    fireEvent.click(screen.getByTestId("unsaved-tab-leave-stay"));
    expect(onStay).toHaveBeenCalled();
    expect(onLeave).not.toHaveBeenCalled();
  });

  it("does not leave twice after Leave now then timer", () => {
    const { onLeave } = renderOpen();
    fireEvent.click(screen.getByTestId("unsaved-tab-leave-now"));
    act(() => {
      vi.advanceTimersByTime(UNSAVED_TAB_LEAVE_SECONDS * 1000);
    });
    expect(onLeave).toHaveBeenCalledTimes(1);
  });
});
