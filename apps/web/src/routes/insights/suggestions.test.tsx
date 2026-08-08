// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { InsightsSuggestionsPage } from "./suggestions";

const __dirname = dirname(fileURLToPath(import.meta.url));

const navigateMock = vi.fn();

vi.mock("react-router", async () => {
  const actual = await vi.importActual<typeof import("react-router")>("react-router");
  return {
    ...actual,
    useNavigate: () => navigateMock,
    useParams: () => ({ workspaceSlug: "acme" }),
  };
});

async function initI18n() {
  const instance = i18n.createInstance();
  const insightsJson = JSON.parse(
    readFileSync(resolve(__dirname, "../../i18n/locales/en/insights.json"), "utf-8"),
  );
  const commonJson = JSON.parse(
    readFileSync(resolve(__dirname, "../../i18n/locales/en/common.json"), "utf-8"),
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

describe("InsightsSuggestionsPage", () => {
  beforeEach(() => {
    navigateMock.mockReset();
  });

  it("redirects follow-ups to Deal Radar", async () => {
    const i18nInstance = await initI18n();
    render(
      <MemoryRouter>
        <I18nextProvider i18n={i18nInstance}>
          <Routes>
            <Route path="*" element={<InsightsSuggestionsPage />} />
          </Routes>
        </I18nextProvider>
      </MemoryRouter>,
    );

    expect(screen.getByTestId("suggestions-radar-redirect")).toBeInTheDocument();
    expect(screen.getByText(/Follow-ups live on Deal Radar/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Open Deal Radar/i }));
    expect(navigateMock).toHaveBeenCalledWith(
      "/acme/dashboard?filter=buying_window",
    );
  });
});
