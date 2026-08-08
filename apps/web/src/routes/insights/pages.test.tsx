// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, act, fireEvent } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { InsightsPagesPage, mergeInsightDocuments } from "./pages";
import type { Document } from "@/types";

const __dirname = dirname(fileURLToPath(import.meta.url));

const {
  getDocumentsMock,
  getPageAnalyticsMock,
  getDocumentReadingFunnelMock,
  getDocumentReadingSessionsMock,
  getDocumentVisitorsMock,
} = vi.hoisted(() => ({
  getDocumentsMock: vi.fn(),
  getPageAnalyticsMock: vi.fn(),
  getDocumentReadingFunnelMock: vi.fn(),
  getDocumentReadingSessionsMock: vi.fn(),
  getDocumentVisitorsMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getDocuments: getDocumentsMock,
    getPageAnalytics: getPageAnalyticsMock,
    getDocumentReadingFunnel: getDocumentReadingFunnelMock,
    getDocumentReadingSessions: getDocumentReadingSessionsMock,
    getDocumentVisitors: getDocumentVisitorsMock,
  },
}));

vi.mock("@/components/documents/DocumentAnalytics", () => ({
  DocumentAnalytics: () => <div data-testid="document-analytics" />,
}));

const generalDocs: Document[] = [
  {
    id: "doc-1",
    title: "Pitch Deck",
    sourceType: "pdf",
    fileName: "pitch.pdf",
    fileType: "pdf",
    fileSize: 1,
    pageCount: 10,
    status: "ready",
    category: "general",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
];

const dealRoomDocs: Document[] = [
  {
    id: "doc-2",
    title: "Financial Model",
    sourceType: "pdf",
    fileName: "fin.pdf",
    fileType: "pdf",
    fileSize: 1,
    pageCount: 5,
    status: "ready",
    category: "deal_room",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
];

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
        <MemoryRouter initialEntries={["/acme/insights/pages"]}>
          <Routes>
            <Route path=":workspaceSlug/insights/pages" element={<InsightsPagesPage />} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>,
    );
    await new Promise((r) => setTimeout(r, 0));
  });
  return result;
}

describe("mergeInsightDocuments", () => {
  it("dedupes by id and sorts by title", () => {
    const merged = mergeInsightDocuments(generalDocs, [
      ...dealRoomDocs,
      { ...generalDocs[0]!, title: "Pitch Deck Dup" },
    ]);
    expect(merged.map((d) => d.id)).toEqual(["doc-2", "doc-1"]);
  });
});

describe("InsightsPagesPage", () => {
  beforeEach(() => {
    getDocumentsMock.mockReset();
    getPageAnalyticsMock.mockReset();
    getDocumentReadingFunnelMock.mockReset();
    getDocumentReadingSessionsMock.mockReset();
    getDocumentVisitorsMock.mockReset();
    getDocumentsMock.mockImplementation((_status: string, category: string) => {
      if (category === "general") return Promise.resolve({ data: generalDocs });
      if (category === "deal_room") return Promise.resolve({ data: dealRoomDocs });
      return Promise.resolve({ data: [] });
    });
    getPageAnalyticsMock.mockResolvedValue({ data: [{ pageNumber: 1, views: 2, avgDuration: 10 }] });
    getDocumentReadingFunnelMock.mockResolvedValue({
      documentId: "doc-1",
      pageCount: 3,
      sessionCount: 2,
      completedSessions: 1,
      completionRate: 0.5,
      medianMaxPage: 2,
      avgPagesPerSession: 2,
      avgDurationSeconds: 30,
      biggestDropOffPage: 2,
      sessionModel: "reading_session",
      steps: [
        { pageNumber: 1, visitorsReached: 2, dropOffFromPrev: 0 },
        { pageNumber: 2, visitorsReached: 1, dropOffFromPrev: 0.5 },
        { pageNumber: 3, visitorsReached: 1, dropOffFromPrev: 0 },
      ],
    });
    getDocumentReadingSessionsMock.mockResolvedValue({
      documentId: "doc-1",
      pageCount: 3,
      sessionModel: "reading_session",
      sessions: [
        {
          id: "s1",
          linkId: "l1",
          visitorId: "v1",
          visitorEmail: "sarah@example.com",
          startedAt: "2026-06-19T10:00:00Z",
          lastActivityAt: "2026-06-19T10:20:00Z",
          maxPage: 3,
          distinctPageCount: 2,
          totalDurationSeconds: 60,
          completed: true,
          pages: [
            { pageNumber: 1, durationSeconds: 20 },
            { pageNumber: 3, durationSeconds: 40 },
          ],
        },
      ],
    });
    getDocumentVisitorsMock.mockResolvedValue({
      data: [
        {
          visitorId: "v1",
          visitorEmail: "alex@example.com",
          pageViewCount: 4,
          avgDurationSeconds: 22,
          lastSeenAt: "2026-06-19T10:20:00Z",
        },
      ],
    });
  });

  it("loads general and deal_room documents and shows sessions + funnel", async () => {
    await renderPage();

    await waitFor(() => {
      // Header title removed; document picker is right-aligned searchable combobox.
      expect(screen.queryByText("Page engagement")).not.toBeInTheDocument();
      expect(screen.queryByText(/Library and data-room documents/i)).not.toBeInTheDocument();
      const picker = screen.getByTestId("insights-document-picker");
      expect(picker).toHaveTextContent(/Financial Model/i);
      expect(picker).not.toHaveTextContent("doc-2");
    });

    expect(getDocumentsMock).toHaveBeenCalledWith("all", "general");
    expect(getDocumentsMock).toHaveBeenCalledWith("all", "deal_room");

    await waitFor(() => {
      expect(screen.getByText("sarah@example.com")).toBeInTheDocument();
    });
    await waitFor(() => {
      // mergeInsightDocuments sorts by title → Financial Model (doc-2) first.
      expect(getPageAnalyticsMock).toHaveBeenCalledWith("doc-2", { days: 30 });
      expect(getDocumentReadingFunnelMock).toHaveBeenCalledWith("doc-2", { days: 30 });
      expect(getDocumentReadingSessionsMock).toHaveBeenCalledWith("doc-2", 40, { days: 30 });
      expect(getDocumentVisitorsMock).toHaveBeenCalledWith("doc-2", { days: 30 });
    });
    await waitFor(() => {
      expect(screen.getByTestId("reading-funnel")).toBeInTheDocument();
      expect(screen.getByTestId("reading-sessions")).toBeInTheDocument();
    });
    expect(screen.getByText("Reading funnel")).toBeInTheDocument();
    expect(screen.getByText("Reading sessions")).toBeInTheDocument();
    expect(screen.getByText("Recent visitors")).toBeInTheDocument();
    expect(screen.getByText("alex@example.com")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "30d" })).toBeInTheDocument();
  });

  it("filters documents from the searchable picker", async () => {
    await renderPage();
    await waitFor(() => {
      expect(screen.getByTestId("insights-document-picker")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("insights-document-picker"));
    const search = await screen.findByPlaceholderText(/Search documents/i);
    fireEvent.change(search, { target: { value: "Pitch" } });

    await waitFor(() => {
      const options = screen.getAllByRole("option");
      expect(options.some((el) => /Pitch Deck/i.test(el.textContent ?? ""))).toBe(true);
      expect(options.every((el) => !/Financial Model/i.test(el.textContent ?? ""))).toBe(
        true,
      );
    });
  });
});
