/** @vitest-environment jsdom */
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "@/lib/api";
import { useKnowledgeFollowUpChips } from "./useKnowledgeFollowUpChips";
import type { DealRoomKnowledgeQATurn } from "@/types";

type FollowUpResult = Awaited<ReturnType<typeof api.suggestDealRoomKnowledgeFollowUps>>;

vi.mock("@/lib/api", () => ({
  api: {
    suggestDealRoomKnowledgeFollowUps: vi.fn(),
    recordDealRoomKnowledgeDeskEvent: vi.fn().mockResolvedValue(undefined),
  },
}));

const t = (key: string, params?: Record<string, string>) =>
  params?.sourceName
    ? `${key}:${params.sourceName}`
    : key;

function turn(partial: Partial<DealRoomKnowledgeQATurn> & { id: string }): DealRoomKnowledgeQATurn {
  return {
    sessionId: "s1",
    sequence: 1,
    question: "q",
    refused: false,
    resultStatus: "answered",
    hits: [{ chunkId: "c", text: "liability cap", score: 0.9, sourceName: "NDA.pdf" }],
    createdAt: "2026-08-03T00:00:00Z",
    ...partial,
  };
}

describe("useKnowledgeFollowUpChips", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("first-paints two narrow chips for mixed GMV answers", () => {
    vi.mocked(api.suggestDealRoomKnowledgeFollowUps).mockReturnValue(
      new Promise(() => {}),
    );
    const { result } = renderHook(() =>
      useKnowledgeFollowUpChips(
        "room-1",
        turn({
          id: "t-gmv",
          question: "年增长GMV多少？",
          answer:
            "材料可见 Managed Ad Spend 约 4.8 亿元，但未给出 GMV 年增长。",
          hits: [
            {
              chunkId: "h1",
              text: "4.8 亿",
              score: 0.9,
              sourceName: "口径.pdf",
            },
          ],
          claims: [
            {
              text: "Managed Ad Spend 约 4.8 亿元",
              hitIds: ["h1"],
              confidence: "grounded",
            },
          ],
        }),
        t,
      ),
    );
    expect(result.current.chips?.map((c) => c.id)).toEqual([
      "narrow-scope",
      "name-clause",
    ]);
    expect(result.current.source).toBe("template");
    expect(result.current.chips?.some((c) => /4\.8|Ad Spend/i.test(c.text))).toBe(
      false,
    );
  });

  it("rejects mission checklist chips and keeps local split", async () => {
    vi.mocked(api.suggestDealRoomKnowledgeFollowUps).mockResolvedValue({
      source: "mission",
      items: [{ id: "mission-option_pool", text: "How is the option pool sized?" }],
    });

    const { result } = renderHook(() =>
      useKnowledgeFollowUpChips("room-1", turn({ id: "t1" }), t),
    );

    await waitFor(() => {
      expect(result.current.upgrading).toBe(false);
    });
    expect(result.current.source).not.toBe("mission");
    expect(result.current.chips?.some((c) => c.id === "mission-option_pool")).toBe(
      false,
    );
    expect(result.current.chips?.length).toBeGreaterThan(0);
  });

  it("upgrades to gap split chips", async () => {
    vi.mocked(api.suggestDealRoomKnowledgeFollowUps).mockResolvedValue({
      source: "gap",
      items: [
        {
          id: "gap-verify-claim",
          text: "How does NDA.pdf support the liability cap?",
          kind: "verify",
          slot: 0,
        },
      ],
    });

    const { result } = renderHook(() =>
      useKnowledgeFollowUpChips("room-1", turn({ id: "t1" }), t),
    );

    await waitFor(() => {
      expect(result.current.upgrading).toBe(false);
      expect(result.current.source).toBe("gap");
      expect(result.current.chips?.[0]?.id).toBe("gap-verify-claim");
    });
  });

  it("shows local split upgrading then upgrades to llm items", async () => {
    let resolve!: (v: {
      source: string;
      items: { id: string; text: string }[];
    }) => void;
    vi.mocked(api.suggestDealRoomKnowledgeFollowUps).mockReturnValue(
      new Promise((r) => {
        resolve = r;
      }),
    );

    const { result } = renderHook(() =>
      useKnowledgeFollowUpChips("room-1", turn({ id: "t1" }), t),
    );

    expect(result.current.upgrading).toBe(true);
    expect(result.current.chips?.length).toBeGreaterThan(0);
    expect(result.current.source).toBe("gap");

    await act(async () => {
      resolve({
        source: "llm",
        items: [{ id: "llm-1", text: "Ask about liability in NDA.pdf?" }],
      });
    });

    await waitFor(() => {
      expect(result.current.upgrading).toBe(false);
      expect(result.current.source).toBe("llm");
      expect(result.current.chips?.[0]?.text).toContain("NDA.pdf");
    });
  });

  it("ignores stale responses after turn changes", async () => {
    const resolvers = new Map<string, (v: FollowUpResult) => void>();
    vi.mocked(api.suggestDealRoomKnowledgeFollowUps).mockImplementation(
      (_roomId, turnId) =>
        new Promise<FollowUpResult>((resolve) => {
          resolvers.set(turnId, resolve);
        }),
    );

    const { result, rerender } = renderHook(
      ({ turnId }) => useKnowledgeFollowUpChips("room-1", turn({ id: turnId }), t),
      { initialProps: { turnId: "t-a" } },
    );

    rerender({ turnId: "t-b" });

    await act(async () => {
      resolvers.get("t-a")?.({
        source: "llm",
        items: [{ id: "stale", text: "STALE FROM A" }],
      });
      resolvers.get("t-b")?.({
        source: "llm",
        items: [{ id: "fresh", text: "FRESH FROM B" }],
      });
    });

    await waitFor(() => {
      expect(result.current.chips?.some((c) => c.text.includes("STALE"))).toBe(
        false,
      );
      expect(result.current.chips?.[0]?.text).toBe("FRESH FROM B");
    });
  });

  it("defers upgrade while engaged and applies on leave", async () => {
    let resolve!: (v: {
      source: string;
      items: { id: string; text: string }[];
    }) => void;
    vi.mocked(api.suggestDealRoomKnowledgeFollowUps).mockReturnValue(
      new Promise((r) => {
        resolve = r;
      }),
    );

    const { result } = renderHook(() =>
      useKnowledgeFollowUpChips("room-1", turn({ id: "t1" }), t),
    );

    const templateText = result.current.chips?.[0]?.text;
    expect(templateText).toBeTruthy();

    act(() => {
      result.current.setEngaged(true);
    });

    await act(async () => {
      resolve({
        source: "llm",
        items: [{ id: "llm-1", text: "UPGRADED CHIP" }],
      });
    });

    await waitFor(() => {
      expect(result.current.upgrading).toBe(false);
    });
    expect(result.current.chips?.[0]?.text).toBe(templateText);

    act(() => {
      result.current.setEngaged(false);
    });

    expect(result.current.chips?.[0]?.text).toBe("UPGRADED CHIP");
    expect(result.current.source).toBe("llm");
  });

  it("keeps local split on empty/error soft-fail", async () => {
    vi.mocked(api.suggestDealRoomKnowledgeFollowUps).mockResolvedValue({
      source: "template",
      items: [],
    });

    const { result } = renderHook(() =>
      useKnowledgeFollowUpChips("room-1", turn({ id: "t1" }), t),
    );

    await waitFor(() => {
      expect(result.current.upgrading).toBe(false);
    });
    expect(result.current.chips?.length).toBeGreaterThan(0);
    expect(result.current.source).toBe("gap");
  });

  it("records followups_upgrade_failed on network soft-fail", async () => {
    vi.mocked(api.suggestDealRoomKnowledgeFollowUps).mockRejectedValue(
      new Error("network"),
    );

    const { result } = renderHook(() =>
      useKnowledgeFollowUpChips("room-1", turn({ id: "t1" }), t),
    );

    await waitFor(() => {
      expect(result.current.upgrading).toBe(false);
    });
    expect(result.current.chips?.length).toBeGreaterThan(0);
    expect(api.recordDealRoomKnowledgeDeskEvent).toHaveBeenCalledWith("room-1", {
      type: "followups_upgrade_failed",
    });
  });

  it("ignores server template strings so FE i18n chips stay", async () => {
    vi.mocked(api.suggestDealRoomKnowledgeFollowUps).mockResolvedValue({
      source: "template",
      items: [{ id: "server", text: "HARDCODED SERVER STRING" }],
    });

    const { result } = renderHook(() =>
      useKnowledgeFollowUpChips("room-1", turn({ id: "t1" }), t),
    );

    await waitFor(() => {
      expect(result.current.upgrading).toBe(false);
    });
    expect(result.current.chips?.some((c) => c.text.includes("HARDCODED"))).toBe(
      false,
    );
    expect(result.current.source).toBe("gap");
  });
});
