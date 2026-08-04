// @vitest-environment jsdom
import { describe, expect, it, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { createTestI18n } from "@/i18n/test-utils";
import { EvalGoldReviewPanel } from "./EvalGoldReviewPanel";

const listCandidates = vi.fn();
const reviewCandidate = vi.fn();
const getOps = vi.fn();
const exportSeeds = vi.fn();
const downloadExport = vi.fn();

vi.mock("@/lib/api", () => ({
  api: {
    listDealRoomKnowledgeEvalCandidates: (...args: unknown[]) =>
      listCandidates(...args),
    reviewDealRoomKnowledgeEvalCandidate: (...args: unknown[]) =>
      reviewCandidate(...args),
    getDealRoomKnowledgeOps: (...args: unknown[]) => getOps(...args),
    exportDealRoomKnowledgeEvalCandidates: (...args: unknown[]) =>
      exportSeeds(...args),
  },
}));

vi.mock("@/lib/knowledge/downloadEvalSeeds", () => ({
  downloadEvalSeedExport: (...args: unknown[]) => downloadExport(...args),
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

const dealRooms = {
  "knowledge.evalGoldTitle": "Gold review queue",
  "knowledge.evalGoldHint": "Accept wrong-citation samples.",
  "knowledge.evalGoldLoading": "Loading…",
  "knowledge.evalGoldLoadFailed": "Failed",
  "knowledge.evalGoldEmpty": "Empty",
  "knowledge.evalGoldKindWrongCitation": "Wrong citation",
  "knowledge.evalGoldKindNotAnswering": "Not answering",
  "knowledge.evalGoldNote": "Note: {{note}}",
  "knowledge.evalGoldUnknownSource": "Unknown source",
  "knowledge.evalGoldAccept": "Accept gold",
  "knowledge.evalGoldReject": "Reject",
  "knowledge.evalGoldExport": "Export {{count}} seeds",
  "knowledge.evalGoldExportSuccess": "Exported {{count}} seeds",
  "knowledge.evalGoldExportFailed": "Export failed",
  "knowledge.evalGoldExportEmpty": "No accepted seeds",
  "knowledge.evalGoldAcceptedReady": "{{count}} accepted — ready to export",
};

function mockOps(accepted = 0) {
  getOps.mockResolvedValue({
    scope: "workspace",
    windowHours: 24,
    turnsTotal: 0,
    turnsByStatus: {},
    avgDurationMs: 0,
    answersQuota: { used: 0, limit: 0, windowHours: 24 },
    retentionDays: 90,
    coldArchiveCount: 0,
    evalCandidatesByStatus: { accepted, pending: 0 },
    pendingEvalCandidates: 0,
  });
}

describe("EvalGoldReviewPanel", () => {
  beforeEach(() => {
    listCandidates.mockReset();
    reviewCandidate.mockReset();
    getOps.mockReset();
    exportSeeds.mockReset();
    downloadExport.mockReset();
    mockOps(0);
  });

  it("renders nothing when queue and accepted are empty", async () => {
    listCandidates.mockResolvedValue({ items: [] });
    const i18n = await createTestI18n({ dealRooms });
    const { container } = render(
      <I18nextProvider i18n={i18n}>
        <EvalGoldReviewPanel roomId="room-1" />
      </I18nextProvider>,
    );
    await waitFor(() => {
      expect(listCandidates).toHaveBeenCalled();
    });
    await waitFor(() => {
      expect(container.querySelector('[data-testid="knowledge-eval-gold-review"]')).toBeNull();
    });
  });

  it("lists pending candidates and accepts gold", async () => {
    listCandidates.mockResolvedValue({
      items: [
        {
          id: "c1",
          roomId: "room-1",
          turnId: "t1",
          feedbackKind: "wrong_citation",
          question: "What is the purchase price?",
          answer: "Fifty million [1].",
          note: "Wrong schedule",
          reviewStatus: "pending",
          createdAt: "2026-08-04T00:00:00Z",
          snapshot: {
            hits: [
              {
                chunkId: "h1",
                sourceName: "SPA_Schedule.pdf",
                excerpt: "fifty million",
              },
            ],
          },
        },
      ],
    });
    reviewCandidate.mockResolvedValue({
      id: "c1",
      reviewStatus: "accepted",
      expect: "reject_or_rebind",
    });
    const onReviewed = vi.fn();
    const i18n = await createTestI18n({ dealRooms });
    render(
      <I18nextProvider i18n={i18n}>
        <EvalGoldReviewPanel roomId="room-1" onReviewed={onReviewed} />
      </I18nextProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("knowledge-eval-gold-review")).toBeInTheDocument();
    });
    expect(screen.getByText("What is the purchase price?")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("knowledge-eval-gold-accept-c1"));
    await waitFor(() => {
      expect(reviewCandidate).toHaveBeenCalledWith("room-1", "c1", {
        reviewStatus: "accepted",
      });
    });
    await waitFor(() => {
      expect(onReviewed).toHaveBeenCalled();
    });
  });

  it("exports accepted seeds as JSON download", async () => {
    listCandidates.mockResolvedValue({ items: [] });
    mockOps(2);
    exportSeeds.mockResolvedValue({
      description: "Accepted",
      seeds: [
        { id: "a", kind: "wrong_citation", question: "q1", expect: "reject_or_rebind" },
        { id: "b", kind: "wrong_citation", question: "q2", expect: "reject_or_rebind" },
      ],
    });
    const i18n = await createTestI18n({ dealRooms });
    render(
      <I18nextProvider i18n={i18n}>
        <EvalGoldReviewPanel roomId="room-abcd1234" />
      </I18nextProvider>,
    );
    await waitFor(() => {
      expect(screen.getByTestId("knowledge-eval-gold-export")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByTestId("knowledge-eval-gold-export"));
    await waitFor(() => {
      expect(exportSeeds).toHaveBeenCalledWith("room-abcd1234");
    });
    await waitFor(() => {
      expect(downloadExport).toHaveBeenCalled();
    });
  });
});
