// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { ReadingFunnelCard } from "./ReadingFunnelCard";
import type { DocumentReadingFunnel } from "@/lib/api";

const __dirname = dirname(fileURLToPath(import.meta.url));

async function initI18n() {
  const instance = i18n.createInstance();
  const insightsJson = JSON.parse(
    readFileSync(resolve(__dirname, "../../i18n/locales/en/insights.json"), "utf-8"),
  );
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["insights"],
    defaultNS: "insights",
    resources: { en: { insights: insightsJson } },
    interpolation: { escapeValue: false },
  });
  return instance;
}

const funnel: DocumentReadingFunnel = {
  documentId: "doc-1",
  pageCount: 3,
  sessionCount: 4,
  completedSessions: 2,
  completionRate: 0.5,
  medianMaxPage: 2.5,
  avgPagesPerSession: 2.2,
  avgDurationSeconds: 40,
  biggestDropOffPage: 2,
  steps: [
    { pageNumber: 1, visitorsReached: 4, dropOffFromPrev: 0 },
    { pageNumber: 2, visitorsReached: 3, dropOffFromPrev: 0.25 },
    { pageNumber: 3, visitorsReached: 2, dropOffFromPrev: 0.333 },
  ],
};

describe("ReadingFunnelCard", () => {
  it("renders session KPIs and drop-off callout", async () => {
    const i18nInstance = await initI18n();
    render(
      <I18nextProvider i18n={i18nInstance}>
        <ReadingFunnelCard funnel={funnel} />
      </I18nextProvider>,
    );

    expect(screen.getByTestId("reading-funnel")).toBeInTheDocument();
    expect(screen.getByText("Reading funnel")).toBeInTheDocument();
    expect(screen.getByText("50%")).toBeInTheDocument();
    expect(screen.getByTestId("reading-funnel-drop")).toHaveTextContent(/page 2/i);
    expect(screen.getByText("p1")).toBeInTheDocument();
  });

  it("renders empty state when there are no sessions", async () => {
    const i18nInstance = await initI18n();
    render(
      <I18nextProvider i18n={i18nInstance}>
        <ReadingFunnelCard
          funnel={{
            ...funnel,
            sessionCount: 0,
            steps: [],
          }}
        />
      </I18nextProvider>,
    );
    expect(screen.getByTestId("reading-funnel-empty")).toBeInTheDocument();
  });
});
