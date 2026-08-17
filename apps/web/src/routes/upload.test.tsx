// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { UploadPage } from "./upload";
import type { Document } from "@/types";

const navigateMock = vi.fn();
let onUploadComplete:
  | ((documents: Document[]) => void)
  | undefined;

vi.mock("react-router", async () => {
  const actual = await vi.importActual<typeof import("react-router")>("react-router");
  return {
    ...actual,
    useNavigate: () => navigateMock,
  };
});

vi.mock("@/components/upload/Uploader", () => ({
  Uploader: ({
    onUploadComplete: onComplete,
  }: {
    onUploadComplete?: (documents: Document[]) => void;
  }) => {
    onUploadComplete = onComplete;
    return <div data-testid="uploader" />;
  },
}));

async function renderUpload(entry = "/acme/documents/upload") {
  const instance = i18n.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["documents"],
    defaultNS: "documents",
    resources: {
      en: {
        documents: {
          upload: { title: "Upload", description: "Upload files" },
        },
      },
    },
    interpolation: { escapeValue: false },
  });
  return render(
    <I18nextProvider i18n={instance}>
      <MemoryRouter initialEntries={[entry]}>
        <Routes>
          <Route path="/:workspaceSlug/documents/upload" element={<UploadPage />} />
        </Routes>
      </MemoryRouter>
    </I18nextProvider>,
  );
}

const uploadedDoc = (id: string, overrides: Partial<Document> = {}): Document => ({
  id,
  title: `${id}.pdf`,
  sourceType: "pdf",
  fileName: `${id}.pdf`,
  fileType: "pdf",
  fileSize: 1000,
  pageCount: 1,
  status: "ready",
  category: "general",
  createdAt: "2026-08-16T10:00:00Z",
  updatedAt: "2026-08-16T10:00:00Z",
  ...overrides,
});

describe("UploadPage", () => {
  beforeEach(() => {
    navigateMock.mockReset();
    onUploadComplete = undefined;
  });

  it("sends a successful library upload into the create-link pipeline", async () => {
    await renderUpload();
    onUploadComplete?.([uploadedDoc("doc_1"), uploadedDoc("doc_2")]);
    expect(navigateMock).toHaveBeenCalledWith(
      "/acme/links/new?documentId=doc_1&documentId=doc_2",
    );
  });

  it("does not open the create-link pipeline for agreement uploads", async () => {
    await renderUpload("/acme/documents/upload?category=agreement");
    onUploadComplete?.([uploadedDoc("nda_1", { category: "agreement" })]);
    expect(navigateMock).toHaveBeenCalledWith("/acme/agreement-documents");
  });

  it("does not enter the create-link pipeline before documents are ready", async () => {
    await renderUpload();
    onUploadComplete?.([uploadedDoc("doc_1", { status: "processing" })]);
    expect(navigateMock).toHaveBeenCalledWith("/acme/documents");
    expect(navigateMock).not.toHaveBeenCalledWith(
      expect.stringContaining("/links/new"),
    );
  });
});
