import { useRef, useEffect } from "react";
import { Robot, PaperPlaneRight, Spinner } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useAIStore } from "@/stores/aiStore";
import { useTranslation } from "react-i18next";
import type { Evidence } from "@/types";

function EvidenceCard({ evidence }: { evidence: Evidence }) {
  const { t } = useTranslation("ai");
  const { setHighlight } = useAIStore();
  return (
    <button
      type="button"
      className="mt-2 w-full rounded-md border border-border bg-muted/50 p-2 text-left text-xs transition-colors hover:bg-muted"
      onClick={() => setHighlight(evidence, evidence.page_number)}
    >
      <span className="text-caption text-muted-foreground">
        {t("evidence.page", { pageNumber: evidence.page_number })}
      </span>
      <p className="mt-0.5 line-clamp-2">{evidence.quote}</p>
    </button>
  );
}

interface SidebarAIChatProps {
  documentId?: string;
  publicSessionToken?: string;
}

export function SidebarAIChat({ documentId, publicSessionToken }: SidebarAIChatProps) {
  const { t } = useTranslation("ai");
  const { messages, pending, sendMessage } = useAIStore();
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages]);

  const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const form = e.currentTarget;
    const formData = new FormData(form);
    const content = String(formData.get("message") || "").trim();
    if (!content || pending) return;
    sendMessage(content, { documentId, publicSessionToken });
    form.reset();
  };

  const displayMessages = messages.filter((msg) => msg.id !== "welcome");

  return (
    <div className="flex h-full flex-col bg-card">
      {/* Message list */}
      <div
        ref={scrollRef}
        className="flex-1 space-y-3 overflow-y-auto p-3"
        aria-live="polite"
        aria-busy={pending}
      >
        {displayMessages.length === 0 && (
          <div className="flex flex-col items-center py-10 text-center text-muted-foreground">
            <Robot size={28} className="mb-3 opacity-25" />
            <p className="text-sm font-medium">{t("viewer.emptyTitle")}</p>
            <p className="mt-1 text-xs">{t("viewer.emptyExample")}</p>
          </div>
        )}

        {displayMessages.map((msg) => {
              const content = msg.content.startsWith("ai:")
            ? (() => {
                const key = msg.content;
                // Handle search results with interpolated query
                const raw = msg as unknown as Record<string, unknown>;
                const query = (raw._query as string) ?? "";
                if (key === "ai:search.results" || key === "ai:search.noResults" || key === "ai:search.error") {
                  return t(key, { query });
                }
                return t(key);
              })()
            : msg.content;

          return (
            <div
              key={msg.id}
              className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}
            >
              <div
                className={`max-w-[90%] rounded-lg px-3 py-2 text-sm ${
                  msg.role === "user"
                    ? "bg-primary text-primary-foreground"
                    : "bg-muted text-foreground"
                }`}
              >
                <p className="whitespace-pre-wrap break-words">{content}</p>
                {msg.evidences?.map((ev) => (
                  <EvidenceCard key={ev.chunk_id} evidence={ev} />
                ))}
              </div>
            </div>
          );
        })}

        {pending && (
          <div className="flex justify-start">
            <div className="flex items-center gap-2 rounded-lg bg-muted px-3 py-2 text-sm text-muted-foreground">
              <Spinner size={14} className="animate-spin" />
              {t("viewer.thinking")}
            </div>
          </div>
        )}
      </div>

      {/* Input area */}
      <div className="border-t border-border p-3">
        <form onSubmit={handleSubmit} className="flex gap-2">
          <Input
            name="message"
            placeholder={t("viewer.placeholder")}
            className="h-9 flex-1 text-sm"
            autoComplete="off"
          />
          <Button type="submit" size="icon" className="h-9 w-9 shrink-0" disabled={pending}>
            <PaperPlaneRight size={15} weight="bold" />
          </Button>
        </form>
      </div>
    </div>
  );
}
