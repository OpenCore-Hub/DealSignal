// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { InsightsPage } from "./insights";

const __dirname = dirname(fileURLToPath(import.meta.url));

async function initI18n() {
  const instance = i18n.createInstance();
  const insightsJson = JSON.parse(
    readFileSync(resolve(__dirname, "../i18n/locales/en/insights.json"), "utf-8"),
  );
  const commonJson = JSON.parse(
    readFileSync(resolve(__dirname, "../i18n/locales/en/common.json"), "utf-8"),
  );
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["insights", "common"],
    defaultNS: "insights",
    resources: { en: { insights: insightsJson, common: commonJson } },
    interpolation: { escapeValue: false },
  });
  return instance;
}

describe("InsightsPage nav", () => {
  it("labels the suggestions tab as a pointer to Deal Radar, not a third inbox", async () => {
    const i18nInstance = await initI18n();
    render(
      <MemoryRouter initialEntries={["/acme/insights/overview"]}>
        <I18nextProvider i18n={i18nInstance}>
          <Routes>
            <Route path="/:workspaceSlug/insights" element={<InsightsPage />}>
              <Route path="overview" element={<div />} />
              <Route path="pages" element={<div />} />
              <Route path="access" element={<div />} />
              <Route path="key-pages" element={<div />} />
              <Route path="suggestions" element={<div />} />
            </Route>
          </Routes>
        </I18nextProvider>
      </MemoryRouter>,
    );

    const radarTab = screen.getByRole("link", { name: "Go to Deal Radar" });
    expect(radarTab).toHaveAttribute("href", "/acme/insights/suggestions");
    expect(screen.queryByRole("link", { name: "Suggestions" })).not.toBeInTheDocument();
  });
});
