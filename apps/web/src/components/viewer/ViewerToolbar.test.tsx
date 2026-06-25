// @vitest-environment jsdom
import { describe, it, expect, vi, beforeAll } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import i18next from "i18next";
import { I18nextProvider, initReactI18next } from "react-i18next";
import { ViewerToolbar } from "./ViewerToolbar";
import type { Document } from "@/types";

const mockDocument: Document = {
  id: "doc-001",
  title: "Q3 Pitch",
  sourceType: "pdf",
  fileName: "Q3 Pitch.pdf",
  fileType: "pdf",
  fileSize: 1024 * 1024,
  pageCount: 5,
  status: "ready",
  createdAt: "2026-06-21T00:00:00Z",
  updatedAt: "2026-06-21T00:00:00Z",
};

async function createI18n() {
  const instance = i18next.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["common", "documents"],
    defaultNS: "common",
    resources: {
      en: {
        common: {
          download: "Download",
        },
        documents: {
          viewer: {
            meta: "{{fileType}} · {{fileSize}} · {{pageCount}} pages",
            zoomOut: "Zoom out",
            zoomIn: "Zoom in",
            previousPage: "Previous page",
            nextPage: "Next page",
          },
        },
      },
    },
    interpolation: { escapeValue: false },
  });
  return instance;
}

async function renderToolbar(props: Partial<Parameters<typeof ViewerToolbar>[0]> = {}) {
  const i18n = await createI18n();
  const defaultProps = {
    doc: mockDocument,
    page: 1,
    totalPages: 5,
    zoom: 100,
    onZoomOut: vi.fn(),
    onZoomIn: vi.fn(),
    onPreviousPage: vi.fn(),
    onNextPage: vi.fn(),
    onDownload: vi.fn(),
  };
  return render(
    <I18nextProvider i18n={i18n}>
      <ViewerToolbar {...defaultProps} {...props} />
    </I18nextProvider>
  );
}

describe("ViewerToolbar", () => {
  beforeAll(async () => {
    if (!i18next.isInitialized) {
      await i18next.use(initReactI18next).init({ lng: "en", fallbackLng: "en" });
    }
  });

  it("renders document title and meta", async () => {
    await renderToolbar();
    expect(screen.getByText("Q3 Pitch")).toBeInTheDocument();
    expect(screen.getByText("PDF · 1 MB · 5 pages")).toBeInTheDocument();
    expect(screen.getByText("1 / 5")).toBeInTheDocument();
  });

  it("calls navigation handlers", async () => {
    const onNextPage = vi.fn();
    const onPreviousPage = vi.fn();
    await renderToolbar({ page: 2, onNextPage, onPreviousPage });

    fireEvent.click(screen.getByRole("button", { name: /Next page/i }));
    expect(onNextPage).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole("button", { name: /Previous page/i }));
    expect(onPreviousPage).toHaveBeenCalledTimes(1);
  });

  it("disables previous on first page and next on last page", async () => {
    const { rerender } = await renderToolbar({ page: 1, totalPages: 3 });
    expect(screen.getByRole("button", { name: /Previous page/i })).toBeDisabled();
    expect(screen.getByRole("button", { name: /Next page/i })).toBeEnabled();

    const i18n = await createI18n();
    rerender(
      <I18nextProvider i18n={i18n}>
        <ViewerToolbar
          doc={mockDocument}
          page={3}
          totalPages={3}
          zoom={100}
          onZoomOut={vi.fn()}
          onZoomIn={vi.fn()}
          onPreviousPage={vi.fn()}
          onNextPage={vi.fn()}
          onDownload={vi.fn()}
        />
      </I18nextProvider>
    );
    expect(screen.getByRole("button", { name: /Previous page/i })).toBeEnabled();
    expect(screen.getByRole("button", { name: /Next page/i })).toBeDisabled();
  });

  it("calls zoom and download handlers", async () => {
    const onZoomIn = vi.fn();
    const onZoomOut = vi.fn();
    const onDownload = vi.fn();
    await renderToolbar({ onZoomIn, onZoomOut, onDownload });

    fireEvent.click(screen.getByRole("button", { name: /Zoom in/i }));
    expect(onZoomIn).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole("button", { name: /Zoom out/i }));
    expect(onZoomOut).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole("button", { name: /Download/i }));
    expect(onDownload).toHaveBeenCalledTimes(1);
  });
});
