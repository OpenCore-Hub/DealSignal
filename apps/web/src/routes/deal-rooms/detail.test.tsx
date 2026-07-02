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
import type { DealRoom, DealRoomFolder, DealRoomFolderDocs, DealRoomMember, DealRoomAccessRequest, DealRoomTemplate, Document } from "@/types";

const __dirname = dirname(fileURLToPath(import.meta.url));

const { getDealRoomByIdMock, getDealRoomTemplatesMock, getDocumentsMock, uploadDocumentMock, addDealRoomDocumentMock } = vi.hoisted(() => ({
  getDealRoomByIdMock: vi.fn(),
  getDealRoomTemplatesMock: vi.fn(),
  getDocumentsMock: vi.fn(),
  uploadDocumentMock: vi.fn(),
  addDealRoomDocumentMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getDealRoomById: getDealRoomByIdMock,
    getDealRoomTemplates: getDealRoomTemplatesMock,
    getDocuments: getDocumentsMock,
    uploadDocument: uploadDocumentMock,
    addDealRoomDocument: addDealRoomDocumentMock,
  },
}));

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
  { id: "rm_1", email: "john@acme.capital", role: "owner", nda_status: "signed", status: "active", name: "John Doe" },
];

const mockAccessRequests: DealRoomAccessRequest[] = [
  { id: "ra_1", email: "sarah@horizon.vc", status: "pending", reason: "Would like to review deck" },
];

const mockRoom: DealRoom = {
  id: "room-1",
  name: "Series A Data Room",
  description: "Due diligence materials",
  slug: "series-a-data-room",
  template: "series-a",
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
  accessRequests: mockAccessRequests,
};

const mockTemplates: DealRoomTemplate[] = [
  {
    id: "tpl-series-a",
    name: "Series A",
    description: "Growth-stage data room",
    scenario: "series-a",
    folderStructure: [{ name: "Financials" }],
    recommendedFiles: ["Pitch deck", "Financial model", "Cap table"],
    defaultPermissionLevel: "medium",
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

async function renderPage() {
  const i18nInstance = await initI18n();
  let result!: ReturnType<typeof render>;
  await act(async () => {
    result = render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter initialEntries={["/acme/deal-rooms/room-1"]}>
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

describe("DealRoomDetailPage", () => {
  beforeEach(() => {
    getDealRoomByIdMock.mockReset();
    getDealRoomTemplatesMock.mockReset();
    getDocumentsMock.mockReset();
    uploadDocumentMock.mockReset();
    addDealRoomDocumentMock.mockReset();
    getDealRoomTemplatesMock.mockResolvedValue({ data: mockTemplates });
    getDocumentsMock.mockResolvedValue({ data: mockWorkspaceDocs });
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
      expect(screen.getByText("Series A Data Room")).toBeInTheDocument();
    });

    expect(screen.getByText("Due diligence materials")).toBeInTheDocument();
    expect(screen.getByText("01 Pitch Deck")).toBeInTheDocument();
    expect(screen.getByText("02 Financials")).toBeInTheDocument();
    expect(screen.getByText("john@acme.capital")).toBeInTheDocument();
    expect(screen.getByText("sarah@horizon.vc")).toBeInTheDocument();
    expect(screen.queryByText("Upload progress")).not.toBeInTheDocument();
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
      expect(screen.getByText("Series A Data Room")).toBeInTheDocument();
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
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Series A Data Room")).toBeInTheDocument();
    });

    const folderRow = screen.getByText("01 Pitch Deck").closest("[role='button']") as HTMLElement;
    expect(folderRow).toBeInTheDocument();

    // Reveal action icons by hovering the folder row.
    fireEvent.mouseEnter(folderRow);

    const uploadButton = within(folderRow).getByRole("button", { name: /add file/i });
    expect(uploadButton).toBeInTheDocument();

    // Trigger the hidden file input by clicking the upload icon.
    fireEvent.click(uploadButton);

    const fileInput = document.querySelector("[data-testid='folder-upload-input']") as HTMLInputElement;
    expect(fileInput).toBeInTheDocument();

    const file = new File(["pdf content"], "Uploaded File.pdf", { type: "application/pdf" });
    await act(async () => {
      fireEvent.change(fileInput, { target: { files: [file] } });
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    await waitFor(() => {
      expect(uploadDocumentMock).toHaveBeenCalledWith(file);
    });
    expect(addDealRoomDocumentMock).toHaveBeenCalledWith("room-1", {
      document_id: "doc_new",
      folder_path: "/pitch",
      sort_order: 1,
    });
  });

  it("shows upload progress dashboard with folder path after uploading", async () => {
    getDealRoomByIdMock.mockResolvedValue(mockRoom);
    uploadDocumentMock.mockResolvedValue({
      id: "doc_new",
      title: "Uploaded File.pdf",
      sourceType: "pdf",
      status: "processing",
    });
    addDealRoomDocumentMock.mockResolvedValue({});
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Series A Data Room")).toBeInTheDocument();
    });

    const folderRow = screen.getByText("01 Pitch Deck").closest("[role='button']") as HTMLElement;
    fireEvent.mouseEnter(folderRow);
    const uploadButton = within(folderRow).getByRole("button", { name: /add file/i });
    fireEvent.click(uploadButton);

    const fileInput = document.querySelector("[data-testid='folder-upload-input']") as HTMLInputElement;
    const file = new File(["pdf content"], "Uploaded File.pdf", { type: "application/pdf" });
    await act(async () => {
      fireEvent.change(fileInput, { target: { files: [file] } });
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    await waitFor(() => {
      expect(screen.getByText("Upload progress")).toBeInTheDocument();
    });

    const dashboardCard = screen.getByText("Upload progress").closest("[data-slot='card']") as HTMLElement;
    expect(dashboardCard).toBeInTheDocument();
    expect(within(dashboardCard).getByText("Uploaded File.pdf")).toBeInTheDocument();
    expect(within(dashboardCard).getByText("01 Pitch Deck")).toBeInTheDocument();
    expect(within(dashboardCard).getByText("Uploaded")).toBeInTheDocument();
    // Refetch is triggered so the folder tree will reflect the new document.
    expect(getDealRoomByIdMock).toHaveBeenCalledTimes(2);
  });
});
