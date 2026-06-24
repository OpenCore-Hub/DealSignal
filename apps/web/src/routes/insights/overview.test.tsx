// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { InsightsOverviewPage } from "./overview";
import type { InsightsOverview } from "@/lib/api";
import type { Link, AccessLog } from "@/types";

const __dirname = dirname(fileURLToPath(import.meta.url));

const { getInsightsOverviewMock, getLinksMock, getAccessLogsMock } = vi.hoisted(() => ({
  getInsightsOverviewMock: vi.fn(),
  getLinksMock: vi.fn(),
  getAccessLogsMock: vi.fn(),
}));

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    api: {
      ...actual.api,
      getInsightsOverview: getInsightsOverviewMock,
      getLinks: getLinksMock,
      getAccessLogs: getAccessLogsMock,
    },
  };
});

const mockOverview: InsightsOverview = {
  tierCounts: { hot: 2, warm: 3, cold: 5 },
  topDocuments: [{ id: "doc-1", title: "Q3 Pitch", views: 42, heatLevel: "hot" }],
  topLinks: [{ id: "link-1", shortUrl: "http://localhost:8080/l/abc", views: 12, heatLevel: "warm" }],
  topContacts: [{ id: "c-1", email: "sarah@example.com", score: 88, heatLevel: "hot" }],
};

const mockLinks: Link[] = [
  { id: "link-1", documentId: "doc-1", documentTitle: "Q3 Pitch", shortUrl: "http://localhost:8080/l/abc", accessCount: 12, heatLevel: "warm", createdAt: "2026-06-20T00:00:00Z" },
];

const today = new Date().toISOString();
const mockLogs: AccessLog[] = [{ id: "log-1", linkId: "link-1", visitorEmail: "sarah@example.com", timestamp: today, durationSeconds: 30 }];

async function initI18n() {
  const instance = i18n.createInstance();
  const insightsJson = JSON.parse(readFileSync(resolve(__dirname, "../../i18n/locales/en/insights.json"), "utf-8"));
  const commonJson = JSON.parse(readFileSync(resolve(__dirname, "../../i18n/locales/en/common.json"), "utf-8"));
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

async function renderPage() {
  const i18nInstance = await initI18n();
  let result!: ReturnType<typeof render>;
  await act(async () => {
    result = render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter initialEntries={["/acme/insights/overview"]}>
          <Routes>
            <Route path=":workspaceSlug/insights/overview" element={<InsightsOverviewPage />} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>
    );
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  return result;
}

describe("InsightsOverviewPage", () => {
  beforeEach(() => {
    getInsightsOverviewMock.mockReset();
    getLinksMock.mockReset();
    getAccessLogsMock.mockReset();

    getInsightsOverviewMock.mockResolvedValue(mockOverview);
    getLinksMock.mockResolvedValue({ data: mockLinks });
    getAccessLogsMock.mockResolvedValue({ data: mockLogs });
  });

  it("renders overview stats and top lists", async () => {
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("2")).toBeInTheDocument();
    });

    expect(screen.getByText("Q3 Pitch")).toBeInTheDocument();
    expect(screen.getByText(/sarah@example\.com/)).toBeInTheDocument();
  });

  it("shows error and retries on failure", async () => {
    getInsightsOverviewMock.mockRejectedValue(new Error("network error"));
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("network error")).toBeInTheDocument();
    });

    getInsightsOverviewMock.mockResolvedValue(mockOverview);
    fireEvent.click(screen.getByRole("button", { name: /retry/i }));

    await waitFor(() => {
      expect(screen.getByText("Q3 Pitch")).toBeInTheDocument();
    });
  });
});
