// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, act, fireEvent } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import {
  InsightsPagesPage,
  activeInsightRooms,
  collectDealRoomDocumentIds,
  filterInsightDocuments,
  filterInsightRooms,
  insightDocScope,
  insightRoomPickerMode,
  mergeInsightDocuments,
} from "./pages";
import type { DealRoom, DealRoomFolderDocs, Document } from "@/types";

const __dirname = dirname(fileURLToPath(import.meta.url));

const {
  getDocumentsMock,
  getPageAnalyticsMock,
  getDocumentReadingFunnelMock,
  getDocumentReadingSessionsMock,
  getDocumentVisitorsMock,
  getDealRoomsMock,
  getDealRoomDocumentsMock,
} = vi.hoisted(() => ({
  getDocumentsMock: vi.fn(),
  getPageAnalyticsMock: vi.fn(),
  getDocumentReadingFunnelMock: vi.fn(),
  getDocumentReadingSessionsMock: vi.fn(),
  getDocumentVisitorsMock: vi.fn(),
  getDealRoomsMock: vi.fn(),
  getDealRoomDocumentsMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getDocuments: getDocumentsMock,
    getPageAnalytics: getPageAnalyticsMock,
    getDocumentReadingFunnel: getDocumentReadingFunnelMock,
    getDocumentReadingSessions: getDocumentReadingSessionsMock,
    getDocumentVisitors: getDocumentVisitorsMock,
    getDealRooms: getDealRoomsMock,
    getDealRoomDocuments: getDealRoomDocumentsMock,
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
  {
    id: "doc-3",
    title: "Term Sheet",
    sourceType: "pdf",
    fileName: "term.pdf",
    fileType: "pdf",
    fileSize: 1,
    pageCount: 3,
    status: "ready",
    category: "deal_room",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
];

function roomStub(id: string, name: string, status: DealRoom["status"] = "active"): DealRoom {
  return {
    id,
    name,
    description: "",
    template: "startup-fundraising",
    documentCount: 1,
    memberCount: 0,
    pendingApprovals: 0,
    ndaEnabled: false,
    createdAt: "2026-01-01T00:00:00Z",
    status,
  };
}

function roomFolder(documentId: string, title: string): DealRoomFolderDocs {
  return {
    folder: "/",
    permission: "view",
    documents: [
      {
        id: `item-${documentId}`,
        document_id: documentId,
        title,
        folder_path: "/",
        sort_order: 0,
        source_type: "pdf",
        status: "ready",
        created_at: "2026-01-01T00:00:00Z",
      },
    ],
  };
}

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
    expect(merged.map((d) => d.id)).toEqual(["doc-2", "doc-1", "doc-3"]);
  });
});

describe("filterInsightDocuments", () => {
  const docs = [...generalDocs, ...dealRoomDocs];

  it("scopes by tag then title, ignoring the other category", () => {
    expect(insightDocScope(dealRoomDocs[0]!)).toBe("deal_room");
    expect(filterInsightDocuments(docs, "deal_room", "").map((d) => d.id)).toEqual([
      "doc-2",
      "doc-3",
    ]);
    expect(filterInsightDocuments(docs, "library", "pitch").map((d) => d.id)).toEqual(["doc-1"]);
    expect(filterInsightDocuments(docs, "deal_room", "pitch")).toEqual([]);
    expect(filterInsightDocuments(docs, "deal_room", "data room")).toEqual([]);
    expect(
      filterInsightDocuments(docs, "deal_room", "", new Set(["doc-3"])).map((d) => d.id),
    ).toEqual(["doc-3"]);
  });
});

describe("collectDealRoomDocumentIds", () => {
  it("flattens folder documents and skips archived rooms", () => {
    expect(
      [...collectDealRoomDocumentIds([roomFolder("doc-2", "Financial Model")])],
    ).toEqual(["doc-2"]);
    expect(activeInsightRooms([roomStub("r1", "Alpha"), roomStub("r2", "Beta", "archived")]).map((r) => r.id)).toEqual([
      "r1",
    ]);
  });
});

describe("insightRoomPickerMode", () => {
  it("hides a single room, rails a few, and browses many", () => {
    expect(insightRoomPickerMode(1)).toBe("hidden");
    expect(insightRoomPickerMode(2)).toBe("rail");
    expect(insightRoomPickerMode(5)).toBe("rail");
    expect(insightRoomPickerMode(6)).toBe("browse");
    expect(filterInsightRooms([roomStub("r1", "Alpha"), roomStub("r2", "Beta")], "be").map((r) => r.id)).toEqual([
      "r2",
    ]);
  });
});

describe("InsightsPagesPage", () => {
  beforeEach(() => {
    getDocumentsMock.mockReset();
    getPageAnalyticsMock.mockReset();
    getDocumentReadingFunnelMock.mockReset();
    getDocumentReadingSessionsMock.mockReset();
    getDocumentVisitorsMock.mockReset();
    getDealRoomsMock.mockReset();
    getDealRoomDocumentsMock.mockReset();
    getDealRoomsMock.mockResolvedValue({ data: [] });
    getDealRoomDocumentsMock.mockResolvedValue({ data: [] });
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
      expect(picker).toHaveTextContent(/Data room/i);
      expect(picker).not.toHaveTextContent("doc-2");
      expect(picker).not.toHaveTextContent("·");
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
    expect(screen.getByText(/exclude workspace members/i)).toBeInTheDocument();
    expect(screen.getByText("Reading funnel")).toBeInTheDocument();
    expect(screen.getByText("Reading sessions")).toBeInTheDocument();
    expect(screen.getByText("Recent visitors")).toBeInTheDocument();
    expect(screen.getByText("alex@example.com")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "30d" })).toBeInTheDocument();
  });

  it("filters by source tag then file name", async () => {
    await renderPage();
    await waitFor(() => {
      expect(screen.getByTestId("insights-document-picker")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("insights-document-picker"));

    await waitFor(() => {
      expect(screen.getByTestId("insights-doc-scope-deal_room")).toHaveAttribute(
        "aria-selected",
        "true",
      );
      const options = screen.getAllByRole("option");
      expect(options.some((el) => /Financial Model/i.test(el.textContent ?? ""))).toBe(true);
      expect(options.every((el) => !/Pitch Deck/i.test(el.textContent ?? ""))).toBe(true);
      expect(screen.queryByTestId("insights-doc-room-filter")).not.toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("insights-doc-scope-library"));
    const search = await screen.findByPlaceholderText(/Search documents/i);
    expect(screen.queryByTestId("insights-doc-room-filter")).not.toBeInTheDocument();
    fireEvent.change(search, { target: { value: "Pitch" } });

    await waitFor(() => {
      const options = screen.getAllByRole("option");
      expect(options.some((el) => /Pitch Deck/i.test(el.textContent ?? ""))).toBe(true);
      expect(options.every((el) => !/Financial Model/i.test(el.textContent ?? ""))).toBe(
        true,
      );
    });
  });

  it("filters data-room files by selected room when multiple rooms exist", async () => {
    getDealRoomsMock.mockResolvedValue({
      data: [roomStub("room-a", "Alpha Room"), roomStub("room-b", "Beta Room")],
    });
    getDealRoomDocumentsMock.mockImplementation((roomId: string) => {
      if (roomId === "room-a") {
        return Promise.resolve({ data: [roomFolder("doc-2", "Financial Model")] });
      }
      return Promise.resolve({ data: [roomFolder("doc-3", "Term Sheet")] });
    });

    await renderPage();
    await waitFor(() => {
      expect(screen.getByTestId("insights-document-picker")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("insights-document-picker"));
    await screen.findByTestId("insights-doc-room-filter");
    expect(screen.getByPlaceholderText(/Search files/i)).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("insights-doc-room-room-b"));

    await waitFor(() => {
      expect(getDealRoomDocumentsMock).toHaveBeenCalledWith("room-b");
      const options = screen.getAllByRole("option");
      expect(options.some((el) => /Term Sheet/i.test(el.textContent ?? ""))).toBe(true);
      expect(options.every((el) => !/Financial Model/i.test(el.textContent ?? ""))).toBe(true);
      expect(options.every((el) => !/Pitch Deck/i.test(el.textContent ?? ""))).toBe(true);
    });
  });

  it("opens a room directory first when many data rooms exist", async () => {
    getDealRoomsMock.mockResolvedValue({
      data: Array.from({ length: 6 }, (_, index) =>
        roomStub(`room-${index}`, `Room ${index + 1}`),
      ),
    });
    getDealRoomDocumentsMock.mockResolvedValue({
      data: [roomFolder("doc-3", "Term Sheet")],
    });

    await renderPage();
    await waitFor(() => {
      expect(screen.getByTestId("insights-document-picker")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("insights-document-picker"));
    expect(await screen.findByTestId("insights-doc-room-search")).toBeInTheDocument();
    expect(screen.queryByPlaceholderText(/Search files/i)).not.toBeInTheDocument();
    expect(screen.queryByRole("option")).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId("insights-doc-room-room-5"));

    await waitFor(() => {
      expect(getDealRoomDocumentsMock).toHaveBeenCalledWith("room-5");
      expect(screen.getByPlaceholderText(/Search files/i)).toBeInTheDocument();
      expect(screen.getByTestId("insights-doc-room-back")).toHaveTextContent("Room 6");
    });
  });
});
