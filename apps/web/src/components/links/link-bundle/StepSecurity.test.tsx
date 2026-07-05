// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import i18next from "i18next";
import { I18nextProvider, initReactI18next } from "react-i18next";
import { MemoryRouter, Routes, Route } from "react-router";
import { StepSecurity } from "./StepSecurity";
import {
  BundlePipelineProvider,
  createInitialState,
} from "./BundlePipelineContext";
import { buildConfigFromPreset } from "./pipelineUtils";
import type { BundlePipelineState } from "./BundlePipelineContext";

async function setupI18n() {
  const i18n = i18next.createInstance();
  await i18n.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    resources: {
      en: {
        links: {
          "creator.securityScore": "Security Score",
          "creator.frictionScore": "Friction Score",
          "creator.securityOptions": "Security Options",
          "creator.contactLabel": "Contact",
          "creator.sectionAccessControl": "Access Control",
          "creator.requireEmailVerification": "Require email verification",
          "creator.requireEmailDesc": "Visitor must verify email",
          "creator.whitelist": "Whitelist",
          "creator.whitelistDesc": "Only allow specified emails",
          "creator.password": "Password",
          "creator.passwordDesc": "Require password",
          "creator.sectionContentProtection": "Content Protection",
          "creator.nda": "NDA",
          "creator.ndaDesc": "Require NDA",
          "creator.allowDownload": "Allow download",
          "creator.allowDownloadDesc": "Allow download",
          "creator.watermark": "Dynamic watermark",
          "creator.watermarkDesc": "Dynamic watermark",
          "creator.sectionAdvanced": "Advanced",
          "creator.expiry": "Expiry",
          "creator.expiryPlaceholder": "Expiry",
          "creator.expiryDays.7": "7 days",
          "creator.expiryDays.30": "30 days",
          "creator.expiryDays.90": "90 days",
          "creator.expiryDays.custom": "Custom",
          "creator.maxViews": "Max views",
          "creator.maxViewsPlaceholder": "Max views",
          "creator.maxViewsOptions.unlimited": "Unlimited",
          "creator.maxViewsOptions.10": "10",
          "creator.maxViewsOptions.50": "50",
          "creator.maxViewsOptions.100": "100",
          "creator.whitelistPlaceholder": "Emails/domains",
          "preset.public.label": "Public",
          "preset.public.description": "Public",
          "preset.standard.label": "Standard",
          "preset.standard.description": "Standard",
          "preset.confidential.label": "Confidential",
          "preset.confidential.description": "Confidential",
          "preset.collaborative.label": "Collaborative",
          "preset.collaborative.description": "Collaborative",
          "preset.customized.label": "Customized",
          "preset.customized.description": "Customized",
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

async function renderStepSecurity(overrides?: Partial<BundlePipelineState>) {
  const i18n = await setupI18n();
  const initialState = createInitialState({
    step: 2,
    config: buildConfigFromPreset("customized"),
    ...overrides,
  });
  render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route
            path="/"
            element={
              <BundlePipelineProvider initialState={initialState}>
                <StepSecurity />
              </BundlePipelineProvider>
            }
          />
        </Routes>
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("StepSecurity integration", () => {
  it("renders custom security options for customized preset", async () => {
    await renderStepSecurity();
    expect(screen.getByText("Security Options")).toBeInTheDocument();
  });

  it("toggles allow download switch", async () => {
    await renderStepSecurity();
    const switches = screen.getAllByRole("switch");
    // Order: email, whitelist, password, nda, download, watermark
    fireEvent.click(switches[4]);
    // Re-query after state-driven re-render to avoid stale DOM references
    // (the old switches array may contain detached nodes after React re-renders).
    await waitFor(() => {
      const updated = screen.getAllByRole("switch");
      expect(updated[4]).toHaveAttribute("aria-checked", "true");
    });
  });

  it("keeps custom options visible when toggling whitelist on from customized", async () => {
    await renderStepSecurity();
    const switches = screen.getAllByRole("switch");
    fireEvent.click(switches[1]); // whitelist
    // After toggling whitelist from customized, the config exactly matches the
    // standard preset. The custom options panel should remain visible so the
    // user can continue editing.
    expect(screen.getByText("Security Options")).toBeInTheDocument();
  });
});
