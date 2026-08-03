import { describe, expect, it } from "vitest";
import { createSSEParser } from "./parseSSE";

describe("createSSEParser", () => {
  it("parses complete frames across chunk boundaries", () => {
    const p = createSSEParser();
    expect(p.push('event: phase\ndata: {"phase":"retrieving"}\n')).toEqual([]);
    expect(p.push("\n")).toEqual([
      { event: "phase", data: '{"phase":"retrieving"}' },
    ]);
    const more = p.push(
      'event: done\ndata: {"sessionId":"s1"}\n\nevent: error\ndata: {"message":"x"}\n\n',
    );
    expect(more).toEqual([
      { event: "done", data: '{"sessionId":"s1"}' },
      { event: "error", data: '{"message":"x"}' },
    ]);
  });
});
