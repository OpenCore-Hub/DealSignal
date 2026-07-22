// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act, within } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { DealRoomDetailPage } from "./detail";
import type { DealRoom, DealRoomFolder, DealRoomFolderDocs, DealRoomMember, DealRoomTemplate, Document } from "@/types";

const __dirname = dirname(fileURLToPath(import.meta.url));

const { getDealRoomByIdMock, getDealRoomTemplatesMock, getDocumentsMock, getDocumentByIdMock, uploadDocumentMock, addDealRoomDocumentMock, createDealRoomFolderMock, getDealRoomLinksMock, getLinkAnalyticsMock, listLinkQuestionsMock } = vi.hoisted(() => ({
  getDealRoomByIdMock: vi.fn(),
  getDealRoomTemplatesMock: vi.fn(),
  getDocumentsMock: vi.fn(),
  getDocumentByIdMock: vi.fn(),
  uploadDocumentMock: vi.fn(),
  addDealRoomDocumentMock: vi.fn(),
  createDealRoomFolderMock: vi.fn(),
  getDealRoomLinksMock: vi.fn(),
  getLinkAnalyticsMock: vi.fn(),
  listLinkQuestionsMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getDealRoomById: getDealRoomByIdMock,
    getDealRoomTemplates: getDealRoomTemplatesMock,
    getDocuments: getDocumentsMock,
    getDocumentById: getDocumentByIdMock,
    uploadDocument: uploadDocumentMock,
    addDealRoomDocument: addDealRoomDocumentMock,
    createDealRoomFolder: createDealRoomFolderMock,
    getDealRoomLinks: getDealRoomLinksMock,
    getLinkAnalytics: getLinkAnalyticsMock,
    listLinkQuestions: listLinkQuestionsMock,
  },
}));

vi.mock("@/lib/formatters", async () => {
  const actual = await vi.importActual<typeof import("@/lib/formatters")>("@/lib/formatters");
  return {
    ...actual,
    formatFileSize: vi.fn(() => "4.2 MB"),
  };
});

const mockFolders: DealRoomFolder[] = [
  { path: "/pitch", name: "01 Pitch Deck", description: "Latest fundraising deck", sort_order: 0 },
  { path: "/financials", name: "02 Financials", description: "Historical financials", sort_order: 1 },
];

const mockFolderDocs: DealRoomFolderDocs[] = [
  {
    folder: "/pitch",
    permission: "view",
    documents: [
      {
        id: "rd_1",
        document_id: "doc_1",
        title: "Acme Seed Round Pitch Deck",
        folder_path: "/pitch",
        sort_order: 0,
        source_type: "pdf",
        status: "ready",
        page_count: 18,
        file_size: 4_200_000,
        created_at: "2026-06-18T09:30:00Z",
      },
    ],
  },
];

const mockMembers: DealRoomMember[] = [
  { id: "rm_1", email: "john@acme.capital", role: "admin", nda_status: "signed", status: "active", name: "John Doe" },
  { id: "rm_2", email: "owner@acme.capital", role: "owner", nda_status: "signed", status: "active", name: "Owner" },
];

const mockRoom: DealRoom = {
  id: "room-1",
  name: "Series A Data Room",
  description: "Due diligence materials",
  slug: "series-a-data-room",
  template: "series-a-plus",
  documentCount: 1,
  memberCount: 1,
  pendingApprovals: 1,
  ndaEnabled: true,
  requiresApproval: true,
  createdAt: "2026-06-20T10:00:00Z",
  status: "active",
  folders: mockFolders,
  documents: mockFolderDocs,
  members: mockMembers,
};

const mockTemplates: DealRoomTemplate[] = [
  {
    id: "tpl-series-a",
    name: "Series A",
    description: "Growth-stage data room",
    scenario: "series-a-plus",
    folderStructure: [{ name: "Financials" }],
    recommendedFiles: ["Pitch deck", "Financial model", "Cap table"],
    defaultPermissionLevel: "standard",
    ndaEnabled: true,
  },
];

const mockWorkspaceDocs: Document[] = [
  {
    id: "doc_1",
    title: "Acme Seed Round Pitch Deck",
    sourceType: "pdf",
    fileName: "Acme Seed Round Pitch Deck.pdf",
    fileType: "pdf",
    fileSize: 4_200_000,
    pageCount: 18,
    status: "ready",
    createdAt: "2026-06-18T09:30:00Z",
    updatedAt: "2026-06-18T09:45:00Z",
  },
  {
    id: "doc_2",
    title: "Financial Model 2026-2028",
    sourceType: "xlsx",
    fileName: "Financial Model 2026-2028.xlsx",
    fileType: "xlsx",
    fileSize: 1_800_000,
    pageCount: 12,
    status: "ready",
    createdAt: "2026-06-17T14:20:00Z",
    updatedAt: "2026-06-17T14:25:00Z",
  },
];

async function initI18n() {
  const instance = i18n.createInstance();
  const dealRoomsJson = JSON.parse(readFileSync(resolve(__dirname, "../../i18n/locales/en/dealRooms.json"), "utf-8"));
  const commonJson = JSON.parse(readFileSync(resolve(__dirname, "../../i18n/locales/en/common.json"), "utf-8"));
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["dealRooms", "common"],
    defaultNS: "dealRooms",
    resources: { en: { dealRooms: dealRoomsJson, common: commonJson } },
    interpolation: { escapeValue: false },
  });
  return instance;
}

async function renderPage(initialEntry = "/acme/deal-rooms/room-1") {
  const i18nInstance = await initI18n();
  let result!: ReturnType<typeof render>;
  await act(async () => {
    result = render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter initialEntries={[initialEntry]}>
          <Routes>
            <Route path=":workspaceSlug/deal-rooms/:roomId" element={<DealRoomDetailPage />} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>
    );
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  return result;
}

Object.defineProperty(window, "matchMedia", {
  writable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
});

describe("DealRoomDetailPage", () => {
  beforeEach(() => {
    getDealRoomByIdMock.mockReset();
    getDealRoomTemplatesMock.mockReset();
    getDocumentsMock.mockReset();
    uploadDocumentMock.mockReset();
    addDealRoomDocumentMock.mockReset();
    createDealRoomFolderMock.mockReset();
    getDocumentByIdMock.mockReset();
    getDealRoomLinksMock.mockReset();
    getLinkAnalyticsMock.mockReset();
    listLinkQuestionsMock.mockReset();
    getDealRoomTemplatesMock.mockResolvedValue({ data: mockTemplates });
    getDocumentsMock.mockResolvedValue({ data: mockWorkspaceDocs });
    getDealRoomLinksMock.mockResolvedValue({ data: [] });
    getLinkAnalyticsMock.mockResolvedValue({ data: { access_code_contacts: [] } });
    listLinkQuestionsMock.mockResolvedValue({ data: [] });
  });

  it("renders loading skeleton", async () => {
    getDealRoomByIdMock.mockReturnValue(new Promise(() => {}));
    await renderPage();
    expect(document.querySelector("[aria-busy='true']")).toBeInTheDocument();
  });

  it("renders deal room details, folders and hides empty upload dashboard", async () => {
    getDealRoomByIdMock.mockResolvedValue(mockRoom);
    await renderPage();

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Series A Data Room" })).toBeInTheDocument();
    });

    expect(screen.getByText("Due diligence materials")).toBeInTheDocument();
    expect(screen.getByText("01 Pitch Deck")).toBeInTheDocument();
    expect(screen.getByText("02 Financials")).toBeInTheDocument();
    expect(screen.queryByTestId("deal-room-command-strip")).not.toBeInTheDocument();
    expect(screen.queryByTestId("deal-room-readiness")).not.toBeInTheDocument();
    expect(screen.queryByText("Folder structure")).not.toBeInTheDocument();
    expect(screen.queryByTestId("upload-progress-popup")).not.toBeInTheDocument();
  });

  it("switches to participants tab and shows links section", async () => {
    getDealRoomByIdMock.mockResolvedValue(mockRoom);
    await renderPage("/acme/deal-rooms/room-1?tab=participants");

    await waitFor(() => {
      expect(screen.getByText(/no links found/i)).toBeInTheDocument();
    });
    expect(screen.queryByRole("heading", { name: "Series A Data Room" })).not.toBeInTheDocument();
    expect(screen.queryByText("Due diligence materials")).not.toBeInTheDocument();
    expect(screen.queryByText("Invitees")).not.toBeInTheDocument();
  });

  it("shows error and retries on failure", async () => {
    getDealRoomByIdMock.mockRejectedValue(new Error("network error"));
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("network error")).toBeInTheDocument();
    });

    getDealRoomByIdMock.mockResolvedValue(mockRoom);
    fireEvent.click(screen.getByRole("button", { name: /retry/i }));

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Series A Data Room" })).toBeInTheDocument();
    });
  });

  it("uploads a file directly to a folder via the folder upload icon", async () => {
    getDealRoomByIdMock.mockResolvedValue(mockRoom);
    uploadDocumentMock.mockResolvedValue({
      id: "doc_new",
      title: "Uploaded File.pdf",
      sourceType: "pdf",
      status: "processing",
    });
    addDealRoomDocumentMock.mockResolvedValue({});
    getDocumentByIdMock.mockResolvedValue({
      id: "doc_new",
      title: "Uploaded File.pdf",
      sourceType: "pdf",
      fileName: "Uploaded File.pdf",
      fileType: "pdf",
      fileSize: 1_000,
      pageCount: 1,
      status: "ready",
      createdAt: "2026-06-20T10:00:00Z",
      updatedAt: "2026-06-20T10:00:00Z",
    });
    await renderPage();

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Series A Data Room" })).toBeInTheDocument();
    });

    const folderRow = screen.getByText("01 Pitch Deck").closest("[role='button']") as HTMLElement;
    expect(folderRow).toBeInTheDocument();

    // Reveal action icons by hovering the folder row.
    fireEvent.mouseEnter(folderRow);

    const actionsButton = within(folderRow).getByRole("button", { name: /actions for 01 pitch deck/i });
    fireEvent.click(actionsButton);

    const uploadButton = screen.getByText(/^add file$/i);
    expect(uploadButton).toBeInTheDocument();

    // Trigger the hidden file input by clicking the upload icon.
    fireEvent.click(uploadButton);

    const fileInput = document.querySelector("[data-testid='folder-upload-input-/pitch']") as HTMLInputElement;
    expect(fileInput).toBeInTheDocument();

    const file = new File(["pdf content"], "Uploaded File.pdf", { type: "application/pdf" });
    await act(async () => {
      fireEvent.change(fileInput, { target: { files: [file] } });
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    await waitFor(() => {
      expect(uploadDocumentMock).toHaveBeenCalledWith(file, undefined, { skipEmbedding: true });
    });
    expect(addDealRoomDocumentMock).toHaveBeenCalledWith("room-1", {
      document_id: "doc_new",
      folder_path: "/pitch",
      sort_order: 1,
    });
  });

  it("expands a folder when starting to create a subfolder", async () => {
    getDealRoomByIdMock.mockResolvedValue(mockRoom);
    createDealRoomFolderMock.mockResolvedValue({});
    await renderPage();

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Series A Data Room" })).toBeInTheDocument();
    });

    const folderRow = screen.getByText("02 Financials").closest("[role='button']") as HTMLElement;
    expect(folderRow).toBeInTheDocument();

    fireEvent.contextMenu(folderRow);
    const newSubfolderButton = screen.getByText(/new subfolder/i);
    expect(newSubfolderButton).toBeInTheDocument();

    fireEvent.click(newSubfolderButton);

    await waitFor(() => {
      expect(screen.getByPlaceholderText(/folder name/i)).toBeInTheDocument();
    });
  });

  it("shows centered floating upload progress bar while uploading", async () => {
    getDealRoomByIdMock.mockResolvedValue(mockRoom);
    uploadDocumentMock.mockResolvedValue({
      id: "doc_new",
      title: "Uploaded File.pdf",
      sourceType: "pdf",
      status: "processing",
    });
    addDealRoomDocumentMock.mockResolvedValue({});
    getDocumentByIdMock.mockResolvedValue({
      id: "doc_new",
      title: "Uploaded File.pdf",
      sourceType: "pdf",
      fileName: "Uploaded File.pdf",
      fileType: "pdf",
      fileSize: 1_000,
      pageCount: 1,
      status: "ready",
      createdAt: "2026-06-20T10:00:00Z",
      updatedAt: "2026-06-20T10:00:00Z",
    });
    await renderPage();

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Series A Data Room" })).toBeInTheDocument();
    });

    const folderRow = screen.getByText("01 Pitch Deck").closest("[role='button']") as HTMLElement;
    const actionsButton = within(folderRow).getByRole("button", { name: /actions for 01 pitch deck/i });
    fireEvent.click(actionsButton);
    const uploadButton = screen.getByText(/^add file$/i);
    fireEvent.click(uploadButton);

    const fileInput = document.querySelector("[data-testid='folder-upload-input-/pitch']") as HTMLInputElement;
    const file = new File(["pdf content"], "Uploaded File.pdf", { type: "application/pdf" });
    await act(async () => {
      fireEvent.change(fileInput, { target: { files: [file] } });
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    const popup = await waitFor(() => screen.getByTestId("upload-progress-popup"));
    expect(popup).toBeInTheDocument();
    await waitFor(() => {
      expect(within(popup).getByText("100%")).toBeInTheDocument();
    });
    // Refetch is triggered so the folder tree will reflect the new document.
    expect(getDealRoomByIdMock).toHaveBeenCalledTimes(2);
  });

  it("keeps the deal room rendered while refetching after an upload finishes", async () => {
    let callCount = 0;
    getDealRoomByIdMock.mockImplementation(() => {
      callCount++;
      if (callCount === 1) {
        return Promise.resolve(mockRoom);
      }
      // Slow down the background refetch so we can verify the page does not
      // flash back to the loading skeleton while the upload overlay is shown.
      return new Promise((resolve) => setTimeout(() => resolve(mockRoom), 800));
    });
    uploadDocumentMock.mockResolvedValue({
      id: "doc_new",
      title: "Uploaded File.pdf",
      sourceType: "pdf",
      status: "processing",
    });
    addDealRoomDocumentMock.mockResolvedValue({});
    getDocumentByIdMock.mockResolvedValue({
      id: "doc_new",
      title: "Uploaded File.pdf",
      sourceType: "pdf",
      fileName: "Uploaded File.pdf",
      fileType: "pdf",
      fileSize: 1_000,
      pageCount: 1,
      status: "ready",
      createdAt: "2026-06-20T10:00:00Z",
      updatedAt: "2026-06-20T10:00:00Z",
    });
    await renderPage();

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Series A Data Room" })).toBeInTheDocument();
    });

    const folderRow = screen.getByText("01 Pitch Deck").closest("[role='button']") as HTMLElement;
    const actionsButton = within(folderRow).getByRole("button", { name: /actions for 01 pitch deck/i });
    fireEvent.click(actionsButton);
    const uploadButton = screen.getByText(/^add file$/i);
    fireEvent.click(uploadButton);

    const fileInput = document.querySelector("[data-testid='folder-upload-input-/pitch']") as HTMLInputElement;
    const file = new File(["pdf content"], "Uploaded File.pdf", { type: "application/pdf" });
    await act(async () => {
      fireEvent.change(fileInput, { target: { files: [file] } });
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    // The upload overlay should appear and the page content must stay visible
    // (no loading skeleton) while the background refetch is in flight.
    await waitFor(() => {
      expect(screen.getByTestId("upload-progress-popup")).toBeInTheDocument();
    });
    await waitFor(
      () => {
        expect(document.querySelector("[aria-busy='true']")).not.toBeInTheDocument();
        expect(screen.getByRole("heading", { name: "Series A Data Room" })).toBeInTheDocument();
      },
      { timeout: 1_000 }
    );
  }, 10_000);

  it("reflects real backend processing status in the progress bar", async () => {
    getDealRoomByIdMock.mockResolvedValue(mockRoom);
    uploadDocumentMock.mockResolvedValue({
      id: "doc_new",
      title: "Uploaded File.pdf",
      sourceType: "pdf",
      status: "processing",
    });
    addDealRoomDocumentMock.mockResolvedValue({});
    getDocumentByIdMock
      .mockResolvedValueOnce({
        id: "doc_new",
        title: "Uploaded File.pdf",
        sourceType: "pdf",
        fileName: "Uploaded File.pdf",
        fileType: "pdf",
        fileSize: 1_000,
        pageCount: 1,
        status: "processing",
        createdAt: "2026-06-20T10:00:00Z",
        updatedAt: "2026-06-20T10:00:00Z",
      })
      .mockResolvedValueOnce({
        id: "doc_new",
        title: "Uploaded File.pdf",
        sourceType: "pdf",
        fileName: "Uploaded File.pdf",
        fileType: "pdf",
        fileSize: 1_000,
        pageCount: 1,
        status: "ready",
        createdAt: "2026-06-20T10:00:00Z",
        updatedAt: "2026-06-20T10:00:00Z",
      });
    await renderPage();

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Series A Data Room" })).toBeInTheDocument();
    });

    const folderRow = screen.getByText("01 Pitch Deck").closest("[role='button']") as HTMLElement;
    const actionsButton = within(folderRow).getByRole("button", { name: /actions for 01 pitch deck/i });
    fireEvent.click(actionsButton);
    const uploadButton = screen.getByText(/^add file$/i);
    fireEvent.click(uploadButton);

    const fileInput = document.querySelector("[data-testid='folder-upload-input-/pitch']") as HTMLInputElement;
    const file = new File(["pdf content"], "Uploaded File.pdf", { type: "application/pdf" });
    await act(async () => {
      fireEvent.change(fileInput, { target: { files: [file] } });
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    const popup = await waitFor(() => screen.getByTestId("upload-progress-popup"));
    await waitFor(() => {
      expect(within(popup).getByText("95%")).toBeInTheDocument();
    });

    await waitFor(
      () => {
        expect(within(popup).getByText("100%")).toBeInTheDocument();
      },
      { timeout: 6_000 }
    );
  }, 10_000);
});

