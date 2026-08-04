// @vitest-environment jsdom
import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { createTestI18n } from "@/i18n/test-utils";
import { KnowledgeOpsStrip } from "./KnowledgeOpsStrip";

const getOps = vi.fn();

vi.mock("@/lib/api", () => ({
  api: {
    getDealRoomKnowledgeOps: (...args: unknown[]) => getOps(...args),
  },
}));

const dealRooms = {
  "knowledge.opsLoading": "Loading ops…",
  "knowledge.opsUnavailable": "Ops unavailable",
  "knowledge.opsTurns": "{{count}} turns / {{hours}}h",
  "knowledge.opsAvgLatency": "avg {{ms}}ms",
  "knowledge.opsP95Latency": "p95 {{ms}}ms",
  "knowledge.opsCostUnits": "{{units}} cost units",
  "knowledge.opsRefusals": "{{count}} refusals/gaps",
  "knowledge.opsPendingEval": "{{count}} gold reviews pending",
  "knowledge.opsAcceptedEval": "{{count}} gold seeds accepted",
  "knowledge.opsQuota": "quota {{used}}/{{limit}}",
  "knowledge.opsColdArchives": "{{count}} cold archives",
  "knowledge.opsFingerprint": "corpus {{fingerprint}}",
};

describe("KnowledgeOpsStrip", () => {
  beforeEach(() => {
    getOps.mockReset();
  });

  it("renders loading then ops fields", async () => {
    getOps.mockResolvedValue({
      scope: "workspace",
      windowHours: 24,
      turnsTotal: 3,
      turnsByStatus: {},
      avgDurationMs: 120,
      p95DurationMs: 400,
      costUnitsTotal: 2,
      refusalsByKind: { no_hits: 1 },
      judgmentsByKind: { grounded: 1 },
      evalCandidatesByStatus: { pending: 1, accepted: 2 },
      pendingEvalCandidates: 1,
      answersQuota: { used: 3, limit: 100, windowHours: 24 },
      coldArchiveCount: 1,
      retentionDays: 90,
      roomCorpusFingerprint: "abcdef0123456789ffff",
    });
    const i18n = await createTestI18n({ dealRooms });
    render(
      <I18nextProvider i18n={i18n}>
        <KnowledgeOpsStrip roomId="room-1" />
      </I18nextProvider>,
    );
    expect(screen.getByTestId("deal-room-knowledge-ops")).toHaveTextContent(
      "Loading ops…",
    );
    await waitFor(() => {
      expect(screen.getByTestId("deal-room-knowledge-ops-p95")).toHaveTextContent(
        "p95 400ms",
      );
    });
    expect(screen.getByTestId("deal-room-knowledge-ops-cost")).toHaveTextContent(
      "2 cost units",
    );
    expect(screen.getByTestId("deal-room-knowledge-ops-eval-accepted")).toHaveTextContent(
      "2 gold seeds accepted",
    );
    expect(screen.getByTestId("deal-room-knowledge-ops-fingerprint")).toHaveTextContent(
      "abcdef01…ffff",
    );
  });

  it("shows unavailable when ops fetch fails", async () => {
    getOps.mockRejectedValue(new Error("boom"));
    const i18n = await createTestI18n({ dealRooms });
    render(
      <I18nextProvider i18n={i18n}>
        <KnowledgeOpsStrip roomId="room-1" />
      </I18nextProvider>,
    );
    await waitFor(() => {
      expect(screen.getByTestId("deal-room-knowledge-ops")).toHaveTextContent(
        "Ops unavailable",
      );
    });
  });
});
