// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, act } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { HeatBreakdownDialog } from "./HeatBreakdownDialog";

const __dirname = dirname(fileURLToPath(import.meta.url));

const { getLinkHeatScoreMock, getDocumentHeatScoreMock } = vi.hoisted(() => ({
  getLinkHeatScoreMock: vi.fn(),
  getDocumentHeatScoreMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getLinkHeatScore: getLinkHeatScoreMock,
    getDocumentHeatScore: getDocumentHeatScoreMock,
  },
}));

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

describe("HeatBreakdownDialog", () => {
  beforeEach(() => {
    getLinkHeatScoreMock.mockReset();
    getDocumentHeatScoreMock.mockReset();
    getLinkHeatScoreMock.mockResolvedValue({
      linkId: "link-1",
      score: 88,
      level: "hot",
      trend: "rising",
      circle: "founder",
      breakdown: {
        opens: 30,
        revisits: 18,
        avgDurationMinutes: 12,
        keyPageViews: 25,
        forwardSignals: 15,
        downloads: 0,
        bouncePenalty: 0,
      },
      updatedAt: "2026-06-20T00:00:00Z",
    });
  });

  it("loads and renders heat.Compute factors when open", async () => {
    const i18nInstance = await initI18n();
    await act(async () => {
      render(
        <I18nextProvider i18n={i18nInstance}>
          <HeatBreakdownDialog
            open
            onOpenChange={() => {}}
            linkId="link-1"
            linkLabel="Q3 Pitch"
          />
        </I18nextProvider>,
      );
      await new Promise((r) => setTimeout(r, 0));
    });

    await waitFor(() => {
      expect(screen.getByText("Why this heat level")).toBeInTheDocument();
    });
    expect(getLinkHeatScoreMock).toHaveBeenCalledWith("link-1");
    expect(getDocumentHeatScoreMock).not.toHaveBeenCalled();
    expect(screen.getByText("Key pages")).toBeInTheDocument();
    expect(screen.getByText(/88 pts/)).toBeInTheDocument();
    expect(screen.getByText("Rising")).toBeInTheDocument();
    expect(screen.getByText(/Founder · engagement factors/i)).toBeInTheDocument();
    expect(screen.queryByText("File extras")).not.toBeInTheDocument();
    expect(screen.queryByText("Reading depth")).not.toBeInTheDocument();
    expect(screen.queryByText("Key-page evidence")).not.toBeInTheDocument();
  });

  it("shows skim key-page evidence for a share", async () => {
    getLinkHeatScoreMock.mockResolvedValue({
      linkId: "link-1",
      score: 3,
      level: "cold",
      trend: "stable",
      circle: "founder",
      breakdown: {
        opens: 3,
        revisits: 0,
        avgDurationMinutes: 0.3,
        keyPageViews: 0,
        forwardSignals: 0,
        downloads: 0,
        bouncePenalty: 0,
      },
      keyPages: {
        engaged: 0,
        total: 1,
        minSeconds: 3,
        pages: [{ pageNumber: 1, title: "Financials", engagedViews: 0, totalViews: 1 }],
      },
      updatedAt: "2026-06-20T00:00:00Z",
    });

    const i18nInstance = await initI18n();
    await act(async () => {
      render(
        <I18nextProvider i18n={i18nInstance}>
          <HeatBreakdownDialog
            open
            onOpenChange={() => {}}
            linkId="link-1"
            linkLabel="Q3 Pitch"
          />
        </I18nextProvider>,
      );
      await new Promise((r) => setTimeout(r, 0));
    });

    await waitFor(() => {
      expect(screen.getByText("Key-page evidence")).toBeInTheDocument();
    });
    expect(screen.getByText(/Title matched, but dwell was under 3s/i)).toBeInTheDocument();
    expect(screen.getByText(/p1 · Financials/i)).toBeInTheDocument();
    expect(screen.queryByText("File extras")).not.toBeInTheDocument();
  });

  it("loads document-native heat and contributing shares", async () => {
    getDocumentHeatScoreMock.mockResolvedValue({
      documentId: "doc-1",
      title: "Q3 Pitch",
      score: 61,
      level: "warm",
      trend: "stable",
      circle: "founder",
      breakdown: {
        opens: 20,
        revisits: 0,
        avgDurationMinutes: 10,
        keyPageViews: 15,
        forwardSignals: 0,
        downloads: 0,
        bouncePenalty: 0,
      },
      views: 3,
      overlay: { readingDepth: 8, qaCitations: 6, crossDomain: 1.1 },
      contributingLinks: [{ id: "link-1", name: "Investor share", pageViews: 12 }],
      keyPages: {
        engaged: 0,
        total: 1,
        minSeconds: 3,
        pages: [{ pageNumber: 1, title: "Financials", engagedViews: 0, totalViews: 1 }],
      },
      updatedAt: "2026-06-20T00:00:00Z",
    });

    const i18nInstance = await initI18n();
    await act(async () => {
      render(
        <I18nextProvider i18n={i18nInstance}>
          <HeatBreakdownDialog
            open
            onOpenChange={() => {}}
            kind="document"
            entityId="doc-1"
            label="Q3 Pitch"
          />
        </I18nextProvider>,
      );
      await new Promise((r) => setTimeout(r, 0));
    });

    await waitFor(() => {
      expect(screen.getByText("Why this heat level")).toBeInTheDocument();
    });
    expect(getDocumentHeatScoreMock).toHaveBeenCalledWith("doc-1");
    expect(getLinkHeatScoreMock).not.toHaveBeenCalled();
    expect(screen.getByText(/Founder · reading on this file/i)).toBeInTheDocument();
    expect(screen.getByText("Investor share")).toBeInTheDocument();
    expect(screen.getByText(/12 page views/i)).toBeInTheDocument();
    expect(screen.getByText(/61 pts/)).toBeInTheDocument();
    expect(screen.getByText("File extras")).toBeInTheDocument();
    expect(screen.getByText("Reading depth")).toBeInTheDocument();
    expect(screen.getByText("Q&A citations")).toBeInTheDocument();
    expect(screen.getByText("Cross-domain attention")).toBeInTheDocument();
    expect(screen.getByText("Key-page evidence")).toBeInTheDocument();
    expect(screen.getByText(/Title matched, but dwell was under 3s/i)).toBeInTheDocument();
    expect(screen.getByText(/p1 · Financials/i)).toBeInTheDocument();
  });

  it("hides document extras when overlay is all zeros", async () => {
    getDocumentHeatScoreMock.mockResolvedValue({
      documentId: "doc-1",
      title: "Q3 Pitch",
      score: 12,
      level: "cold",
      trend: "stable",
      breakdown: {
        opens: 3,
        revisits: 0,
        avgDurationMinutes: 2,
        keyPageViews: 0,
        forwardSignals: 0,
        downloads: 0,
        bouncePenalty: 0,
      },
      overlay: { readingDepth: 0, qaCitations: 0, crossDomain: 0 },
      views: 1,
      contributingLinks: [],
      updatedAt: "2026-06-20T00:00:00Z",
    });

    const i18nInstance = await initI18n();
    await act(async () => {
      render(
        <I18nextProvider i18n={i18nInstance}>
          <HeatBreakdownDialog
            open
            onOpenChange={() => {}}
            kind="document"
            entityId="doc-1"
            label="Q3 Pitch"
          />
        </I18nextProvider>,
      );
      await new Promise((r) => setTimeout(r, 0));
    });

    await waitFor(() => {
      expect(screen.getByText(/12 pts/)).toBeInTheDocument();
    });
    expect(screen.queryByText("File extras")).not.toBeInTheDocument();
    expect(screen.queryByText("Reading depth")).not.toBeInTheDocument();
  });

  it("truncates a long share URL in the subtitle so the dialog cannot overflow", async () => {
    const longUrl = "http://localhost:5173/l/b4a79da1ebbe2132d8634c02da1d9844";
    const i18nInstance = await initI18n();
    await act(async () => {
      render(
        <I18nextProvider i18n={i18nInstance}>
          <HeatBreakdownDialog
            open
            onOpenChange={() => {}}
            linkId="link-1"
            linkLabel={longUrl}
          />
        </I18nextProvider>,
      );
      await new Promise((r) => setTimeout(r, 0));
    });

    await waitFor(() => {
      expect(screen.getByTestId("heat-breakdown-label")).toHaveTextContent(longUrl);
    });
    const label = screen.getByTestId("heat-breakdown-label");
    expect(label).toHaveClass("truncate");
    expect(label).toHaveAttribute("title", longUrl);
    expect(screen.getByTestId("heat-breakdown-dialog")).toHaveClass("overflow-x-hidden");
    expect(screen.getByTestId("heat-breakdown-dialog")).toHaveClass("min-w-0");
  });
});
