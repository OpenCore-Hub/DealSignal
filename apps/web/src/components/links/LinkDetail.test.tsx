// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { I18nextProvider } from "react-i18next";
import { LinkDetail } from "./LinkDetail";
import { createTestI18n } from "@/i18n/test-utils";
import enCommon from "@/i18n/locales/en/common.json";
import enLinks from "@/i18n/locales/en/links.json";
import enLinkShare from "@/i18n/locales/en/linkShare.json";
import type { Link } from "@/types";

const {
  getLinkByIdMock,
  getAccessLogsMock,
  getLinkAnalyticsMock,
  getDocumentByIdMock,
  listLinkAskMock,
} = vi.hoisted(() => ({
  getLinkByIdMock: vi.fn(),
  getAccessLogsMock: vi.fn(),
  getLinkAnalyticsMock: vi.fn(),
  getDocumentByIdMock: vi.fn(),
  listLinkAskMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getLinkById: getLinkByIdMock,
    getAccessLogs: getAccessLogsMock,
    getLinkAnalytics: getLinkAnalyticsMock,
    getDocumentById: getDocumentByIdMock,
    listLinkAsk: listLinkAskMock,
    answerAskTurn: vi.fn(),
  },
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("@/hooks/useWorkspaceAccess", () => ({
  useWorkspaceAccess: () => ({
    canWrite: true,
    canManage: true,
    loading: false,
    role: "owner",
    canRead: true,
    isGuest: false,
  }),
}));

vi.mock("@/components/common/PageDurationChart", () => ({
  PageDurationChart: () => null,
}));

vi.mock("./LinkAccessLog", () => ({
  LinkAccessLog: () => null,
}));

function libraryLink(): Link {
  return {
    id: "link_lib",
    documentIds: ["doc-1"],
    folderPaths: [],
    documentTitle: "Deck",
    shortUrl: "https://example.com/s/abc",
    accessCount: 0,
    heatLevel: "cold",
    createdAt: "2026-08-01T00:00:00Z",
    isBundle: false,
    documents: [],
    qaEnabled: true,
    canManageAsk: true,
  };
}

const emptyAnalytics = {
  total_views: 0,
  unique_visitors: 0,
  download_attempts: 0,
  views_over_time: [],
  average_duration_seconds: 0,
  recent_visitors: [],
  key_pages: [],
  page_durations: [],
  qa_records: [],
};

async function renderDetail(path: string) {
  const i18n = await createTestI18n({
    common: enCommon as unknown as Record<string, unknown>,
    links: enLinks as unknown as Record<string, unknown>,
    linkShare: enLinkShare as unknown as Record<string, unknown>,
  });
  return render(
    <MemoryRouter initialEntries={[path]}>
      <I18nextProvider i18n={i18n}>
        <Routes>
          <Route path="/:workspaceSlug/links/:linkId" element={<LinkDetail />} />
        </Routes>
      </I18nextProvider>
    </MemoryRouter>,
  );
}

describe("LinkDetail Ask inbox", () => {
  beforeEach(() => {
    getLinkByIdMock.mockReset().mockResolvedValue(libraryLink());
    getAccessLogsMock.mockReset().mockResolvedValue({ data: [] });
    getLinkAnalyticsMock.mockReset().mockResolvedValue({ data: emptyAnalytics });
    getDocumentByIdMock.mockReset().mockResolvedValue(null);
    listLinkAskMock.mockReset().mockResolvedValue({ data: [] });
  });

  it("mounts Ask inbox for a document-library link without dealRoomId", async () => {
    await renderDetail("/acme/links/link_lib?askInbox=needs_host");

    expect(await screen.findByText("Ask inbox")).toBeInTheDocument();
    expect(await screen.findByText(/No Ask questions yet/i)).toBeInTheDocument();
    expect(getLinkByIdMock).toHaveBeenCalledWith("link_lib");
  });
});
