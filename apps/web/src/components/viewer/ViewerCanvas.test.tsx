// @vitest-environment jsdom
import { describe, it, expect, vi, beforeAll, afterEach } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import i18next from "i18next";
import { I18nextProvider, initReactI18next } from "react-i18next";
import { ViewerCanvas } from "./ViewerCanvas";
import type { Document, PageAnalytics, Evidence } from "@/types";
import type { WatermarkInfo } from "./WatermarkOverlay";

const mockDocument: Document = {
  id: "doc-001",
  title: "Q3 Pitch",
  sourceType: "pdf",
  fileName: "Q3 Pitch.pdf",
  fileType: "pdf",
  fileSize: 1024 * 1024,
  pageCount: 3,
  status: "ready",
  createdAt: "2026-06-21T00:00:00Z",
  updatedAt: "2026-06-21T00:00:00Z",
};

const mockPages = Array.from({ length: 3 }, (_, i) => ({
  pageNumber: i + 1,
  width: 612,
  height: 792,
}));

const mockAnalytics: PageAnalytics[] = [
  { pageNumber: 1, title: "Cover", viewCount: 10, avgDurationSeconds: 5, exitRate: 0.05 },
  { pageNumber: 2, title: "Problem", viewCount: 5, avgDurationSeconds: 3, exitRate: 0.1 },
  { pageNumber: 3, title: "Solution", viewCount: 0, avgDurationSeconds: 0, exitRate: 0 },
];

const demoEvidence: Evidence[] = [
  {
    chunk_id: "ev-001",
    page_number: 1,
    quote: "Revenue grew 3x.",
    boxes: [{ x: 0.1, y: 0.2, w: 0.4, h: 0.05 }],
    score: 0.95,
  },
];

const demoWatermark: WatermarkInfo = {
  email: "visitor@example.test",
  ip: "203.0.113.1",
  viewedAt: "2026-06-21T14:00:00Z",
};

async function createCanvasI18n() {
  const instance = i18next.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["documents"],
    defaultNS: "documents",
    resources: {
      en: {
        documents: {
          viewer: {
            processing: "Document status: {{status}}",
            pageLabel: "Page {{pageNumber}}",
            pagePlaceholder: "Page {{pageNumber}}",
            previewPlaceholder: "Document preview placeholder",
            currentPageStats: "Viewed {{count}} times · avg. time {{duration}}.",
            pageHeat: "Page heat",
            thumbnailViews: "{{count}} views · {{duration}}",
            printWarning: "Print and capture discouraged",
            printWarningHint: "Content may include identifying watermarks and stay attributable.",
            inactiveBlurWarning: "Content blurred while inactive",
            inactiveBlurHint: "Helps reduce leak risk; screenshots remain attributable via watermark.",
          },
        },
      },
    },
    interpolation: { escapeValue: false },
  });
  return instance;
}

async function renderCanvas(props: Partial<Parameters<typeof ViewerCanvas>[0]> = {}) {
  const i18n = await createCanvasI18n();
  const defaultProps = {
    doc: mockDocument,
    page: 1,
    zoom: 100,
    pages: mockPages,
    analytics: mockAnalytics,
    imageUrl: null as string | null,
    onSelectPage: vi.fn(),
  };
  return render(
    <I18nextProvider i18n={i18n}>
      <ViewerCanvas {...defaultProps} {...props} />
    </I18nextProvider>
  );
}

describe("ViewerCanvas", () => {
  beforeAll(async () => {
    if (!i18next.isInitialized) {
      await i18next.use(initReactI18next).init({ lng: "en", fallbackLng: "en" });
    }
  });

  afterEach(() => {
    vi.restoreAllMocks();
    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      get: () => "visible",
    });
  });

  it("renders the page image when a signed URL is available", async () => {
    await renderCanvas({ imageUrl: "https://cdn.example.com/page-1.png" });
    const img = screen.getByAltText("Page 1") as HTMLImageElement;
    expect(img.src).toBe("https://cdn.example.com/page-1.png");
  });

  it("renders a placeholder when no image URL is available", async () => {
    await renderCanvas({ imageUrl: null });
    expect(screen.getAllByText("Page 1").length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText("Document preview placeholder")).toBeInTheDocument();
  });

  it("renders processing message when document is not ready", async () => {
    await renderCanvas({ doc: { ...mockDocument, status: "processing" }, imageUrl: null });
    expect(screen.getByText("Document status: processing")).toBeInTheDocument();
  });

  it("renders highlight overlay for active evidence", async () => {
    await renderCanvas({ evidence: demoEvidence });
    expect(screen.getByTitle("Revenue grew 3x.")).toBeInTheDocument();
  });

  it("renders watermark overlay", async () => {
    await renderCanvas({ watermark: demoWatermark });
    expect(document.querySelector("canvas")).toBeInTheDocument();
  });

  it("displays current page analytics", async () => {
    await renderCanvas({ page: 2, imageUrl: "https://cdn.example.com/page-2.png" });
    expect(screen.getByText(/Viewed 5 times/)).toBeInTheDocument();
  });

  it("renders thumbnail navigation", async () => {
    await renderCanvas();
    const thumbnails = screen.getAllByRole("button", { name: /Page \d/i });
    expect(thumbnails).toHaveLength(3);
  });

  it("does not show inactive blur when screenshot protection is off", async () => {
    await renderCanvas({ screenshotProtectionEnabled: false });
    await act(async () => {
      fireEvent.blur(window);
    });
    expect(screen.queryByTestId("inactive-blur-overlay")).not.toBeInTheDocument();
  });

  it("blurs content when the window becomes inactive and protection is on", async () => {
    await renderCanvas({
      screenshotProtectionEnabled: true,
      imageUrl: "https://cdn.example.com/page-1.png",
    });
    expect(screen.queryByTestId("inactive-blur-overlay")).not.toBeInTheDocument();

    vi.spyOn(document, "hasFocus").mockReturnValue(false);
    await act(async () => {
      fireEvent.blur(window);
    });

    expect(screen.getByTestId("inactive-blur-overlay")).toBeInTheDocument();
    expect(screen.getByText("Content blurred while inactive")).toBeInTheDocument();
    expect(
      screen.getByText("Helps reduce leak risk; screenshots remain attributable via watermark.")
    ).toBeInTheDocument();

    vi.spyOn(document, "hasFocus").mockReturnValue(true);
    await act(async () => {
      fireEvent.focus(window);
    });
    expect(screen.queryByTestId("inactive-blur-overlay")).not.toBeInTheDocument();
  });

  it("blurs content when the tab becomes hidden", async () => {
    await renderCanvas({ screenshotProtectionEnabled: true });

    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      get: () => "hidden",
    });
    await act(async () => {
      document.dispatchEvent(new Event("visibilitychange"));
    });

    expect(screen.getByTestId("inactive-blur-overlay")).toBeInTheDocument();

    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      get: () => "visible",
    });
    vi.spyOn(document, "hasFocus").mockReturnValue(true);
    await act(async () => {
      document.dispatchEvent(new Event("visibilitychange"));
    });
    expect(screen.queryByTestId("inactive-blur-overlay")).not.toBeInTheDocument();
  });
});
