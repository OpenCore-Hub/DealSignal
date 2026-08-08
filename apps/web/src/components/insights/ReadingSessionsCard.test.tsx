// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { ReadingSessionsCard } from "./ReadingSessionsCard";
import type { DocumentReadingSessions } from "@/lib/api";

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

const data: DocumentReadingSessions = {
  documentId: "doc-1",
  pageCount: 5,
  sessionModel: "reading_session",
  sessions: [
    {
      id: "s1",
      linkId: "l1",
      visitorId: "v1",
      visitorEmail: "buyer@example.com",
      startedAt: "2026-08-07T10:00:00Z",
      lastActivityAt: "2026-08-07T10:20:00Z",
      maxPage: 5,
      distinctPageCount: 3,
      totalDurationSeconds: 90,
      completed: true,
      pages: [
        { pageNumber: 1, durationSeconds: 30 },
        { pageNumber: 3, durationSeconds: 40 },
        { pageNumber: 5, durationSeconds: 20 },
      ],
    },
  ],
};

describe("ReadingSessionsCard", () => {
  it("renders session actor, completion, and page chips", async () => {
    const i18nInstance = await initI18n();
    render(
      <I18nextProvider i18n={i18nInstance}>
        <ReadingSessionsCard data={data} />
      </I18nextProvider>,
    );
    expect(screen.getByTestId("reading-sessions")).toBeInTheDocument();
    expect(screen.getByText("buyer@example.com")).toBeInTheDocument();
    expect(screen.getByText("Completed")).toBeInTheDocument();
    expect(screen.getByText("p1")).toBeInTheDocument();
    expect(screen.getByText("p5")).toBeInTheDocument();
  });

  it("renders empty state", async () => {
    const i18nInstance = await initI18n();
    render(
      <I18nextProvider i18n={i18nInstance}>
        <ReadingSessionsCard
          data={{ documentId: "doc-1", pageCount: 0, sessionModel: "reading_session", sessions: [] }}
        />
      </I18nextProvider>,
    );
    expect(screen.getByTestId("reading-sessions-empty")).toBeInTheDocument();
  });
});
