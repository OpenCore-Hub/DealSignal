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
import { useWorkspaceAccess } from "@/hooks/useWorkspaceAccess";
import { ApiError } from "@/lib/apiClient";
import type { DealRoom, DealRoomFolder, DealRoomFolderDocs, DealRoomMember, DealRoomTemplate, Document } from "@/types";

const __dirname = dirname(fileURLToPath(import.meta.url));

const {
  getDealRoomByIdMock,
  getDealRoomTemplatesMock,
  getDocumentsMock,
  getDocumentByIdMock,
  uploadDealRoomDocumentMock,
  addDealRoomDocumentMock,
  createDealRoomFolderMock,
  getDealRoomLinksMock,
  getLinkAnalyticsMock,
  listRoomAskMock,
  getDealRoomKnowledgeMock,
  getDealRoomAnalyticsMock,
  getDealRoomMembersMock,
  logoutMock,
} = vi.hoisted(() => ({
  getDealRoomByIdMock: vi.fn(),
  getDealRoomTemplatesMock: vi.fn(),
  getDocumentsMock: vi.fn(),
  getDocumentByIdMock: vi.fn(),
  uploadDealRoomDocumentMock: vi.fn(),
  addDealRoomDocumentMock: vi.fn(),
  createDealRoomFolderMock: vi.fn(),
  getDealRoomLinksMock: vi.fn(),
  getLinkAnalyticsMock: vi.fn(),
  listRoomAskMock: vi.fn(),
  getDealRoomKnowledgeMock: vi.fn(),
  getDealRoomAnalyticsMock: vi.fn(),
  getDealRoomMembersMock: vi.fn(),
  logoutMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getDealRoomById: getDealRoomByIdMock,
    getDealRoomTemplates: getDealRoomTemplatesMock,
    getDocuments: getDocumentsMock,
    getDocumentById: getDocumentByIdMock,
    checkDocumentExists: vi.fn().mockResolvedValue({ exists: false }),
    checkDealRoomUploadExists: vi.fn().mockResolvedValue({ exists: false, replaceable: false }),
    uploadDealRoomDocument: uploadDealRoomDocumentMock,
    addDealRoomDocument: addDealRoomDocumentMock,
    removeDealRoomDocument: vi.fn().mockResolvedValue(undefined),
    createDealRoomFolder: createDealRoomFolderMock,
    getDealRoomLinks: getDealRoomLinksMock,
    getLinkAnalytics: getLinkAnalyticsMock,
    listRoomAsk: listRoomAskMock,
    getDealRoomKnowledge: getDealRoomKnowledgeMock,
    getDealRoomAnalytics: getDealRoomAnalyticsMock,
    syncDealRoomKnowledge: vi.fn().mockResolvedValue(undefined),
    queryDealRoomKnowledge: vi.fn(),
    getActiveDealRoomKnowledgeSession: vi.fn().mockResolvedValue({
      session: null,
      turns: [],
    }),
    listDealRoomKnowledgeSessions: vi.fn().mockResolvedValue({ items: [] }),
    getDealRoomKnowledgeSession: vi.fn().mockResolvedValue({
      session: null,
      turns: [],
    }),
    createDealRoomKnowledgeSession: vi.fn(),
    queryDealRoomKnowledgeSession: vi.fn(),
    streamDealRoomKnowledgeSession: vi.fn(),
    closeDealRoomKnowledgeSession: vi.fn(),
    upsertDealRoomKnowledgeTurnFeedback: vi.fn(),
    suggestDealRoomKnowledgeFollowUps: vi.fn().mockResolvedValue({
      source: "template",
      items: [],
    }),
    recordDealRoomKnowledgeDeskEvent: vi.fn().mockResolvedValue(undefined),
    getDealRoomKnowledgeMissionProgress: vi.fn().mockResolvedValue({
      packId: "financing_dd_v1",
      title: "Financing due diligence",
      source: "template_default",
      covered: 0,
      total: 0,
      items: [],
    }),
    listDealRoomKnowledgeMissions: vi.fn().mockResolvedValue({ items: [] }),
    setDealRoomKnowledgeMission: vi.fn(),
    getDealRoomKnowledgeOps: vi.fn().mockResolvedValue({
      scope: "workspace",
      windowHours: 24,
      turnsTotal: 0,
      turnsByStatus: {},
      avgDurationMs: 0,
      p95DurationMs: 0,
      costUnitsTotal: 0,
      refusalsByKind: {},
      judgmentsByKind: {},
      evalCandidatesByStatus: {},
      pendingEvalCandidates: 0,
      answersQuota: { used: 0, limit: 100, windowHours: 24 },
      coldArchiveCount: 0,
      retentionDays: 90,
    }),
    listDealRoomKnowledgeEvalCandidates: vi.fn().mockResolvedValue({ items: [] }),
    reviewDealRoomKnowledgeEvalCandidate: vi.fn(),
    exportDealRoomKnowledgeEvalCandidates: vi.fn().mockResolvedValue({
      description: "Accepted",
      seeds: [],
    }),
    listDealRoomKnowledgeArchives: vi.fn().mockResolvedValue({ items: [] }),
    getDealRoomKnowledgeArchive: vi.fn().mockRejectedValue(new Error("not found")),
    getDealRoomAccessPolicy: vi.fn().mockResolvedValue({ data: null }),
    getDealRoomAccessRequests: vi.fn().mockResolvedValue({ data: [] }),
    getPendingLinkAccessRequests: vi.fn().mockResolvedValue({ data: [] }),
    getBillingInfo: vi.fn().mockResolvedValue(null),
    getDealRoomMembers: getDealRoomMembersMock,
    inviteDealRoomMember: vi.fn(),
    updateDealRoomMemberRole: vi.fn(),
    removeDealRoomMember: vi.fn(),
    listNDATemplates: vi.fn().mockResolvedValue({ data: [] }),
    logout: logoutMock,
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
  isAdmin: true,
  canContribute: true,
  roomRole: "owner",
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
  const documentsJson = JSON.parse(readFileSync(resolve(__dirname, "../../i18n/locales/en/documents.json"), "utf-8"));
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["dealRooms", "common", "documents"],
    defaultNS: "dealRooms",
    resources: {
      en: { dealRooms: dealRoomsJson, common: commonJson, documents: documentsJson },
    },
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
            <Route path=":workspaceSlug/deal-rooms" element={<div>rooms-list</div>} />
            <Route path="/login" element={<div>login-page</div>} />
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
    vi.mocked(useWorkspaceAccess).mockReturnValue({
      role: "member",
      loading: false,
      canRead: true,
      canWrite: true,
      canManage: false,
      isGuest: false,
    });
    getDealRoomByIdMock.mockReset();
    getDealRoomTemplatesMock.mockReset();
    getDocumentsMock.mockReset();
    uploadDealRoomDocumentMock.mockReset();
    addDealRoomDocumentMock.mockReset();
    createDealRoomFolderMock.mockReset();
    getDocumentByIdMock.mockReset();
    getDealRoomLinksMock.mockReset();
    getLinkAnalyticsMock.mockReset();
    listRoomAskMock.mockReset();
    getDealRoomKnowledgeMock.mockReset();
    getDealRoomAnalyticsMock.mockReset();
    getDealRoomMembersMock.mockReset();
    logoutMock.mockReset();
    getDealRoomTemplatesMock.mockResolvedValue({ data: mockTemplates });
    getDocumentsMock.mockResolvedValue({ data: mockWorkspaceDocs });
    getDealRoomLinksMock.mockResolvedValue({ data: [] });
    getLinkAnalyticsMock.mockResolvedValue({ data: { access_code_contacts: [] } });
    listRoomAskMock.mockResolvedValue({ data: [] });
    getDealRoomKnowledgeMock.mockResolvedValue({
      enabled: true,
      status: "ready",
      documents: [],
    });
    getDealRoomAnalyticsMock.mockResolvedValue({
      totalViews: 0,
      uniqueVisitors: 0,
      activeLinkCount: 0,
      documentCount: 0,
      viewsOverTime: [],
      recentVisitors: [],
    });
    getDealRoomMembersMock.mockResolvedValue({ data: mockMembers });
  });

  async function uploadViaPageFileInput(...files: File[]) {
    // Drive the page-level multi-file input used by deal-room uploads.
    // Names that do not match a folder recommendation fall through to /pitch.
    const fileInput = await screen.findByTestId("deal-room-page-upload-input");
    await act(async () => {
      Object.defineProperty(fileInput, "files", {
        configurable: true,
        value: files,
      });
      fireEvent.change(fileInput);
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
  }

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
    // Workspace catalog must stay off the enter-room critical path.
    expect(getDocumentsMock).not.toHaveBeenCalled();
  });

  it("switches to links tab and shows links section", async () => {
    getDealRoomByIdMock.mockResolvedValue(mockRoom);
    await renderPage("/acme/deal-rooms/room-1?tab=links");

    await waitFor(() => {
      expect(screen.getByText(/no links found/i)).toBeInTheDocument();
    });
    expect(screen.getByRole("heading", { name: "Series A Data Room" })).toBeInTheDocument();
    expect(screen.getByTestId("deal-room-page-tabs")).toBeInTheDocument();
    expect(screen.queryByText("Invitees")).not.toBeInTheDocument();
  });

  it("moves the active page tab to the first position", async () => {
    getDealRoomByIdMock.mockResolvedValue(mockRoom);
    await renderPage("/acme/deal-rooms/room-1?tab=knowledge");

    await waitFor(() => {
      expect(screen.getByTestId("deal-room-page-tabs")).toBeInTheDocument();
    });
    const tabs = screen.getByTestId("deal-room-page-tabs");
    const triggers = Array.from(tabs.querySelectorAll('[role="tab"]')).map(
      (el) => el.getAttribute("data-testid"),
    );
    expect(triggers[0]).toBe("deal-room-page-tab-knowledge");
  });

  it("shows access for oversight viewers as a read-only tab and still shows knowledge", async () => {
    vi.mocked(useWorkspaceAccess).mockReturnValue({
      role: "admin",
      loading: false,
      canRead: true,
      canWrite: true,
      canManage: true,
      isGuest: false,
    });
    getDealRoomByIdMock.mockResolvedValue({
      ...mockRoom,
      isAdmin: false,
      canContribute: false,
      oversight: true,
    });
    await renderPage("/acme/deal-rooms/room-1?tab=knowledge");

    await waitFor(() => {
      expect(screen.getByTestId("deal-room-page-tabs")).toBeInTheDocument();
    });
    expect(screen.getByTestId("deal-room-page-tab-knowledge")).toBeInTheDocument();
    expect(screen.getByTestId("deal-room-page-tab-access")).toBeInTheDocument();
    expect(screen.getByTestId("deal-room-page-tab-documents")).toBeInTheDocument();
    expect(screen.getByTestId("deal-room-page-tab-members")).toBeInTheDocument();
    expect(screen.getByTestId("deal-room-oversight-banner")).toBeInTheDocument();
  });

  it("shows access for a workspace guest who is a room admin", async () => {
    vi.mocked(useWorkspaceAccess).mockReturnValue({
      role: "guest",
      loading: false,
      canRead: true,
      canWrite: false,
      canManage: false,
      isGuest: true,
    });
    getDealRoomByIdMock.mockResolvedValue({
      ...mockRoom,
      isAdmin: true,
      canContribute: true,
      oversight: false,
    });
    await renderPage("/acme/deal-rooms/room-1?tab=access");

    await waitFor(() => {
      expect(screen.getByTestId("deal-room-page-tab-access")).toBeInTheDocument();
    });
    expect(screen.getByTestId("deal-room-page-tab-knowledge")).toBeInTheDocument();
  });

  it("shows members as a page tab and lets room managers invite", async () => {
    getDealRoomByIdMock.mockResolvedValue(mockRoom);
    await renderPage("/acme/deal-rooms/room-1?tab=members");

    await waitFor(() => {
      expect(screen.getByTestId("deal-room-members-tab")).toBeInTheDocument();
    });
    expect(screen.getByTestId("deal-room-page-tab-members")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /invite members/i })).toBeInTheDocument();
    expect(await screen.findByText("John Doe")).toBeInTheDocument();
  });

  it("shows members read-only for oversight without invite", async () => {
    getDealRoomByIdMock.mockResolvedValue({
      ...mockRoom,
      isAdmin: false,
      canContribute: false,
      oversight: true,
      roomRole: "",
    });
    await renderPage("/acme/deal-rooms/room-1?tab=members");

    await waitFor(() => {
      expect(screen.getByTestId("deal-room-members-tab")).toBeInTheDocument();
    });
    expect(screen.getByTestId("deal-room-page-tab-members")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /invite members/i })).not.toBeInTheDocument();
    expect(await screen.findByText("John Doe")).toBeInTheDocument();
  });

  it("shows members for a room member without invite", async () => {
    getDealRoomByIdMock.mockResolvedValue({
      ...mockRoom,
      isAdmin: false,
      canContribute: true,
      oversight: false,
      roomRole: "member",
    });
    await renderPage("/acme/deal-rooms/room-1?tab=members");

    await waitFor(() => {
      expect(screen.getByTestId("deal-room-members-tab")).toBeInTheDocument();
    });
    expect(screen.queryByRole("button", { name: /invite members/i })).not.toBeInTheDocument();
  });

  it("keeps settings as policy only and points to the members tab", async () => {
    getDealRoomByIdMock.mockResolvedValue(mockRoom);
    await renderPage("/acme/deal-rooms/room-1?tab=settings");

    await waitFor(() => {
      expect(screen.getByTestId("deal-room-settings-tab")).toBeInTheDocument();
    });
    expect(screen.queryByTestId("deal-room-members-panel")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /invite members/i })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /open members/i })).toBeInTheDocument();
  });

  it("lets a room member upload from the folder tree without manage chrome", async () => {
    getDealRoomByIdMock.mockResolvedValue({
      ...mockRoom,
      isAdmin: false,
      canContribute: true,
      oversight: false,
      roomRole: "member",
    });
    await renderPage();

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Series A Data Room" })).toBeInTheDocument();
    });

    expect(screen.queryByTestId("folder-tree-create-directory")).not.toBeInTheDocument();
    const folderRow = screen.getByText("02 Financials").closest("[role='button']") as HTMLElement;
    fireEvent.click(within(folderRow).getByRole("checkbox", { name: "02 Financials" }));
    expect(await screen.findByTestId("folder-tree-bulk-upload")).toBeInTheDocument();
    expect(screen.queryByTestId("folder-tree-bulk-create-subfolder")).not.toBeInTheDocument();
    expect(screen.queryByTestId("folder-tree-bulk-remove-files")).not.toBeInTheDocument();
  });

  it("migrates legacy participants tab to links", async () => {
    getDealRoomByIdMock.mockResolvedValue(mockRoom);
    await renderPage("/acme/deal-rooms/room-1?tab=participants");

    await waitFor(() => {
      expect(screen.getByText(/no links found/i)).toBeInTheDocument();
    });
  });

  it("shows error and retries on failure", async () => {
    getDealRoomByIdMock.mockRejectedValue(new Error("network error"));
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Failed to load")).toBeInTheDocument();
    });

    getDealRoomByIdMock.mockResolvedValue(mockRoom);
    fireEvent.click(screen.getByRole("button", { name: /retry/i }));

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Series A Data Room" })).toBeInTheDocument();
    });
  });

  it("offers switch account instead of retry when the room is forbidden", async () => {
    getDealRoomByIdMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "forbidden",
        message: "forbidden",
        requestId: "r1",
      }),
    );
    logoutMock.mockResolvedValue(undefined);
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText(/not available on this account/i)).toBeInTheDocument();
    });
    expect(screen.queryByRole("button", { name: /retry/i })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /switch account/i }));
    await waitFor(() => {
      expect(logoutMock).toHaveBeenCalled();
      expect(screen.getByText("login-page")).toBeInTheDocument();
    });
  });

  it("uploads a file to a folder via toolbar batch upload", async () => {
    getDealRoomByIdMock.mockResolvedValue(mockRoom);
    uploadDealRoomDocumentMock.mockResolvedValue({
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

    const file = new File(["pdf content"], "Uploaded File.pdf", { type: "application/pdf" });
    await uploadViaPageFileInput(file);

    await waitFor(() => {
      expect(uploadDealRoomDocumentMock).toHaveBeenCalledWith(
        "room-1",
        file,
        expect.objectContaining({
          folderPath: "/pitch",
          sortOrder: 1,
          onUploadProgress: expect.any(Function),
        }),
      );
    });
    expect(addDealRoomDocumentMock).not.toHaveBeenCalled();
  });

  it("continues the page batch when one file fails", async () => {
    getDealRoomByIdMock.mockResolvedValue(mockRoom);
    uploadDealRoomDocumentMock
      .mockRejectedValueOnce(
        new ApiError({
          status: 500,
          code: "internal_error",
          message: "upload failed",
          requestId: "r1",
        }),
      )
      .mockResolvedValueOnce({
        id: "doc_ok",
        title: "b.pdf",
        sourceType: "pdf",
        status: "processing",
      });
    getDocumentByIdMock.mockResolvedValue({
      id: "doc_ok",
      title: "b.pdf",
      sourceType: "pdf",
      fileName: "b.pdf",
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

    const first = new File(["a"], "a.pdf", { type: "application/pdf" });
    const second = new File(["b"], "b.pdf", { type: "application/pdf" });
    await uploadViaPageFileInput(first, second);

    await waitFor(() => {
      expect(uploadDealRoomDocumentMock).toHaveBeenCalledTimes(2);
    });
    expect(uploadDealRoomDocumentMock).toHaveBeenNthCalledWith(
      1,
      "room-1",
      first,
      expect.objectContaining({ folderPath: "/pitch" }),
    );
    expect(uploadDealRoomDocumentMock).toHaveBeenNthCalledWith(
      2,
      "room-1",
      second,
      expect.objectContaining({ folderPath: "/pitch" }),
    );
  });

  it("creates a subdirectory from the selection toolbar", async () => {
    getDealRoomByIdMock.mockResolvedValue(mockRoom);
    createDealRoomFolderMock.mockResolvedValue({});
    await renderPage();

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Series A Data Room" })).toBeInTheDocument();
    });

    const folderRow = screen.getByText("02 Financials").closest("[role='button']") as HTMLElement;
    fireEvent.click(within(folderRow).getByRole("checkbox", { name: "02 Financials" }));
    fireEvent.click(await screen.findByTestId("folder-tree-bulk-create-subfolder"));

    await waitFor(() => {
      expect(screen.getByPlaceholderText(/folder name/i)).toBeInTheDocument();
    });
  });

  it("shows centered floating upload progress bar while uploading", async () => {
    getDealRoomByIdMock.mockResolvedValue(mockRoom);
    uploadDealRoomDocumentMock.mockResolvedValue({
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

    const file = new File(["pdf content"], "Uploaded File.pdf", { type: "application/pdf" });
    await uploadViaPageFileInput(file);

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
    uploadDealRoomDocumentMock.mockResolvedValue({
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

    const file = new File(["pdf content"], "Uploaded File.pdf", { type: "application/pdf" });
    await uploadViaPageFileInput(file);

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
    uploadDealRoomDocumentMock.mockResolvedValue({
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

    const file = new File(["pdf content"], "Uploaded File.pdf", { type: "application/pdf" });
    await uploadViaPageFileInput(file);

    const popup = await waitFor(() => screen.getByTestId("upload-progress-popup"));
    await waitFor(() => {
      expect(within(popup).getByText("50%")).toBeInTheDocument();
    });

    await waitFor(
      () => {
        expect(within(popup).getByText("100%")).toBeInTheDocument();
      },
      { timeout: 6_000 }
    );
  }, 10_000);
});

