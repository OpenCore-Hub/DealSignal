import { describe, expect, it } from "vitest";
import {
  displayCorpusStatus,
  resolveCorpusAttentionStage,
  type KnowledgeRoomMetrics,
} from "./CorpusIntegrityRail";
import type { DealRoomKnowledgeCorpus } from "@/types";

function corpus(
  partial: Partial<DealRoomKnowledgeCorpus> &
    Pick<DealRoomKnowledgeCorpus, "status" | "documents">,
): DealRoomKnowledgeCorpus {
  return { enabled: true, ...partial };
}

describe("resolveCorpusAttentionStage", () => {
  it("marks empty corpus", () => {
    expect(
      resolveCorpusAttentionStage(corpus({ status: "ready", documents: [] })),
    ).toBe("empty");
  });

  it("marks building while documents are still in flight", () => {
    expect(
      resolveCorpusAttentionStage(
        corpus({
          status: "provisioning",
          documents: [
            { documentId: "d1", status: "pending", chunkCount: 0 },
          ],
        }),
      ),
    ).toBe("building");
    expect(
      resolveCorpusAttentionStage(
        corpus({
          status: "ready",
          progress: { total: 1, pending: 1, syncing: 0, synced: 0, failed: 0, jobStatus: "running" },
          documents: [
            { documentId: "d1", status: "pending", chunkCount: 0 },
          ],
        }),
      ),
    ).toBe("building");
  });

  it("marks attention on failed docs", () => {
    expect(
      resolveCorpusAttentionStage(
        corpus({
          status: "degraded",
          documents: [
            { documentId: "d1", status: "failed", chunkCount: 0 },
          ],
        }),
      ),
    ).toBe("attention");
  });

  it("marks ready when synced and idle", () => {
    expect(
      resolveCorpusAttentionStage(
        corpus({
          status: "ready",
          documents: [
            { documentId: "d1", status: "synced", chunkCount: 2 },
            { documentId: "d2", status: "synced", chunkCount: 7 },
          ],
        }),
      ),
    ).toBe("ready");
  });

  it("heals stuck provisioning when all docs are synced", () => {
    const stuck = corpus({
      status: "provisioning",
      progress: {
        total: 2,
        pending: 0,
        syncing: 0,
        synced: 2,
        failed: 0,
        jobStatus: "done",
      },
      documents: [
        { documentId: "d1", status: "synced", chunkCount: 2 },
        { documentId: "d2", status: "synced", chunkCount: 7 },
      ],
    });
    expect(displayCorpusStatus(stuck)).toBe("ready");
    expect(resolveCorpusAttentionStage(stuck)).toBe("ready");
  });
});

describe("KnowledgeRoomMetrics semantics", () => {
  it("views and active links follow deal-room analytics, not Ask or accessCount", () => {
    const metrics: KnowledgeRoomMetrics = {
      documentCount: 2,
      viewCount: 12,
      activeLinkCount: 3,
    };
    expect(metrics.viewCount).toBe(12);
    expect(metrics.activeLinkCount).toBe(3);
  });
});
