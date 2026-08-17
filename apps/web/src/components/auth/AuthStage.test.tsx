// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { AuthStage, railFromNamespace } from "./AuthStage";

const rail = railFromNamespace((key) => key, "stage");

describe("AuthStage", () => {
  it("renders the signal field and form title without a card shell", () => {
    render(
      <AuthStage rail={rail} kicker="Members" title="Sign in">
        <button type="button">Continue</button>
      </AuthStage>,
    );

    expect(screen.getByRole("heading", { level: 1, name: "Sign in" })).toBeInTheDocument();
    expect(screen.getByText("stage.headline")).toBeInTheDocument();
    expect(screen.getByText("stage.signals.0.title")).toBeInTheDocument();
    expect(document.querySelector("[data-slot='card']")).toBeNull();
  });
});
