/** @vitest-environment jsdom */
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import {
  claimCiteIndex,
  claimIsFactStyled,
  renderBoundClaims,
} from "./boundAnswer";

const hits = [
  { chunkId: "c1", text: "price", score: 0.9, sourceName: "SPA.pdf" },
  { chunkId: "c2", text: "contracts", score: 0.8, sourceName: "Disc.xlsx" },
];

describe("boundAnswer helpers", () => {
  it("maps hitIds to 1-based cite index", () => {
    expect(claimCiteIndex({ text: "x", hitIds: ["c2"] }, hits)).toBe(2);
    expect(claimCiteIndex({ text: "x" }, hits)).toBe(0);
  });

  it("requires confidence + hitIds for fact styling", () => {
    expect(
      claimIsFactStyled({ text: "x", hitIds: ["c1"], confidence: "grounded" }),
    ).toBe(true);
    expect(claimIsFactStyled({ text: "x", hitIds: ["c1"] })).toBe(false);
    expect(claimIsFactStyled({ text: "x", confidence: "grounded" })).toBe(false);
  });
});

describe("renderBoundClaims", () => {
  it("renders fact claim with cite button and muted narrative", () => {
    const onCite = vi.fn();
    render(
      <div>
        {renderBoundClaims(
          [
            {
              text: "Purchase price is ten million.",
              hitIds: ["c1"],
              confidence: "grounded",
            },
            { text: "In summary," },
          ],
          hits,
          null,
          onCite,
        )}
      </div>,
    );
    expect(screen.getByTestId("knowledge-claim-fact")).toBeInTheDocument();
    expect(screen.getByTestId("knowledge-claim-narrative")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("knowledge-cite-1"));
    expect(onCite).toHaveBeenCalledWith(1);
  });
});
