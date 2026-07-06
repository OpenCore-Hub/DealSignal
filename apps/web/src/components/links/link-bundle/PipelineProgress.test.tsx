// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import i18next from "i18next";
import { I18nextProvider, initReactI18next } from "react-i18next";
import { PipelineProgress } from "./PipelineProgress";
import {
  BundlePipelineProvider,
  createInitialState,
} from "./BundlePipelineContext";

// Minimal i18n setup
async function setupI18n() {
  const i18n = i18next.createInstance();
  await i18n.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    resources: {
      en: {
        links: {
          "bundle.stepDocuments": "Documents",
          "bundle.stepSecurity": "Security",
          "bundle.stepReview": "Review",
        },
      },
    },
    interpolation: { escapeValue: false },
  });
  return i18n;
}

async function renderProgress(step: 1 | 2 | 3) {
  const i18n = await setupI18n();
  const initialState = createInitialState({ step });
  return render(
    <I18nextProvider i18n={i18n}>
      <BundlePipelineProvider initialState={initialState}>
        <PipelineProgress />
      </BundlePipelineProvider>
    </I18nextProvider>,
  );
}

describe("PipelineProgress", () => {
  it("renders all three step labels", async () => {
    await renderProgress(1);
    expect(screen.getByText("Documents")).toBeInTheDocument();
    expect(screen.getByText("Security")).toBeInTheDocument();
    expect(screen.getByText("Review")).toBeInTheDocument();
  });

  it("shows step 1 as current and steps 2-3 as future", async () => {
    await renderProgress(1);
    const buttons = screen.getAllByRole("button");
    // Only past steps are clickable; current and future steps are disabled
    expect(buttons[0]).toBeDisabled(); // step 1 current
    expect(buttons[1]).toBeDisabled(); // step 2 future
    expect(buttons[2]).toBeDisabled(); // step 3 future
  });

  it("shows checkmark for completed steps", async () => {
    await renderProgress(3);
    // Steps 1 and 2 should show checkmarks (past) as enabled buttons
    const buttons = screen.getAllByRole("button");
    expect(buttons[0]).not.toBeDisabled();
    expect(buttons[1]).not.toBeDisabled();
  });

  it("allows clicking on past steps", async () => {
    await renderProgress(2);
    // Step 1 is past, should be clickable
    const buttons = screen.getAllByRole("button");
    expect(buttons[0]).not.toBeDisabled(); // Step 1 (past)
    expect(buttons[2]).toBeDisabled(); // Step 3 (future)
  });
});
