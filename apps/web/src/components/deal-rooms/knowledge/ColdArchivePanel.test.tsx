// @vitest-environment jsdom
import { describe, expect, it, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { createTestI18n } from "@/i18n/test-utils";
import { ColdArchivePanel } from "./ColdArchivePanel";

const listArchives = vi.fn();
const getArchive = vi.fn();
const downloadPack = vi.fn();

vi.mock("@/lib/api", () => ({
  api: {
    listDealRoomKnowledgeArchives: (...args: unknown[]) => listArchives(...args),
    getDealRoomKnowledgeArchive: (...args: unknown[]) => getArchive(...args),
  },
}));

vi.mock("@/lib/knowledge/downloadDiligence", () => ({
  downloadDiligencePack: (...args: unknown[]) => downloadPack(...args),
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

const dealRooms = {
  "knowledge.archivesTitle": "Cold archives",
  "knowledge.archivesHint": "Read-only packs.",
  "knowledge.archivesLoading": "Loading…",
  "knowledge.archivesLoadFailed": "Failed",
  "knowledge.archivesUntitled": "Untitled",
  "knowledge.archivesMeta": "{{turns}} turns · {{status}} · {{when}}",
  "knowledge.archivesStatusCold": "cold",
  "knowledge.archivesStatusRestored": "viewed",
  "knowledge.archivesOpen": "View",
  "knowledge.archivesDownload": "Download",
  "knowledge.archivesOpenFailed": "Open failed",
  "knowledge.archivesDownloadSuccess": "Downloaded",
  "knowledge.archivesDownloadFailed": "Download failed",
  "knowledge.archivesPreviewTitle": "Preview",
  "knowledge.archivesPreviewClose": "Close",
  "knowledge.archivesPreviewMeta": "{{turns}} turns · {{fingerprint}}",
  "knowledge.archivesPreviewMore": "+{{count}} more",
};

describe("ColdArchivePanel", () => {
  beforeEach(() => {
    listArchives.mockReset();
    getArchive.mockReset();
    downloadPack.mockReset();
  });

  it("renders nothing when there are no archives", async () => {
    listArchives.mockResolvedValue({ items: [] });
    const i18n = await createTestI18n({ dealRooms });
    const { container } = render(
      <I18nextProvider i18n={i18n}>
        <ColdArchivePanel roomId="room-1" />
      </I18nextProvider>,
    );
    await waitFor(() => {
      expect(listArchives).toHaveBeenCalled();
    });
    await waitFor(() => {
      expect(container.querySelector('[data-testid="knowledge-cold-archives"]')).toBeNull();
    });
  });

  it("opens a read-only preview without mutating live sessions", async () => {
    listArchives.mockResolvedValue({
      items: [
        {
          id: "arch-1",
          workspaceId: "ws",
          roomId: "room-1",
          sessionId: "sess-1",
          title: "Old session",
          turnCount: 1,
          status: "cold",
          archivedAt: "2026-07-01T12:00:00Z",
        },
      ],
    });
    getArchive.mockResolvedValue({
      archive: {
        id: "arch-1",
        workspaceId: "ws",
        roomId: "room-1",
        sessionId: "sess-1",
        title: "Old session",
        turnCount: 1,
        status: "restored_readonly",
        archivedAt: "2026-07-01T12:00:00Z",
        corpusFingerprint: "abcdef0123456789",
      },
      pack: {
        schemaVersion: "1",
        exportedAt: "2026-07-01T12:00:00Z",
        workspaceId: "ws",
        roomId: "room-1",
        sessionId: "sess-1",
        corpusFingerprint: "abcdef0123456789",
        session: { id: "sess-1", status: "closed" },
        turns: [
          {
            id: "t1",
            sessionId: "sess-1",
            sequence: 1,
            question: "What was the cap?",
            answer: "Ten million.",
            refused: false,
            resultStatus: "answered",
            hits: [],
          },
        ],
      },
    });
    const i18n = await createTestI18n({ dealRooms });
    render(
      <I18nextProvider i18n={i18n}>
        <ColdArchivePanel roomId="room-1" />
      </I18nextProvider>,
    );
    await waitFor(() => {
      expect(screen.getByTestId("knowledge-cold-archives")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByTestId("knowledge-cold-archive-open-arch-1"));
    await waitFor(() => {
      expect(getArchive).toHaveBeenCalledWith("room-1", "arch-1");
    });
    await waitFor(() => {
      expect(screen.getByTestId("knowledge-cold-archive-preview")).toHaveTextContent(
        "What was the cap?",
      );
    });
  });

  it("downloads the diligence pack JSON", async () => {
    listArchives.mockResolvedValue({
      items: [
        {
          id: "arch-2",
          workspaceId: "ws",
          roomId: "room-1",
          sessionId: "sess-2",
          turnCount: 0,
          status: "cold",
          archivedAt: "2026-07-02T12:00:00Z",
        },
      ],
    });
    getArchive.mockResolvedValue({
      archive: {
        id: "arch-2",
        workspaceId: "ws",
        roomId: "room-1",
        sessionId: "sess-2",
        turnCount: 0,
        status: "restored_readonly",
        archivedAt: "2026-07-02T12:00:00Z",
      },
      pack: {
        schemaVersion: "1",
        exportedAt: "2026-07-02T12:00:00Z",
        workspaceId: "ws",
        roomId: "room-1",
        sessionId: "sess-2",
        session: { id: "sess-2", status: "closed" },
        turns: [],
      },
    });
    const i18n = await createTestI18n({ dealRooms });
    render(
      <I18nextProvider i18n={i18n}>
        <ColdArchivePanel roomId="room-1" />
      </I18nextProvider>,
    );
    await waitFor(() => {
      expect(screen.getByTestId("knowledge-cold-archive-download-arch-2")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByTestId("knowledge-cold-archive-download-arch-2"));
    await waitFor(() => {
      expect(downloadPack).toHaveBeenCalled();
    });
  });
});
