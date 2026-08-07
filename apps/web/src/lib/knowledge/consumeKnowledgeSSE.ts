import { ApiError } from "@/lib/apiClient";
import { createSSEParser } from "@/lib/knowledge/parseSSE";
import type { KnowledgeStreamEvent } from "@/lib/knowledge/streamEvents";
import type {
  DealRoomKnowledgeSessionQueryResult,
} from "@/types";

type StreamDonePayload = DealRoomKnowledgeSessionQueryResult & {
  refused?: boolean;
  resultStatus?: string;
};

export async function consumeKnowledgeSSE(
  response: Response,
  opts: {
    signal?: AbortSignal;
    onEvent: (event: KnowledgeStreamEvent) => void;
    requireDone?: boolean;
  },
): Promise<DealRoomKnowledgeSessionQueryResult | null> {
  if (!response.body) {
    throw new ApiError({
      status: response.status,
      code: "stream_incomplete",
      message: "stream_incomplete",
      requestId: "",
    });
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  const parser = createSSEParser();
  let doneResult: DealRoomKnowledgeSessionQueryResult | null = null;

  const handleFrames = (frames: { event: string; data: string }[]) => {
    for (const frame of frames) {
      let payload: unknown;
      try {
        payload = JSON.parse(frame.data) as unknown;
      } catch {
        continue;
      }
      if (frame.event === "phase") {
        const phasePayload = payload as {
          phase?: string;
          retrieveQuery?: string;
          rewriteApplied?: boolean;
        };
        const phase = phasePayload.phase;
        if (phase === "retrieving" || phase === "generating") {
          opts.onEvent({
            type: "phase",
            phase,
            retrieveQuery: phasePayload.retrieveQuery,
            rewriteApplied: phasePayload.rewriteApplied,
          });
        }
        continue;
      }
      if (frame.event === "sources") {
        const src = payload as {
          results?: DealRoomKnowledgeSessionQueryResult["results"];
          grounded?: boolean;
        };
        opts.onEvent({
          type: "sources",
          results: src.results ?? [],
          grounded: !!src.grounded,
        });
        continue;
      }
      if (frame.event === "token") {
        const text = (payload as { text?: string }).text ?? "";
        if (text) opts.onEvent({ type: "token", text });
        continue;
      }
      if (frame.event === "error") {
        const err = payload as { code?: string; message?: string };
        const code = err.code ?? "stream_error";
        opts.onEvent({ type: "error", message: code });
        throw new ApiError({
          status: 0,
          code,
          message: code,
          requestId: "",
        });
      }
      if (frame.event === "done") {
        const data = payload as StreamDonePayload;
        opts.onEvent({
          type: "done",
          answer: data.answer,
          results: data.results ?? data.turn?.hits ?? [],
          refused: data.refused ?? data.turn?.refused,
          resultStatus: data.resultStatus ?? data.turn?.resultStatus,
          retrieveQuery: data.turn?.retrieveQuery,
          rewriteApplied: data.turn?.rewriteApplied,
          claims: data.turn?.claims,
          unresolved: data.turn?.unresolved,
          conflicts: data.turn?.conflicts,
          multiHop: data.turn?.multiHop,
          refusal: data.turn?.refusal,
          judgment: data.turn?.judgment,
        });
        doneResult = {
          sessionId: data.sessionId,
          turn: data.turn,
          query: data.query,
          mode: data.mode,
          answer: data.answer,
          results: data.results ?? [],
          sessionState: data.sessionState,
        };
      }
    }
  };

  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      handleFrames(parser.push(decoder.decode(value, { stream: true })));
    }
    handleFrames(parser.flush());
  } catch (err) {
    if (opts.signal?.aborted || (err instanceof DOMException && err.name === "AbortError")) {
      throw err;
    }
    if (err instanceof ApiError) {
      throw err;
    }
    if (opts.signal?.aborted) {
      throw new DOMException("Aborted", "AbortError");
    }
    throw new ApiError({
      status: 0,
      code: "stream_incomplete",
      message: "stream_incomplete",
      requestId: "",
    });
  }

  if (opts.requireDone !== false && !doneResult) {
    if (opts.signal?.aborted) {
      throw new DOMException("Aborted", "AbortError");
    }
    throw new ApiError({
      status: 0,
      code: "stream_incomplete",
      message: "stream_incomplete",
      requestId: "",
    });
  }
  return doneResult;
}
