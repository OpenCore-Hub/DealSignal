// @vitest-environment jsdom
import { describe, expect, it, vi, beforeEach } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { toast } from "sonner";
import { createTestI18n } from "@/i18n/test-utils";
import { ApiError } from "@/lib/apiClient";
import { useKnowledgeDeskSession } from "./useKnowledgeDeskSession";
import { useKnowledgeQueryStore } from "@/stores/knowledgeQueryStore";

const getActive = vi.fn();
const stream = vi.fn();
const upsertFeedback = vi.fn();
const recordEvent = vi.fn();

vi.mock("@/lib/api", () => ({
  api: {
    getActiveDealRoomKnowledgeSession: (...args: unknown[]) => getActive(...args),
    streamDealRoomKnowledgeSession: (...args: unknown[]) => stream(...args),
    upsertDealRoomKnowledgeTurnFeedback: (...args: unknown[]) => upsertFeedback(...args),
    recordDealRoomKnowledgeDeskEvent: (...args: unknown[]) => recordEvent(...args),
  },
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

async function wrapHook() {
  const i18n = await createTestI18n({
    dealRooms: {
      "knowledge.queryFailed": "Query failed",
      "knowledge.feedback.saveFailed": "Feedback failed",
      "knowledge.errors.knowledge_corpus_not_ready": "Corpus not ready",
    },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <I18nextProvider i18n={i18n}>{children}</I18nextProvider>
  );
}

describe("useKnowledgeDeskSession", () => {
  beforeEach(() => {
    getActive.mockReset();
    stream.mockReset();
    upsertFeedback.mockReset();
    recordEvent.mockReset();
    useKnowledgeQueryStore.getState().clear();
    getActive.mockResolvedValue({ session: null, turns: [] });
    recordEvent.mockResolvedValue(undefined);
  });

  it("hydrates an active session and notifies the caller", async () => {
    const onHydrated = vi.fn();
    getActive.mockResolvedValue({
      session: {
        id: "sess-1",
        roomId: "room-1",
        status: "active",
        createdAt: "2026-08-04T00:00:00Z",
        updatedAt: "2026-08-04T00:00:00Z",
        state: { openQuestions: [{ text: "gap?", sourceTurnId: "t1" }] },
      },
      turns: [
        {
          id: "t1",
          sessionId: "sess-1",
          sequence: 1,
          question: "What is the cap?",
          answer: "Ten million.",
          refused: false,
          resultStatus: "answered",
          hits: [{ chunkId: "c1", sourceName: "SPA.pdf" }],
        },
      ],
    });
    const wrapper = await wrapHook();
    const { result } = renderHook(
      () =>
        useKnowledgeDeskSession("room-1", {
          onActiveSessionHydrated: onHydrated,
        }),
      { wrapper },
    );
    await waitFor(() => {
      expect(result.current.sessionHydrated).toBe(true);
    });
    expect(onHydrated).toHaveBeenCalled();
    expect(result.current.activeSessionId).toBe("sess-1");
    expect(result.current.turns).toHaveLength(1);
    expect(result.current.sessionState?.openQuestions?.[0]?.text).toBe("gap?");
  });

  it("blocks ask when allowAsk is false", async () => {
    const wrapper = await wrapHook();
    const { result } = renderHook(
      () => useKnowledgeDeskSession("room-1", { allowAsk: false }),
      { wrapper },
    );
    await waitFor(() => {
      expect(result.current.sessionHydrated).toBe(true);
    });
    await act(async () => {
      result.current.setQuery("What is the price?");
    });
    await act(async () => {
      await result.current.onAsk();
    });
    expect(stream).not.toHaveBeenCalled();
  });

  it("streams an ask and merges the audit turn", async () => {
    stream.mockImplementation(
      async (
        _roomId: string,
        body: { clientRequestId?: string },
        opts: { onEvent?: (e: unknown) => void },
      ) => {
        expect(body.clientRequestId).toBeTruthy();
        opts.onEvent?.({ type: "phase", phase: "retrieving" });
        opts.onEvent?.({
          type: "done",
          turnId: "t-new",
          sessionId: "sess-2",
        });
        return {
          sessionId: "sess-2",
          sessionState: null,
          turn: {
            id: "t-new",
            sessionId: "sess-2",
            sequence: 1,
            question: "What is the price?",
            answer: "Fifty.",
            refused: false,
            resultStatus: "answered",
            hits: [],
          },
        };
      },
    );
    const wrapper = await wrapHook();
    const { result } = renderHook(() => useKnowledgeDeskSession("room-1"), {
      wrapper,
    });
    await waitFor(() => {
      expect(result.current.sessionHydrated).toBe(true);
    });
    await act(async () => {
      await result.current.onAsk("What is the price?");
    });
    expect(stream).toHaveBeenCalled();
    expect(result.current.turns.map((t) => t.id)).toContain("t-new");
    expect(result.current.activeSessionId).toBe("sess-2");
    expect(result.current.asking).toBe(false);
  });

  it("reuses clientRequestId for the same question until it succeeds", async () => {
    stream
      .mockRejectedValueOnce(
        new ApiError({
          status: 429,
          code: "knowledge_query_busy",
          message: "busy",
          requestId: "",
        }),
      )
      .mockResolvedValueOnce({
        sessionId: "sess-2",
        sessionState: null,
        turn: {
          id: "t-new",
          sessionId: "sess-2",
          sequence: 1,
          question: "What is the price?",
          answer: "Fifty.",
          refused: false,
          resultStatus: "answered",
          hits: [],
        },
      });
    const wrapper = await wrapHook();
    const { result } = renderHook(() => useKnowledgeDeskSession("room-1"), {
      wrapper,
    });
    await waitFor(() => {
      expect(result.current.sessionHydrated).toBe(true);
    });
    await act(async () => {
      await result.current.onAsk("What is the price?");
    });
    await act(async () => {
      await result.current.onAsk("What is the price?");
    });
    const firstId = stream.mock.calls[0]?.[1]?.clientRequestId;
    const secondId = stream.mock.calls[1]?.[1]?.clientRequestId;
    expect(firstId).toBeTruthy();
    expect(firstId).toBe(secondId);
  });

  it("preserves composer query when done carries a cancelled error turn", async () => {
    useKnowledgeQueryStore.getState().setDraft("room-1", {
      query: "What is the price?",
      turns: [],
      activeCite: null,
    });
    stream.mockResolvedValueOnce({
      sessionId: "sess-2",
      sessionState: null,
      turn: {
        id: "t-cancel",
        sessionId: "sess-2",
        sequence: 1,
        question: "What is the price?",
        answer: "",
        refused: false,
        resultStatus: "error",
        errorSummary: "client_cancelled",
        hits: [],
      },
    });
    const wrapper = await wrapHook();
    const { result } = renderHook(() => useKnowledgeDeskSession("room-1"), {
      wrapper,
    });
    await waitFor(() => {
      expect(result.current.sessionHydrated).toBe(true);
    });
    await act(async () => {
      await result.current.onAsk("What is the price?");
    });
    expect(result.current.query).toBe("What is the price?");
  });

  it("does not toast client_cancelled when done carries a cancelled error turn", async () => {
    vi.mocked(toast.error).mockClear();
    stream.mockResolvedValueOnce({
      sessionId: "sess-2",
      sessionState: null,
      turn: {
        id: "t-cancel",
        sessionId: "sess-2",
        sequence: 1,
        question: "What is the price?",
        answer: "",
        refused: false,
        resultStatus: "error",
        errorSummary: "client_cancelled",
        hits: [],
      },
    });
    const wrapper = await wrapHook();
    const { result } = renderHook(() => useKnowledgeDeskSession("room-1"), {
      wrapper,
    });
    await waitFor(() => {
      expect(result.current.sessionHydrated).toBe(true);
    });
    await act(async () => {
      await result.current.onAsk("What is the price?");
    });
    expect(vi.mocked(toast.error)).not.toHaveBeenCalled();
    expect(result.current.turns.map((t) => t.id)).toContain("t-cancel");
  });

  it("hydrates silently when busy overlaps a stop-and-reask", async () => {
    vi.mocked(toast.error).mockClear();
    let hydrateCalls = 0;
    getActive.mockImplementation(async () => {
      hydrateCalls++;
      if (hydrateCalls >= 2) {
        return {
          session: { id: "sess-2", roomId: "room-1", status: "active" },
          turns: [
            {
              id: "t-audit",
              sessionId: "sess-2",
              sequence: 1,
              question: "What is the price?",
              answer: "",
              refused: false,
              resultStatus: "error",
              errorSummary: "client_cancelled",
              hits: [],
            },
          ],
        };
      }
      return { session: null, turns: [] };
    });
    stream
      .mockImplementationOnce(
        async (
          _roomId: string,
          _body: unknown,
          opts: { signal?: AbortSignal },
        ) => {
          await new Promise<void>((resolve) => {
            opts.signal?.addEventListener("abort", () => resolve(), { once: true });
          });
          throw new DOMException("Aborted", "AbortError");
        },
      )
      .mockRejectedValueOnce(
        new ApiError({
          status: 429,
          code: "knowledge_query_busy",
          message: "busy",
          requestId: "",
        }),
      );
    const wrapper = await wrapHook();
    const { result } = renderHook(() => useKnowledgeDeskSession("room-1"), {
      wrapper,
    });
    await waitFor(() => {
      expect(result.current.sessionHydrated).toBe(true);
    });
    await act(async () => {
      const first = result.current.onAsk("What is the price?");
      await new Promise((r) => setTimeout(r, 10));
      result.current.onStop();
      await first;
    });
    await act(async () => {
      await result.current.onAsk("What is the price?");
    });
    expect(vi.mocked(toast.error)).not.toHaveBeenCalled();
    expect(result.current.turns.map((t) => t.id)).toContain("t-audit");
  });

  it("hydrates silently when stream_incomplete follows a user stop", async () => {
    vi.mocked(toast.error).mockClear();
    let hydrateCalls = 0;
    getActive.mockImplementation(async () => {
      hydrateCalls++;
      if (hydrateCalls >= 2) {
        return {
          session: { id: "sess-2", roomId: "room-1", status: "active" },
          turns: [
            {
              id: "t-stop",
              sessionId: "sess-2",
              sequence: 1,
              question: "What is the price?",
              answer: "",
              refused: false,
              resultStatus: "error",
              errorSummary: "client_cancelled",
              hits: [],
            },
          ],
        };
      }
      return { session: null, turns: [] };
    });
    stream.mockImplementation(
      async (
        _roomId: string,
        _body: unknown,
        opts: { signal?: AbortSignal },
      ) => {
        await new Promise<void>((resolve) => {
          opts.signal?.addEventListener("abort", () => resolve(), { once: true });
        });
        throw new ApiError({
          status: 0,
          code: "stream_incomplete",
          message: "stream_incomplete",
          requestId: "",
        });
      },
    );
    const wrapper = await wrapHook();
    const { result } = renderHook(() => useKnowledgeDeskSession("room-1"), {
      wrapper,
    });
    await waitFor(() => {
      expect(result.current.sessionHydrated).toBe(true);
    });
    await act(async () => {
      const pending = result.current.onAsk("What is the price?");
      await new Promise((r) => setTimeout(r, 10));
      result.current.onStop();
      await pending;
    });
    expect(vi.mocked(toast.error)).not.toHaveBeenCalled();
    expect(result.current.turns.map((t) => t.id)).toContain("t-stop");
  });

  it("upserts feedback onto the matching turn", async () => {
    useKnowledgeQueryStore.getState().setDraft("room-1", {
      activeSessionId: "sess-1",
      turns: [
        {
          id: "t1",
          sessionId: "sess-1",
          sequence: 1,
          question: "q",
          refused: false,
          resultStatus: "answered",
          hits: [],
          createdAt: "2026-08-01T00:00:00Z",
        },
      ],
      query: "",
      activeCite: null,
    });
    upsertFeedback.mockResolvedValue({ kind: "helpful", note: "ok" });
    const wrapper = await wrapHook();
    const { result } = renderHook(() => useKnowledgeDeskSession("room-1"), {
      wrapper,
    });
    await waitFor(() => {
      expect(result.current.sessionHydrated).toBe(true);
    });
    await act(async () => {
      await result.current.onFeedback("t1", { kind: "helpful" });
    });
    expect(upsertFeedback).toHaveBeenCalledWith("room-1", "t1", {
      kind: "helpful",
    });
    expect(result.current.turns[0]?.feedback?.kind).toBe("helpful");
  });

  it("records cite_open from grounded audit turns", async () => {
    useKnowledgeQueryStore.getState().setDraft("room-1", {
      activeSessionId: "sess-1",
      turns: [
        {
          id: "t1",
          sessionId: "sess-1",
          sequence: 1,
          question: "q",
          refused: false,
          resultStatus: "answered",
          hits: [{ chunkId: "c1", text: "passage", score: 0.9 }],
          createdAt: "2026-08-01T00:00:00Z",
        },
      ],
      query: "",
      activeCite: null,
    });
    const wrapper = await wrapHook();
    const { result } = renderHook(() => useKnowledgeDeskSession("room-1"), {
      wrapper,
    });
    await waitFor(() => {
      expect(result.current.sessionHydrated).toBe(true);
    });
    act(() => {
      result.current.recordCiteOpen();
    });
    expect(recordEvent).toHaveBeenCalledWith("room-1", {
      type: "cite_open",
      turnOutcome: "grounded",
    });
  });
});
