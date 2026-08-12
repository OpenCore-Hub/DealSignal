// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { DocumentDetail } from "./DocumentDetail";
import type { Document } from "@/types";

const {
  getDocumentByIdMock,
  getLinksByDocumentIdMock,
  getPageAnalyticsMock,
  getDocumentVisitorsMock,
} = vi.hoisted(() => ({
  getDocumentByIdMock: vi.fn(),
  getLinksByDocumentIdMock: vi.fn(),
  getPageAnalyticsMock: vi.fn(),
  getDocumentVisitorsMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getDocumentById: getDocumentByIdMock,
    getLinksByDocumentId: getLinksByDocumentIdMock,
    getPageAnalytics: getPageAnalyticsMock,
    getDocumentVisitors: getDocumentVisitorsMock,
    updateDocumentCategory: vi.fn(),
  },
}));

vi.mock("@/lib/formatters", () => ({
  formatFileSize: vi.fn(() => "1 KB"),
  formatRelativeTime: vi.fn(() => "just now"),
}));

vi.mock("./DocumentAnalytics", () => ({ DocumentAnalytics: () => null }));
vi.mock("./DocumentContent", () => ({ DocumentContent: () => null }));
vi.mock("./DocumentInsights", () => ({ DocumentInsights: () => null }));
vi.mock("./DocumentStats", () => ({ DocumentStats: () => null }));
vi.mock("./DocumentVisitorsCard", () => ({ DocumentVisitorsCard: () => null }));
vi.mock("./DocumentLinksCard", () => ({ DocumentLinksCard: () => null }));
vi.mock("./AddToDealRoomDialog", () => ({ AddToDealRoomDialog: () => null }));
vi.mock("@/components/common/SmartBackButton", () => ({
  SmartBackButton: () => <button type="button">back</button>,
}));

const baseDoc = (category: Document["category"]): Document => ({
  id: "doc-1",
  title: "Sample",
  sourceType: "pdf",
  fileName: "sample.pdf",
  fileType: "pdf",
  fileSize: 1000,
  pageCount: 2,
  status: "ready",
  category,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
});

async function initI18n() {
  const instance = i18n.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    resources: {
      en: {
        common: {
          preview: "Preview",
          createLink: "Create Link",
          addToDealRoom: "Add to Data Room",
          retry: "Retry",
        },
        documents: {
          detail: {
            back: "Back",
            notFound: "Not found",
            loadFailed: "Failed: {{error}}",
            meta: "{{fileType}} · {{pageCount}} pages",
            categoryErrors: {
              category_immutable: "Category cannot be changed",
            },
          },
        },
        agreementDocuments: {
          page: {
            setAsAgreement: "Set as Agreement",
            unsetAsAgreement: "Unset as Agreement",
            pdfOnly: "PDF only",
          },
        },
      },
    },
  });
  return instance;
}

async function renderDetail(category: Document["category"]) {
  getDocumentByIdMock.mockResolvedValue(baseDoc(category));
  getLinksByDocumentIdMock.mockResolvedValue({ data: [] });
  getPageAnalyticsMock.mockResolvedValue({ data: [] });
  getDocumentVisitorsMock.mockResolvedValue({ data: [] });
  const instance = await initI18n();
  return render(
    <I18nextProvider i18n={instance}>
      <MemoryRouter initialEntries={["/acme/documents/doc-1"]}>
        <Routes>
          <Route path="/:workspaceSlug/documents/:documentId" element={<DocumentDetail />} />
        </Routes>
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("DocumentDetail category guards", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("hides add-to-room for agreement documents", async () => {
    await renderDetail("agreement");
    expect(await screen.findByText("Sample")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Add to Data Room" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Unset as Agreement" })).toBeEnabled();
  });

  it("hides add-to-room and disables set-as-agreement for deal_room documents", async () => {
    await renderDetail("deal_room");
    expect(await screen.findByText("Sample")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Add to Data Room" })).not.toBeInTheDocument();
    const agreementBtn = screen.getByRole("button", { name: "Set as Agreement" });
    expect(agreementBtn).toBeDisabled();
    await waitFor(() =>
      expect(agreementBtn).toHaveAttribute("title", "Category cannot be changed"),
    );
  });

  it("shows add-to-room for general documents", async () => {
    await renderDetail("general");
    expect(await screen.findByText("Sample")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Add to Data Room" })).toBeInTheDocument();
  });
});
