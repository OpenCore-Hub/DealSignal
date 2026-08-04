import type { ReactNode } from "react";
import ReactMarkdown from "react-markdown";
import remarkBreaks from "remark-breaks";
import rehypeSanitize, { defaultSchema } from "rehype-sanitize";
import {
  mapChildrenWithCiteTokens,
  prepareAnswerMarkdown,
} from "@/lib/knowledge/answerMarkdown";
import { cn } from "@/lib/utils";

interface AnswerMarkdownProps {
  answer: string;
  activeCite?: number | null;
  onCite: (n: number) => void;
  className?: string;
}

/** Limited markdown schema: structure only — no raw HTML, links, or images. */
const answerSanitizeSchema = {
  ...defaultSchema,
  tagNames: ["p", "h2", "h3", "ul", "ol", "li", "strong", "em", "br"],
  attributes: {
    ...defaultSchema.attributes,
  },
};

function withCites(
  className: string,
  onCite: (n: number) => void,
  activeCite: number | null | undefined,
  Tag: "p" | "h2" | "h3" | "li" | "strong" | "em",
) {
  return function MdNode({ children }: { children?: ReactNode }) {
    return (
      <Tag className={className}>
        {mapChildrenWithCiteTokens(children, onCite, activeCite ?? null)}
      </Tag>
    );
  };
}

/**
 * Answer layout from full `answer` text (limited Markdown).
 * Claims stay on the trust path (unresolved / ops) — not used for typography.
 */
export function AnswerMarkdown({
  answer,
  activeCite = null,
  onCite,
  className,
}: AnswerMarkdownProps) {
  const source = prepareAnswerMarkdown(answer);
  if (!source) return null;

  return (
    <div
      className={cn(
        "knowledge-answer-md text-[15px] leading-[1.7] text-foreground/90",
        "[&_p]:mb-3 [&_p:last-child]:mb-0",
        "[&_h2]:mb-2 [&_h2]:mt-4 [&_h2]:text-base [&_h2]:font-semibold [&_h2]:tracking-tight [&_h2:first-child]:mt-0",
        "[&_h3]:mb-1.5 [&_h3]:mt-3 [&_h3]:text-[14px] [&_h3]:font-semibold [&_h3]:tracking-tight [&_h3:first-child]:mt-0",
        "[&_ul]:mb-3 [&_ul]:list-disc [&_ul]:space-y-1.5 [&_ul]:pl-5",
        "[&_ol]:mb-3 [&_ol]:list-decimal [&_ol]:space-y-1.5 [&_ol]:pl-5",
        "[&_li]:leading-[1.65] [&_li>p]:mb-0",
        "[&_strong]:font-semibold [&_strong]:text-foreground",
        "[&_em]:italic",
        className,
      )}
      data-testid="knowledge-answer-markdown"
    >
      <ReactMarkdown
        remarkPlugins={[remarkBreaks]}
        rehypePlugins={[[rehypeSanitize, answerSanitizeSchema]]}
        components={{
          p: withCites("", onCite, activeCite, "p"),
          h2: withCites("", onCite, activeCite, "h2"),
          h3: withCites("", onCite, activeCite, "h3"),
          li: withCites("", onCite, activeCite, "li"),
          strong: withCites("font-semibold text-foreground", onCite, activeCite, "strong"),
          em: withCites("italic", onCite, activeCite, "em"),
          ul: ({ children }) => <ul>{children}</ul>,
          ol: ({ children }) => <ol>{children}</ol>,
          // Strip anything else sanitize might leave.
          a: ({ children }) => <>{children}</>,
        }}
      >
        {source}
      </ReactMarkdown>
    </div>
  );
}
