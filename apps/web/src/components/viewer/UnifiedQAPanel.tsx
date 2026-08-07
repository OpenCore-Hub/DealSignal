import { useState, useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";
import { ChatCenteredDots, PaperPlaneRight, Spinner } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { useVisitorAskPanel } from "@/hooks/useVisitorAskPanel";
import { useVisitorPinnedFAQs } from "@/hooks/useVisitorPinnedFAQs";
import { useVisitorFormalAsk } from "@/hooks/useVisitorFormalAsk";
import { VisitorAskTurnCard } from "./VisitorAskTurnCard";
import { VisitorPinnedFAQSection } from "./VisitorPinnedFAQSection";
import { VisitorFormalQASection } from "./VisitorFormalQASection";
import type { DealRoomKnowledgeQueryHit } from "@/types";

interface UnifiedQAPanelProps {
  token: string;
  sessionToken?: string;
  qaEnabled?: boolean;
  /** When false, uses legacy POST/GET /questions (pre-unified rollout). */
  unifiedAskEnabled?: boolean;
  onOpenCitation?: (hit: DealRoomKnowledgeQueryHit) => void;
}

export function UnifiedQAPanel({
  token,
  sessionToken,
  qaEnabled,
  unifiedAskEnabled = true,
  onOpenCitation,
}: UnifiedQAPanelProps) {
  const { t } = useTranslation("documents");
  const {
    turns,
    loading,
    error,
    submitting,
    escalatingId,
    stoppedTurnIds,
    submitQuestion,
    escalateToHost,
    resolveKnowledgeTurn,
    stopStream,
  } = useVisitorAskPanel({
    token,
    sessionToken,
    qaEnabled,
    unifiedAskEnabled,
  });
  const { faqs: pinnedFaqs } = useVisitorPinnedFAQs({
    token,
    sessionToken,
    qaEnabled,
  });
  const { entries: formalEntries } = useVisitorFormalAsk({
    token,
    sessionToken,
    qaEnabled,
  });

  const [input, setInput] = useState("");
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [turns, submitting]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const text = input.trim();
    if (!text) return;
    setInput("");
    await submitQuestion(text);
  };

  const busy = submitting;

  return (
    <div className="flex h-full flex-col bg-transparent">
      <div
        ref={scrollRef}
        className="flex-1 space-y-4 overflow-y-auto p-4"
        aria-live="polite"
        aria-busy={busy}
      >
        {formalEntries.length > 0 ? (
          <VisitorFormalQASection
            entries={formalEntries}
            onSuggestQuestion={(question) => setInput(question)}
          />
        ) : null}
        {pinnedFaqs.length > 0 ? (
          <VisitorPinnedFAQSection
            faqs={pinnedFaqs}
            onSuggestQuestion={(question) => setInput(question)}
            onOpenCitation={onOpenCitation}
          />
        ) : null}
        {loading ? (
          <div className="flex items-center justify-center py-8">
            <Spinner size={20} className="animate-spin text-muted-foreground" />
          </div>
        ) : turns.length === 0 ? (
          <div className="flex flex-col items-center rounded-2xl border border-border/60 bg-background/60 px-6 py-10 text-center text-muted-foreground">
            <ChatCenteredDots size={28} className="mb-3 opacity-30" />
            <p className="text-sm font-medium text-foreground">{t("viewer.askEmptyUnified")}</p>
            <p className="mt-1 max-w-[28ch] text-xs leading-relaxed">{t("viewer.askEmptyHint")}</p>
          </div>
        ) : (
          turns.map((turn) => {
            const aiTurn = resolveKnowledgeTurn(turn);
            const streaming =
              aiTurn &&
              aiTurn.phase !== "done" &&
              aiTurn.phase !== "refused" &&
              aiTurn.phase !== "error";
            return (
            <VisitorAskTurnCard
              key={turn.id}
              turn={turn}
              aiTurn={aiTurn}
              escalating={escalatingId === turn.id}
              stopped={stoppedTurnIds.has(turn.id)}
              onOpenCitation={onOpenCitation}
              onStopStream={streaming ? () => stopStream(turn.id) : undefined}
              onEscalate={
                turn.lane === "ai" && turn.status === "ai_refused"
                  ? () => void escalateToHost(turn)
                  : undefined
              }
            />
            );
          })
        )}
      </div>

      {error ? (
        <p className="px-4 pt-2 text-center text-xs text-destructive">{error}</p>
      ) : null}

      <div className="space-y-2 border-t border-border/60 p-3">
        <form onSubmit={handleSubmit} className="flex gap-2">
          <Textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder={t("viewer.askUnifiedPlaceholder")}
            maxLength={500}
            rows={2}
            className="min-h-0 flex-1 resize-none rounded-xl border-border/70 bg-background/80 text-sm"
            disabled={busy}
          />
          <Button
            type="submit"
            size="icon"
            className="h-auto shrink-0 rounded-xl"
            disabled={busy || !input.trim()}
            aria-label={t("viewer.askSubmit")}
          >
            <PaperPlaneRight size={16} />
          </Button>
        </form>
      </div>
    </div>
  );
}
