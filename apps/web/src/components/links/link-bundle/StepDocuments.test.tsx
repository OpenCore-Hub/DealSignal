// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, act } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { StepDocuments } from "./StepDocuments";
import { BundlePipelineProvider, createInitialState } from "./BundlePipelineContext";
import { toast } from "sonner";
import type { BundlePipelineState } from "./BundlePipelineContext";

const __dirname = dirname(fileURLToPath(import.meta.url));

const { getDocumentsMock } = vi.hoisted(() => ({
  getDocumentsMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getDocuments: getDocumentsMock,
  },
}));

const mockDocs = [
  {
    id: "doc-2",
    title: "Target Document",
    sourceType: "pdf",
    fileName: "Target Document.pdf",
    fileType: "pdf",
    fileSize: 1_000,
    pageCount: 1,
    status: "ready",
    createdAt: "2026-07-11T00:00:00Z",
    updatedAt: "2026-07-11T00:00:00Z",
  },
];

Object.defineProperty(window, "localStorage", {
  writable: true,
  value: {
    getItem: vi.fn(),
    setItem: vi.fn(),
    removeItem: vi.fn(),
    clear: vi.fn(),
  },
});

async function setupI18n() {
  const instance = i18n.createInstance();
  const linksJson = JSON.parse(readFileSync(resolve(__dirname, "../../../i18n/locales/en/links.json"), "utf-8"));
  const commonJson = JSON.parse(readFileSync(resolve(__dirname, "../../../i18n/locales/en/common.json"), "utf-8"));
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["links", "common"],
    defaultNS: "links",
    resources: { en: { links: linksJson, common: commonJson } },
    interpolation: { escapeValue: false },
  });
  return instance;
}

async function renderStepDocuments(
  route: string,
  overrides?: Partial<BundlePipelineState>
) {
  const i18nInstance = await setupI18n();
  const initialState = createInitialState({
    step: 1,
    mode: "create",
    ...overrides,
  });
  const view = render(
    <I18nextProvider i18n={i18nInstance}>
      <MemoryRouter initialEntries={[route]}>
        <Routes>
          <Route
            path="/links/new"
            element={
              <BundlePipelineProvider initialState={initialState}>
                <StepDocuments />
              </BundlePipelineProvider>
            }
          />
        </Routes>
      </MemoryRouter>
    </I18nextProvider>
  );
  // Flush pending async state updates from draft restoration.
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  return view;
}

describe("StepDocuments", () => {
  beforeEach(() => {
    getDocumentsMock.mockReset();
    getDocumentsMock.mockResolvedValue({ data: mockDocs });
    vi.clearAllMocks();
  });

  it("auto-selects the URL document and ignores a stale draft without warning", async () => {
    const warningSpy = vi.spyOn(toast, "warning").mockImplementation(() => "");

    // Simulate entering from a document row with a stale draft referencing a
    // document that no longer exists.
    await renderStepDocuments("/links/new?documentId=doc-2", {
      pendingDraftDocIds: ["missing-doc"],
    });

    await waitFor(() => {
      // The explicitly requested document is selected.
      expect(screen.getByText("1 selected")).toBeInTheDocument();
    });

    // The stale draft should not trigger the "draft unavailable" warning.
    expect(warningSpy).not.toHaveBeenCalled();
  });

  it("still restores a draft and warns when some draft documents are missing", async () => {
    const warningSpy = vi.spyOn(toast, "warning").mockImplementation(() => "");

    await renderStepDocuments("/links/new", {
      pendingDraftDocIds: ["missing-doc", "doc-2"],
    });

    await waitFor(() => {
      expect(screen.getByText("1 selected")).toBeInTheDocument();
    });

    expect(warningSpy).toHaveBeenCalledTimes(1);
  });
});
