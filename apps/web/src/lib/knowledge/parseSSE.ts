/**
 * Minimal SSE parser for Knowledge Tab streams.
 * Expects `event:` + `data:` frames separated by blank lines.
 */

export interface SSEFrame {
  event: string;
  data: string;
}

/** Feed incremental text chunks; yields complete frames. */
export function createSSEParser(): {
  push: (chunk: string) => SSEFrame[];
  flush: () => SSEFrame[];
} {
  let buffer = "";

  const takeFrames = (flushRemainder: boolean): SSEFrame[] => {
    const frames: SSEFrame[] = [];
    for (;;) {
      const sep = buffer.indexOf("\n\n");
      if (sep < 0) break;
      const raw = buffer.slice(0, sep);
      buffer = buffer.slice(sep + 2);
      const frame = parseFrame(raw);
      if (frame) frames.push(frame);
    }
    if (flushRemainder && buffer.trim()) {
      const frame = parseFrame(buffer);
      buffer = "";
      if (frame) frames.push(frame);
    }
    return frames;
  };

  return {
    push: (chunk: string) => {
      buffer += chunk;
      return takeFrames(false);
    },
    flush: () => takeFrames(true),
  };
}

function parseFrame(raw: string): SSEFrame | null {
  let event = "message";
  const dataLines: string[] = [];
  for (const line of raw.split("\n")) {
    if (line.startsWith("event:")) {
      event = line.slice("event:".length).trim();
    } else if (line.startsWith("data:")) {
      dataLines.push(line.slice("data:".length).trimStart());
    }
  }
  if (dataLines.length === 0) return null;
  return { event, data: dataLines.join("\n") };
}
