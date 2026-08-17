// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { InsightsKeyPagesPage } from "./key-pages";
import type { KeyPageCompliance } from "@/lib/api";

const __dirname = dirname(fileURLToPath(import.meta.url));

const { getKeyPageComplianceMock, getKeyPageSettingsMock, saveKeyPageSettingsMock } = vi.hoisted(
  () => ({
    getKeyPageComplianceMock: vi.fn(),
    getKeyPageSettingsMock: vi.fn(),
    saveKeyPageSettingsMock: vi.fn(),
  }),
);

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    api: {
      ...actual.api,
      getKeyPageCompliance: getKeyPageComplianceMock,
      getKeyPageSettings: getKeyPageSettingsMock,
      saveKeyPageSettings: saveKeyPageSettingsMock,
    },
  };
});

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

const mockReport: KeyPageCompliance = {
  rangeDays: 30,
  circle: "founder",
  generatedAt: "2026-08-08T04:00:00Z",
  totalViews: 3,
  engagedViews: 2,
  uniqueVisitors: 2,
  distinctPages: 1,
  matchRules: [
    {
      category: "financials",
      keywords: ["financial", "财务", "营收"],
    },
    {
      category: "team",
      keywords: ["team", "团队"],
    },
  ],
  byCategory: [{ category: "financials", count: 3 }],
  pages: [
    {
      documentId: "doc-1",
      documentTitle: "Pitch Deck",
      pageNumber: 4,
      pageTitle: "Financial Projections",
      category: "financials",
      views: 3,
      engagedViews: 2,
      uniqueVisitors: 2,
      avgDurationSeconds: 12,
      lastViewedAt: "2026-08-07T12:00:00Z",
    },
  ],
  events: [
    {
      id: "pv-1",
      linkId: "link-1",
      visitorId: "v1",
      visitorEmail: "buyer@example.com",
      documentId: "doc-1",
      documentTitle: "Pitch Deck",
      pageNumber: 4,
      pageTitle: "Financial Projections",
      category: "financials",
      durationSeconds: 15,
      createdAt: "2026-08-07T12:00:00Z",
      dealRoomName: "Series A",
      dealRoomId: "room-1",
    },
    {
      id: "pv-2",
      linkId: "link-1",
      visitorId: "v2",
      visitorEmail: "skim@example.com",
      documentId: "doc-1",
      documentTitle: "Pitch Deck",
      pageNumber: 4,
      pageTitle: "Financial Projections",
      category: "financials",
      durationSeconds: 1,
      createdAt: "2026-08-07T12:01:00Z",
      dealRoomName: "Series A",
      dealRoomId: "room-1",
    },
  ],
  hasMore: false,
  limit: 25,
  offset: 0,
};

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

async function renderPage() {
  const i18nInstance = await initI18n();
  let result!: ReturnType<typeof render>;
  await act(async () => {
    result = render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter initialEntries={["/acme/insights/key-pages"]}>
          <Routes>
            <Route path=":workspaceSlug/insights/key-pages" element={<InsightsKeyPagesPage />} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>,
    );
    await new Promise((r) => setTimeout(r, 0));
  });
  return result;
}

describe("InsightsKeyPagesPage", () => {
  beforeEach(() => {
    getKeyPageComplianceMock.mockReset();
    getKeyPageSettingsMock.mockReset();
    saveKeyPageSettingsMock.mockReset();
    getKeyPageComplianceMock.mockResolvedValue(mockReport);
    getKeyPageSettingsMock.mockResolvedValue({
      defaultCircle: "founder",
      extraKeywords: { custom: ["watermark"] },
      builtinRules: mockReport.matchRules,
      matchRules: mockReport.matchRules,
      canEdit: true,
    });
    saveKeyPageSettingsMock.mockResolvedValue({
      defaultCircle: "founder",
      extraKeywords: { custom: ["watermark"], financials: ["cap table"] },
      builtinRules: mockReport.matchRules,
      matchRules: mockReport.matchRules,
      canEdit: true,
    });
  });

  async function openKeywordsCard() {
    const toggle = await screen.findByTestId("insights-key-pages-keywords-toggle");
    const sw = toggle.querySelector('button[role="switch"]');
    expect(sw).toBeTruthy();
    fireEvent.click(sw!);
    await screen.findByTestId("insights-key-pages-settings");
  }

  it("hides workspace keywords card until the toggle is on", async () => {
    await renderPage();
    await waitFor(() => expect(getKeyPageComplianceMock).toHaveBeenCalled());
    expect(screen.getByTestId("insights-key-pages-keywords-toggle")).toBeInTheDocument();
    expect(screen.queryByTestId("insights-key-pages-settings")).not.toBeInTheDocument();
    await openKeywordsCard();
    expect(screen.getByTestId("insights-key-pages-category-extras")).toBeInTheDocument();
  });

  it("renders summary, page table, and event trail without match-rules panel", async () => {
    await renderPage();
    await waitFor(() => expect(getKeyPageComplianceMock).toHaveBeenCalled());
    expect(screen.queryByTestId("insights-key-pages-match-rules")).not.toBeInTheDocument();
    expect(screen.getByText("buyer@example.com")).toBeInTheDocument();
    expect(screen.getAllByText("Financial Projections").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Financials").length).toBeGreaterThan(0);
    expect(screen.getByText("2/3")).toBeInTheDocument();
    expect(screen.getByText("skim@example.com")).toBeInTheDocument();
    expect(screen.getByText("Skim")).toBeInTheDocument();
    expect(screen.queryByText("buyer@example.com")?.closest("tr")?.textContent).not.toMatch(/Skim/);
  });

  it("saves per-category workspace extras", async () => {
    await renderPage();
    await openKeywordsCard();
    await screen.findByTestId("insights-key-pages-extra-input-custom");
    fireEvent.change(screen.getByTestId("insights-key-pages-extra-input-custom"), {
      target: { value: "watermark, 股权" },
    });
    fireEvent.change(screen.getByTestId("insights-key-pages-extra-input-financials"), {
      target: { value: "cap table" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Save keywords/i }));
    await waitFor(() =>
      expect(saveKeyPageSettingsMock).toHaveBeenCalledWith(
        expect.objectContaining({
          defaultCircle: "founder",
          extraKeywords: {
            custom: ["watermark", "股权"],
            financials: ["cap table"],
          },
        }),
      ),
    );
  });

  it("switches circle when Sales is selected", async () => {
    await renderPage();
    await waitFor(() => expect(screen.getByText("buyer@example.com")).toBeInTheDocument());
    const viewCircleGroup = screen.getByRole("group", { name: /Keyword circle/i });
    fireEvent.click(viewCircleGroup.querySelector("button:nth-child(3)")!);
    await waitFor(() =>
      expect(getKeyPageComplianceMock).toHaveBeenLastCalledWith(
        expect.objectContaining({ circle: "sales", days: 30 }),
      ),
    );
  });
});
