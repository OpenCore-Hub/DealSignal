// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, beforeAll } from "vitest";
import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import i18next from "i18next";
import { I18nextProvider, initReactI18next } from "react-i18next";
import { MemoryRouter, Route, Routes } from "react-router";
import { CanvasViewer } from "./CanvasViewer";
import enFormatters from "@/i18n/locales/en/formatters.json";
import type { Document, PageAnalytics, Evidence } from "@/types";
import type { WatermarkInfo } from "./WatermarkOverlay";

const mockDocument: Document = {
  id: "doc-001",
  title: "Q3 Pitch",
  fileName: "Q3 Pitch.pdf",
  fileType: "pdf",
  fileSize: 1024 * 1024,
  pageCount: 3,
  status: "ready",
  createdAt: "2026-06-21T00:00:00Z",
  updatedAt: "2026-06-21T00:00:00Z",
};

const mockAnalytics: PageAnalytics[] = [
  { pageNumber: 1, title: "Cover", viewCount: 10, avgDurationSeconds: 5, exitRate: 0.05 },
  { pageNumber: 2, title: "Problem", viewCount: 5, avgDurationSeconds: 3, exitRate: 0.1 },
  { pageNumber: 3, title: "Solution", viewCount: 0, avgDurationSeconds: 0, exitRate: 0 },
];

const demoEvidence: Evidence[] = [
  {
    id: "ev-001",
    pageNumber: 1,
    text: "Revenue grew 3x.",
    bbox: { x: 0.1, y: 0.2, w: 0.4, h: 0.05 },
  },
];

const demoWatermark: WatermarkInfo = {
  email: "visitor@example.test",
  ip: "203.0.113.1",
  viewedAt: "2026-06-21T14:00:00Z",
};

const { apiMock, resolveGetDocument, resolveGetPageAnalytics } = vi.hoisted(() => {
  let resolveDoc: (value: Document) => void = () => {};
  let resolveAnalytics: (value: { data: PageAnalytics[] }) => void = () => {};
  return {
    apiMock: {
      getDocumentById: vi.fn(() => new Promise<Document>((r) => { resolveDoc = r; })),
      getPageAnalytics: vi.fn(
        () => new Promise<{ data: PageAnalytics[] }>((r) => { resolveAnalytics = r; })
      ),
    },
    resolveGetDocument: (value: Document) => resolveDoc(value),
    resolveGetPageAnalytics: (value: { data: PageAnalytics[] }) => resolveAnalytics(value),
  };
});

vi.mock("@/lib/api", () => ({ api: apiMock }));

beforeAll(async () => {
  if (!i18next.isInitialized) {
    await i18next.use(initReactI18next).init({
      lng: "en",
      fallbackLng: "en",
      resources: { en: { formatters: enFormatters } },
    });
  } else {
    i18next.addResourceBundle("en", "formatters", enFormatters, true, true);
  }
});

async function createViewerI18n() {
  const instance = i18next.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["common", "documents", "formatters"],
    defaultNS: "common",
    resources: {
      en: {
        common: {
          retry: "Retry",
          download: "Download",
          "error.loadFailed": "Load failed",
        },
        documents: {
          viewer: {
            loadFailed: "Failed to load: {{error}}",
            notFound: "Document does not exist or cannot be loaded",
            meta: "{{fileType}} · {{fileSize}} · {{pageCount}} pages",
            zoomOut: "Zoom out",
            zoomIn: "Zoom in",
            previousPage: "Previous page",
            nextPage: "Next page",
            downloadDisabled: "Download requires backend signed URL support",
            pageHeat: "Page heat",
            pageLabel: "Page {{pageNumber}}",
            thumbnailViews: "{{count}} views · {{duration}}",
            pagePlaceholder: "Page {{pageNumber}}",
            previewPlaceholder: "Document preview placeholder",
            signedUrlNotice: "Real page content will render here once the backend signed URL is loaded.",
            currentPageStats: "Viewed {{count}} times · avg. time {{duration}}.",
          },
        },
        formatters: enFormatters,
      },
    },
    interpolation: { escapeValue: false },
  });
  return instance;
}

async function renderWithProviders(
  initialRoute = "/viewer/doc-001",
  props: { evidence?: Evidence[]; watermark?: WatermarkInfo | null } = {}
) {
  const i18n = await createViewerI18n();
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter initialEntries={[initialRoute]}>
        <Routes>
          <Route path="/viewer/:documentId" element={<CanvasViewer {...props} />} />
        </Routes>
      </MemoryRouter>
    </I18nextProvider>
  );
}

async function loadDocument() {
  await act(async () => {
    resolveGetDocument(mockDocument);
    resolveGetPageAnalytics({ data: mockAnalytics });
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

describe("CanvasViewer", () => {
  beforeEach(() => {
    apiMock.getDocumentById.mockClear();
    apiMock.getPageAnalytics.mockClear();
  });

  it("renders document, thumbnails, highlight and watermark", async () => {
    await renderWithProviders("/viewer/doc-001", {
      evidence: demoEvidence,
      watermark: demoWatermark,
    });
    await loadDocument();

    expect(await screen.findByText("Q3 Pitch")).toBeInTheDocument();
    expect(await screen.findByText("PDF · 1 MB · 3 pages")).toBeInTheDocument();

    const thumbnails = await screen.findAllByRole("button", { name: /Page \d/i, hidden: true });
    expect(thumbnails).toHaveLength(3);

    const highlight = screen.getByTitle("Revenue grew 3x.");
    expect(highlight).toBeInTheDocument();

    expect(screen.getAllByText(/visitor@example\.test · 203\.0\.113\.1/).length).toBeGreaterThan(0);
  });

  it("does not render highlight when evidence is empty", async () => {
    await renderWithProviders("/viewer/doc-001", { evidence: [] });
    await loadDocument();

    await screen.findByText("Q3 Pitch");
    expect(screen.queryByTitle("Revenue grew 3x.")).not.toBeInTheDocument();
  });

  it("does not render watermark when watermark is null", async () => {
    await renderWithProviders("/viewer/doc-001", { watermark: null });
    await loadDocument();

    await screen.findByText("Q3 Pitch");
    expect(screen.queryByText(/viewer@example\.test/)).not.toBeInTheDocument();
  });

  it("switches page when clicking a thumbnail", async () => {
    await renderWithProviders("/viewer/doc-001", { evidence: demoEvidence });
    await loadDocument();

    const thumbnails = await screen.findAllByRole("button", { name: /Page \d/i, hidden: true });
    fireEvent.click(thumbnails[1]);

    await waitFor(() => {
      expect(screen.getByText("2 / 3")).toBeInTheDocument();
    });
  });
});
