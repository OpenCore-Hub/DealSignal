// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { DealRoomDiligenceTab } from "./DealRoomDiligenceTab";
import { ApiError } from "@/lib/apiClient";
import type { DDCoverageSnapshot, Link } from "@/types";

const getDealRoomLinks = vi.fn();
const getDealRoomDocuments = vi.fn();
const getDealRoomKnowledgeBase = vi.fn();
const getDDCoverageSnapshot = vi.fn();
const startDDCoverageScan = vi.fn();
const getDDCoverageRun = vi.fn();
const getDDCoveragePack = vi.fn();
const listDDCoveragePacks = vi.fn();
const putDDCoveragePack = vi.fn();
const resetDDCoveragePack = vi.fn();
const startDDCrossCheck = vi.fn();
const getDDCrossCheckLatest = vi.fn();

vi.mock("@/lib/api", () => ({
  api: {
    getDealRoomLinks: (...args: unknown[]) => getDealRoomLinks(...args),
    getDealRoomDocuments: (...args: unknown[]) => getDealRoomDocuments(...args),
    getDealRoomKnowledgeBase: (...args: unknown[]) => getDealRoomKnowledgeBase(...args),
    getDDCoverageSnapshot: (...args: unknown[]) => getDDCoverageSnapshot(...args),
    startDDCoverageScan: (...args: unknown[]) => startDDCoverageScan(...args),
    getDDCoverageRun: (...args: unknown[]) => getDDCoverageRun(...args),
    getDDCoveragePack: (...args: unknown[]) => getDDCoveragePack(...args),
    listDDCoveragePacks: (...args: unknown[]) => listDDCoveragePacks(...args),
    putDDCoveragePack: (...args: unknown[]) => putDDCoveragePack(...args),
    resetDDCoveragePack: (...args: unknown[]) => resetDDCoveragePack(...args),
    startDDCrossCheck: (...args: unknown[]) => startDDCrossCheck(...args),
    getDDCrossCheckLatest: (...args: unknown[]) => getDDCrossCheckLatest(...args),
  },
}));

const links = [
  {
    id: "link-1",
    name: "Investor pack",
    documentTitle: "Deck",
  },
] as Link[];

const snapshot: DDCoverageSnapshot = {
  id: "snap-1",
  pack_id: "financing_dd_v1",
  pack_version: "1",
  scope: "room",
  run_id: "run-1",
  stale: true,
  coverage_rows: [
    {
      item_id: "cap_table",
      label: "Cap table",
      status: "supported",
      clues: [
        {
          chunk_id: "c1",
          document_id: "doc-1",
          page_number: 3,
          quote: "Cap table excerpt",
          score: 0.9,
          boxes: [],
        },
      ],
    },
    {
      item_id: "option_pool",
      label: "Option pool",
      status: "supported",
      value_type: "percent",
      extracted_value: "15%",
      clues: [
        {
          chunk_id: "c2",
          document_id: "doc-1",
          page_number: 4,
          quote: "Option pool reserved at 15%.",
          score: 0.88,
          boxes: [],
        },
      ],
    },
  ],
  created_at: new Date().toISOString(),
  updated_at: new Date().toISOString(),
};

async function renderTab() {
  const instance = i18n.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    resources: {
      en: {
        dealRooms: {
          diligence: {
            title: "Diligence coverage",
            description: "Scan checklist",
            disabled: "Diligence coverage is not enabled for this environment.",
            packSelectLabel: "Checklist pack",
            packs: {
              financing_dd_v1: "Financing DD",
              ma_redflag_v1: "M&A red flags",
            },
            packReadonly: "M&A red-flag pack is built-in and read-only (no room fork).",
            scopeLabel: "Scan scope",
            scopeRoom: "Entire deal room (KB)",
            scopeRoomHint: "Scope: room",
            scopeLinkHint: "Scope: share link",
            runScan: "Run scan",
            scanning: "Scanning…",
            scanStarted: "Scan queued",
            scanSucceeded: "Scan completed",
            scanFailed: "Scan failed",
            scanInProgress: "A scan is already running for this room",
            runStatus: "Run status: {{status}}",
            run: {
              queued: "Queued",
              running: "Running",
              succeeded: "Succeeded",
              failed: "Failed",
            },
            staleTitle: "Snapshot is out of date",
            staleDescription: "Rebuild KB then rescan",
            loading: "Loading coverage…",
            retry: "Retry",
            emptyTitle: "No coverage snapshot yet",
            emptyDescription: "Run a scan",
            resultsTitle: "Checklist results",
            resultsSummary:
              "{{supported}} supported · {{absent}} absent · {{insufficient}} insufficient ({{total}} items)",
            cluePage: "Page {{page}} — open document",
            extractedValue: "Extracted value",
            valueType: {
              percent: "percent",
              money: "money",
              share: "shares",
            },
            crossCheckTitle: "Dual-document cross-check",
            crossCheckDescription: "Compare two docs",
            crossCheckDocA: "Document A",
            crossCheckDocB: "Document B",
            runCrossCheck: "Run cross-check",
            crossChecking: "Comparing…",
            crossCheckSucceeded: "Cross-check completed",
            crossCheckFailed: "Cross-check failed",
            crossCheckDocsRequired: "Select two different documents",
            crossCheckNeedDocs: "Need two docs",
            crossCheckSummary: "{{conflict}} conflict · {{total}} claims",
            claimStatus: {
              aligned: "Aligned",
              conflict: "Conflict",
              absent_in_scope: "Absent in both",
              insufficient: "Insufficient",
            },
            packTitle: "Checklist pack",
            packDescription: "Fork pack",
            packEdit: "Edit checklist",
            packHide: "Hide editor",
            packBuiltin: "Built-in · v{{version}}",
            packForked: "Forked · v{{version}}",
            packItemId: "Item id",
            packLabelEn: "Label (EN)",
            packLabelZh: "Label (ZH)",
            packQueryEn: "Query keywords (EN)",
            packQueryZh: "Query keywords (ZH)",
            packValueType: "Value type",
            packValueTypeNone: "None",
            packAddItem: "Add item",
            packRemoveItem: "Remove",
            packSave: "Save fork",
            packResetAction: "Reset to built-in",
            packSaved: "Checklist fork saved",
            packSaveFailed: "Failed to save checklist",
            packReset: "Checklist reset to built-in",
            packResetFailed: "Failed to reset checklist",
            packLoadFailed: "Failed to load checklist",
            status: {
              supported: "Supported",
              absent_in_scope: "Absent in scope",
              insufficient: "Insufficient",
            },
          },
        },
      },
    },
  });

  return render(
    <I18nextProvider i18n={instance}>
      <MemoryRouter initialEntries={["/acme/deal-rooms/room-1?tab=diligence"]}>
        <Routes>
          <Route
            path="/:workspaceSlug/deal-rooms/:roomId"
            element={<DealRoomDiligenceTab roomId="room-1" />}
          />
        </Routes>
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("DealRoomDiligenceTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getDealRoomLinks.mockResolvedValue({ data: links });
    getDealRoomDocuments.mockResolvedValue({
      data: [
        {
          folder: "/",
          permission: "admin",
          documents: [
            {
              id: "rd-1",
              document_id: "doc-1",
              title: "SPA",
              folder_path: "/",
              sort_order: 0,
              source_type: "upload",
              status: "ready",
              created_at: new Date().toISOString(),
            },
            {
              id: "rd-2",
              document_id: "doc-2",
              title: "Disclosure",
              folder_path: "/",
              sort_order: 1,
              source_type: "upload",
              status: "ready",
              created_at: new Date().toISOString(),
            },
          ],
        },
      ],
    });
    getDealRoomKnowledgeBase.mockResolvedValue({
      room_id: "room-1",
      status: "ready",
      folder_paths: [],
      document_ids: ["doc-1", "doc-2"],
      active_document_ids: ["doc-1", "doc-2"],
      embedded_count: 2,
      folder_count: 0,
    });
    listDDCoveragePacks.mockResolvedValue({
      data: [
        { pack_id: "financing_dd_v1", pack_version: "1", base_pack_id: "financing_dd_v1", forked: false, items: [] },
        { pack_id: "ma_redflag_v1", pack_version: "1", base_pack_id: "ma_redflag_v1", forked: false, items: [] },
      ],
    });
    getDDCoverageSnapshot.mockResolvedValue(snapshot);
    getDDCrossCheckLatest.mockRejectedValue(
      new ApiError({ status: 404, code: "not_found", message: "not found", requestId: "r0" }),
    );
    getDDCoveragePack.mockResolvedValue({
      pack_id: "financing_dd_v1",
      pack_version: "1",
      base_pack_id: "financing_dd_v1",
      forked: false,
      items: [
        {
          id: "cap_table",
          label_en: "Cap table",
          label_zh: "股权结构表",
          query_en: "cap table",
          query_zh: "股权结构表",
        },
      ],
    });
  });

  it("shows scope hint, stale banner, and coverage rows", async () => {
    await renderTab();
    expect(await screen.findByTestId("deal-room-diligence-tab")).toBeInTheDocument();
    expect(screen.getByTestId("diligence-scope-hint")).toHaveTextContent("Scope: room");
    expect(screen.getByTestId("diligence-stale-banner")).toBeInTheDocument();
    expect(screen.getByTestId("diligence-row-cap_table")).toHaveTextContent("Supported");
    expect(screen.getByTestId("diligence-row-option_pool")).toHaveTextContent("Supported");
    expect(screen.getByTestId("diligence-extracted-option_pool")).toHaveTextContent("15%");
    expect(screen.getByTestId("diligence-pack-editor")).toBeInTheDocument();
    expect(screen.getByTestId("diligence-cross-check")).toBeInTheDocument();
  });

  it("starts a scan with pack_id and polls until succeeded", async () => {
    getDDCoverageSnapshot
      .mockResolvedValueOnce(snapshot)
      .mockResolvedValueOnce({ ...snapshot, stale: false });
    startDDCoverageScan.mockResolvedValue({
      job_id: "run-2",
      run: {
        id: "run-2",
        pack_id: "financing_dd_v1",
        pack_version: "1",
        scope: "room",
        status: "queued",
        triggered_by: "u1",
        created_at: new Date().toISOString(),
      },
    });
    getDDCoverageRun.mockResolvedValue({
      id: "run-2",
      pack_id: "financing_dd_v1",
      pack_version: "1",
      scope: "room",
      status: "succeeded",
      triggered_by: "u1",
      created_at: new Date().toISOString(),
      finished_at: new Date().toISOString(),
    });

    await renderTab();
    fireEvent.click(await screen.findByTestId("diligence-run-scan"));
    await waitFor(() =>
      expect(startDDCoverageScan).toHaveBeenCalledWith(
        "room-1",
        expect.objectContaining({ pack_id: "financing_dd_v1" }),
      ),
    );
    await waitFor(() => expect(getDDCoverageRun).toHaveBeenCalledWith("room-1", "run-2"));
  });

  it("shows latest cross-check conflict claims", async () => {
    getDDCrossCheckLatest.mockResolvedValue({
      id: "xc-1",
      pack_id: "financing_dd_v1",
      pack_version: "1",
      document_a_id: "doc-1",
      document_b_id: "doc-2",
      triggered_by: "u1",
      claims: [
        {
          item_id: "cap_table",
          label: "Cap table",
          status: "conflict",
          clues_a: [
            {
              chunk_id: "xa",
              document_id: "doc-1",
              page_number: 1,
              quote: "A quote",
              score: 0.9,
              boxes: [],
            },
          ],
          clues_b: [
            {
              chunk_id: "xb",
              document_id: "doc-2",
              page_number: 1,
              quote: "B quote",
              score: 0.8,
              boxes: [],
            },
          ],
        },
      ],
      created_at: new Date().toISOString(),
    });

    await renderTab();
    expect(await screen.findByTestId("diligence-claim-cap_table")).toHaveTextContent("Conflict");
    expect(screen.getByTestId("diligence-cross-check-results")).toHaveTextContent("1 conflict");
  });

  it("does not start cross-check without two documents selected", async () => {
    await renderTab();
    fireEvent.click(await screen.findByTestId("diligence-run-cross-check"));
    await waitFor(() => expect(startDDCrossCheck).not.toHaveBeenCalled());
  });

  it("shows disabled state when coverage flag is off", async () => {
    getDDCoverageSnapshot.mockRejectedValue(
      new ApiError({
        status: 404,
        code: "dd_coverage_disabled",
        message: "disabled",
        requestId: "r1",
      }),
    );
    await renderTab();
    expect(await screen.findByText(/not enabled/i)).toBeInTheDocument();
    expect(screen.queryByTestId("diligence-run-scan")).not.toBeInTheDocument();
  });
});
