import { useEffect, useState } from "react";
import { CircleNotch } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { asFeedbackKind, FEEDBACK_KINDS } from "@/lib/knowledge/feedback";
import { cn } from "@/lib/utils";
import type {
  DealRoomKnowledgeFeedbackKind,
  DealRoomKnowledgeTurnFeedback,
} from "@/types";

interface TurnFeedbackControlsProps {
  turnId: string;
  feedback?: DealRoomKnowledgeTurnFeedback;
  disabled?: boolean;
  onSubmit: (body: {
    kind: DealRoomKnowledgeFeedbackKind;
    note?: string;
  }) => Promise<void>;
}

/**
 * Phase C: mutually exclusive feedback kinds (audit, not a social like bar).
 */
export function TurnFeedbackControls({
  turnId,
  feedback,
  disabled,
  onSubmit,
}: TurnFeedbackControlsProps) {
  const { t } = useTranslation("dealRooms");
  const [pendingKind, setPendingKind] = useState<DealRoomKnowledgeFeedbackKind | null>(
    null,
  );
  const [note, setNote] = useState(feedback?.note ?? "");
  const [savingNote, setSavingNote] = useState(false);

  useEffect(() => {
    setNote(feedback?.note ?? "");
  }, [feedback?.kind, feedback?.note, turnId]);

  const selected = asFeedbackKind(feedback?.kind);

  const submitKind = async (kind: DealRoomKnowledgeFeedbackKind) => {
    if (disabled || pendingKind) return;
    setPendingKind(kind);
    try {
      await onSubmit({
        kind,
        note: kind === "wrong_citation" ? note.trim() || undefined : undefined,
      });
    } finally {
      setPendingKind(null);
    }
  };

  const saveNote = async () => {
    if (!selected || selected !== "wrong_citation" || disabled) return;
    const next = note.trim();
    if (next === (feedback?.note ?? "").trim()) return;
    setSavingNote(true);
    try {
      await onSubmit({ kind: "wrong_citation", note: next || undefined });
    } finally {
      setSavingNote(false);
    }
  };

  return (
    <div
      className="space-y-2 px-5 py-4"
      data-testid={`knowledge-turn-feedback-${turnId}`}
    >
      <p className="font-mono text-[10px] font-medium uppercase tracking-[0.12em] text-muted-foreground">
        {t("knowledge.feedback.label")}
      </p>
      <div className="flex flex-wrap gap-2">
        {FEEDBACK_KINDS.map((kind) => {
          const active = selected === kind;
          const busy = pendingKind === kind;
          return (
            <button
              key={kind}
              type="button"
              disabled={disabled || !!pendingKind}
              data-testid={`knowledge-feedback-${kind}`}
              aria-pressed={active}
              className={cn(
                "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-[11px] font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50",
                active
                  ? "border-foreground/30 bg-foreground/[0.06] text-foreground"
                  : "border-border/80 bg-background text-foreground/75 hover:bg-muted/50 hover:text-foreground",
              )}
              onClick={() => {
                void submitKind(kind);
              }}
            >
              {busy ? <CircleNotch size={12} className="animate-spin" /> : null}
              {t(`knowledge.feedback.${kind}`)}
            </button>
          );
        })}
      </div>
      {selected === "wrong_citation" ? (
        <div className="flex items-center gap-2">
          <Input
            value={note}
            disabled={disabled || savingNote}
            maxLength={500}
            placeholder={t("knowledge.feedback.notePlaceholder")}
            data-testid="knowledge-feedback-note"
            className="h-8 text-xs"
            onChange={(e) => setNote(e.target.value)}
            onBlur={() => {
              void saveNote();
            }}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                void saveNote();
              }
            }}
          />
          {savingNote ? (
            <CircleNotch size={14} className="shrink-0 animate-spin text-muted-foreground" />
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
