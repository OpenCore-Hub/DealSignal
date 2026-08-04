import type { ReactNode } from "react";
import { Children } from "react";
import { cn } from "@/lib/utils";

/** Private-use tokens so `[n]` is not parsed as a CommonMark link reference. */
export const CITE_TOKEN_OPEN = "\uE000";
export const CITE_TOKEN_CLOSE = "\uE001";

const CITE_TOKEN_RE = new RegExp(
  `${CITE_TOKEN_OPEN}(\\d+)${CITE_TOKEN_CLOSE}`,
  "g",
);

/** Protect `[n]` cite markers before markdown parse. */
export function encodeAnswerCitations(src: string): string {
  return src.replace(/\[(\d+)\]/g, `${CITE_TOKEN_OPEN}$1${CITE_TOKEN_CLOSE}`);
}

/**
 * Light structural normalize so single-newline headings/lists become real blocks.
 * Does not invent content — only inserts blank lines at block boundaries.
 */
export function normalizeAnswerMarkdown(src: string): string {
  const lines = src.replace(/\r\n/g, "\n").split("\n");
  const out: string[] = [];
  for (const line of lines) {
    const trimmed = line.trim();
    const isBlock =
      /^(#{1,3}\s)/.test(trimmed) ||
      /^[-*]\s+/.test(trimmed) ||
      /^\d+\.\s+/.test(trimmed);
    if (isBlock && out.length > 0 && out[out.length - 1] !== "") {
      out.push("");
    }
    out.push(line);
  }
  return out.join("\n").trim();
}

export function prepareAnswerMarkdown(src: string): string {
  return encodeAnswerCitations(normalizeAnswerMarkdown(src));
}

/** Render a text leaf, restoring cite chips from private-use tokens. */
export function renderTextWithCiteTokens(
  text: string,
  onCite: (n: number) => void,
  activeCite: number | null,
): ReactNode {
  if (!text) return null;
  const parts = text.split(CITE_TOKEN_RE);
  const nodes: ReactNode[] = [];
  for (let i = 0; i < parts.length; i++) {
    const part = parts[i];
    if (part === undefined || part === "") continue;
    if (i % 2 === 1) {
      const n = Number(part);
      if (!Number.isFinite(n) || n <= 0) continue;
      nodes.push(
        <button
          key={`cite-${i}-${n}`}
          type="button"
          className={cn(
            "mx-0.5 inline-flex h-5 min-w-5 items-center justify-center rounded-sm bg-foreground/[0.06] px-1 align-baseline font-mono text-[11px] font-semibold text-foreground transition-colors hover:bg-foreground/10",
            activeCite === n && "bg-foreground/10 ring-1 ring-foreground/15",
          )}
          data-testid={`knowledge-cite-${n}`}
          onClick={() => onCite(n)}
        >
          {n}
        </button>,
      );
      continue;
    }
    nodes.push(part);
  }
  if (nodes.length === 0) return null;
  if (nodes.length === 1) return nodes[0];
  return <>{nodes}</>;
}

/** One-level child walk: string leaves → cite chips; elements pass through. */
export function mapChildrenWithCiteTokens(
  children: ReactNode,
  onCite: (n: number) => void,
  activeCite: number | null,
): ReactNode {
  return Children.map(children, (child) => {
    if (typeof child === "string" || typeof child === "number") {
      return renderTextWithCiteTokens(String(child), onCite, activeCite);
    }
    return child;
  });
}
