import { ArrowDown, ArrowUp, PushPin, Robot, Scales, User, UsersThree } from "@phosphor-icons/react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { AnswerMarkdown } from "@/components/deal-rooms/knowledge/AnswerMarkdown";
import { formatHitLocusLabel } from "@/lib/knowledge/citations";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import {
  ownerAskTurnCanPinFAQ,
  ownerAskTurnCanUnpinFAQ,
  ownerAskTurnHasAIPreview,
  ownerAskTurnIsFormalDegraded,
  ownerAskTurnNeedsFormalPublish,
  ownerAskTurnNeedsHostReply,
  ownerAskTurnStatusBadgeVariant,
} from "@/lib/ownerAskInbox";
import { answerOwnerAskQuestion, ownerAskTurnToVisitorQuestion } from "@/lib/ownerAskTurn";
import { formatRelativeTime } from "@/lib/formatters";
import { useWorkspaceAccess } from "@/hooks/useWorkspaceAccess";
import { cn } from "@/lib/utils";
import type { DealRoomKnowledgeQueryHit, OwnerAskTurn } from "@/types";

export interface OwnerAskInboxCardProps {
  turn: OwnerAskTurn;
  linkLabel?: string;
  i18nNs: "dealRooms" | "linkShare";
  answerDraft: string;
  onAnswerDraftChange: (value: string) => void;
  onAnswered: (turn: OwnerAskTurn) => void;
  onFormalPublished?: (turn: OwnerAskTurn) => void;
  onPinned?: (turn: OwnerAskTurn) => void;
  onUnpinned?: (turn: OwnerAskTurn) => void;
  repeatCount?: number;
  suggestPinFAQ?: boolean;
  faqReorder?: {
    canMoveUp: boolean;
    canMoveDown: boolean;
    moving: boolean;
    onMoveUp: () => void;
    onMoveDown: () => void;
  };
  onOpenCitation?: (hit: DealRoomKnowledgeQueryHit) => void;
}

function statusLabelKey(turn: OwnerAskTurn, prefix: string): string {
  if (turn.lane === "ai" || turn.lane === "hybrid") {
    if (turn.lane === "hybrid" && turn.status === "host_escalated") {
      return `${prefix}.turnStatus.host_escalated`;
    }
    if (turn.lane === "hybrid" && turn.status === "host_pending") {
      return `${prefix}.turnStatus.hybrid_pending`;
    }
    return `${prefix}.turnStatus.${turn.status}`;
  }
  return turn.status === "host_answered"
    ? `${prefix}.questionStatus.answered`
    : `${prefix}.questionStatus.pending`;
}

export function OwnerAskInboxCard({
  turn,
  linkLabel,
  i18nNs,
  answerDraft,
  onAnswerDraftChange,
  onAnswered,
  onFormalPublished,
  onPinned,
  onUnpinned,
  repeatCount,
  suggestPinFAQ,
  faqReorder,
  onOpenCitation,
}: OwnerAskInboxCardProps) {
  const { t } = useTranslation(i18nNs);
  const { canWrite } = useWorkspaceAccess();
  const [submitting, setSubmitting] = useState(false);
  const [formalSubmitting, setFormalSubmitting] = useState(false);
  const [formalAnswer, setFormalAnswer] = useState("");
  const [formalSchedule, setFormalSchedule] = useState("");
  const [formalAnonymize, setFormalAnonymize] = useState(true);
  const [pinning, setPinning] = useState(false);
  const [unpinning, setUnpinning] = useState(false);
  const [activeCite, setActiveCite] = useState<number | null>(null);
  const prefix = i18nNs === "linkShare" ? "management" : "qa";
  const locusFmt = {
    sheetPrefix: t(`${prefix}.sheetLabel`),
    pageSingle: (page: number) => t(`${prefix}.pageSingle`, { page }),
    pageRange: (from: number, to: number) => t(`${prefix}.pageRange`, { from, to }),
    pageListSep: t(`${prefix}.pageListSep`),
    pageList: (pages: string) => t(`${prefix}.pageList`, { pages }),
  };

  const aiPayload = turn.ai_payload;
  const showHostReply = canWrite && ownerAskTurnNeedsHostReply(turn);
  const showFormalPublish = canWrite && ownerAskTurnNeedsFormalPublish(turn);
  const showAIPreview = ownerAskTurnHasAIPreview(turn);
  const canPinFAQ = canWrite && ownerAskTurnCanPinFAQ(turn);
  const canUnpinFAQ = canWrite && ownerAskTurnCanUnpinFAQ(turn);
  const showAiRefusedHint =
    (turn.lane === "ai" && turn.status === "ai_refused") ||
    (turn.lane === "hybrid" &&
      (turn.status === "host_pending" || turn.status === "host_escalated") &&
      aiPayload?.refused);

  useEffect(() => {
    if (!showFormalPublish) return;
    setFormalAnswer(turn.host_answer ?? "");
    setFormalAnonymize(turn.formal_anonymize !== false);
    if (turn.formal_publish_at && turn.formal_status === "scheduled") {
      const local = new Date(turn.formal_publish_at);
      const pad = (n: number) => String(n).padStart(2, "0");
      setFormalSchedule(
        `${local.getFullYear()}-${pad(local.getMonth() + 1)}-${pad(local.getDate())}T${pad(local.getHours())}:${pad(local.getMinutes())}`,
      );
    } else {
      setFormalSchedule("");
    }
  }, [showFormalPublish, turn.host_answer, turn.formal_anonymize, turn.formal_publish_at, turn.formal_status]);

  const handleFormalPublish = async () => {
    const text = formalAnswer.trim();
    if (!text) return;
    try {
      setFormalSubmitting(true);
      let publishAt: string | undefined;
      if (formalSchedule.trim()) {
        publishAt = new Date(formalSchedule).toISOString();
      }
      const res = await api.publishFormalAskTurn(turn.link_id, turn.id, {
        answer: text,
        publishAt,
        anonymize: formalAnonymize,
      });
      onFormalPublished?.(res.data);
      toast.success(
        res.data.formal_status === "scheduled"
          ? t(`${prefix}.formalScheduleSuccess`)
          : t(`${prefix}.formalPublishSuccess`),
      );
    } catch (err) {
      if (err instanceof ApiError && err.code === "formal_not_entitled") {
        toast.error(t(`${prefix}.formalNotEntitled`));
      } else {
        toast.error(t(`${prefix}.formalPublishFailed`));
      }
    } finally {
      setFormalSubmitting(false);
    }
  };

  const handleAnswer = async () => {
    const text = answerDraft.trim();
    if (!text) return;
    try {
      setSubmitting(true);
      const updated = await answerOwnerAskQuestion(
        ownerAskTurnToVisitorQuestion(turn),
        text,
      );
      onAnswered({
        ...turn,
        status: "host_answered",
        host_answer: updated.answer,
        updated_at: updated.updated_at,
      });
    } catch {
      toast.error(t(`${prefix}.answerFailed`));
    } finally {
      setSubmitting(false);
    }
  };

  const handlePinFAQ = async () => {
    try {
      setPinning(true);
      const res = await api.pinAskTurnFAQ(turn.link_id, turn.id);
      onPinned?.(res.data);
      toast.success(t(`${prefix}.pinFaqSuccess`));
    } catch {
      toast.error(t(`${prefix}.pinFaqFailed`));
    } finally {
      setPinning(false);
    }
  };

  const handleUnpinFAQ = async () => {
    try {
      setUnpinning(true);
      const res = await api.unpinAskTurnFAQ(turn.link_id, turn.id);
      onUnpinned?.(res.data);
      toast.success(t(`${prefix}.unpinFaqSuccess`));
    } catch {
      toast.error(t(`${prefix}.unpinFaqFailed`));
    } finally {
      setUnpinning(false);
    }
  };

  const openHit = (hit: DealRoomKnowledgeQueryHit) => {
    if (!onOpenCitation) return;
    onOpenCitation(hit);
  };

  const handleCite = (n: number) => {
    setActiveCite(n);
    const hit = aiPayload?.hits?.[n - 1];
    if (hit) openHit(hit);
  };

  const laneLabel =
    turn.lane === "ai"
      ? t(`${prefix}.sourceAI`)
      : turn.lane === "hybrid"
        ? t(`${prefix}.sourceHybrid`)
        : t(`${prefix}.sourceHost`);

  return (
    <li className="rounded-lg border border-border bg-card p-4">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0 flex-1 space-y-1">
          <p className="text-sm font-medium">
            {turn.visitor_email || t(`${prefix}.anonymous`)}
          </p>
          <p className="text-sm text-muted-foreground">{turn.question}</p>
          <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
            <span>{formatRelativeTime(turn.created_at)}</span>
            {linkLabel ? (
              <>
                <span aria-hidden>·</span>
                <span>
                  {t(`${prefix}.linkLabel`)}: {linkLabel}
                </span>
              </>
            ) : null}
            <span
              className={cn(
                "inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-[10px] font-semibold uppercase",
                turn.lane === "ai" && "bg-sky-500/10 text-sky-700 dark:text-sky-300",
                turn.lane === "hybrid" && "bg-violet-500/10 text-violet-700 dark:text-violet-300",
                turn.lane === "host" && "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300",
              )}
            >
              {turn.lane === "ai" ? <Robot size={10} weight="fill" /> : null}
              {turn.lane === "hybrid" ? <UsersThree size={10} weight="fill" /> : null}
              {turn.lane === "host" ? <User size={10} weight="fill" /> : null}
              {laneLabel}
            </span>
            {turn.pinned_faq_at ? (
              <span className="inline-flex items-center gap-1 rounded-full bg-amber-500/10 px-1.5 py-0.5 text-[10px] font-semibold uppercase text-amber-800 dark:text-amber-200">
                <PushPin size={10} weight="fill" />
                {t(`${prefix}.pinnedFaqBadge`)}
              </span>
            ) : null}
            {showFormalPublish ? (
              <span className="inline-flex items-center gap-1 rounded-full bg-sky-500/10 px-1.5 py-0.5 text-[10px] font-semibold uppercase text-sky-800 dark:text-sky-200">
                <Scales size={10} weight="fill" />
                {t(`${prefix}.formalQueueBadge`)}
              </span>
            ) : null}
            {ownerAskTurnIsFormalDegraded(turn) ? (
              <span className="inline-flex items-center gap-1 rounded-full bg-amber-500/10 px-1.5 py-0.5 text-[10px] font-semibold uppercase text-amber-800 dark:text-amber-200">
                <Scales size={10} weight="fill" />
                {t(`${prefix}.formalDegradedBadge`)}
              </span>
            ) : null}
          </div>
        </div>
        <Badge variant={ownerAskTurnStatusBadgeVariant(turn)}>
          {t(statusLabelKey(turn, prefix))}
        </Badge>
      </div>

      {ownerAskTurnIsFormalDegraded(turn) ? (
        <p className="mt-2 text-xs text-amber-800 dark:text-amber-200">
          {t(`${prefix}.formalDegradedHint`)}
        </p>
      ) : null}

      {turn.host_answer ? (
        <div className="mt-3 rounded-md bg-muted p-2 text-sm">
          <span className="font-medium">{t(`${prefix}.answerLabel`)}</span>{" "}
          {turn.host_answer}
        </div>
      ) : null}

      {showAIPreview ? (
        <div className="mt-3 space-y-2 rounded-md border border-border/60 bg-muted/30 p-3">
          <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            {t(`${prefix}.aiAnswerLabel`)}
          </p>
          <AnswerMarkdown
            answer={aiPayload!.answer!}
            activeCite={activeCite ?? undefined}
            onCite={handleCite}
          />
          {aiPayload?.hits && aiPayload.hits.length > 0 ? (
            <div className="space-y-1.5 pt-1">
              <p className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">
                {t(`${prefix}.aiEvidenceTitle`, { count: aiPayload.hits.length })}
              </p>
              {aiPayload.hits.slice(0, 4).map((hit, idx) => {
                const locus = formatHitLocusLabel(hit, locusFmt);
                const citeNum = idx + 1;
                const viewerPage = hit.viewerPage ?? hit.pages?.[0];
                const canJump = Boolean(onOpenCitation && hit.documentId && viewerPage);
                return (
                  <div
                    key={hit.chunkId || `${hit.documentId}-${idx}`}
                    className={cn(
                      "rounded border border-border/40 bg-background/80 px-2 py-1.5 text-xs transition-colors",
                      activeCite === citeNum && "border-foreground/30 shadow-sm",
                    )}
                  >
                    {locus ? <p className="font-medium">{locus}</p> : null}
                    <p className="line-clamp-2 text-muted-foreground">{hit.text}</p>
                    {canJump ? (
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        className="mt-2 h-7 rounded-full text-[11px]"
                        onClick={() => {
                          setActiveCite(citeNum);
                          openHit(hit);
                        }}
                      >
                        {t(`${prefix}.openPage`, { page: viewerPage })}
                      </Button>
                    ) : null}
                  </div>
                );
              })}
            </div>
          ) : null}
        </div>
      ) : null}

      {showAiRefusedHint ? (
        <p className="mt-3 text-sm text-muted-foreground">{t(`${prefix}.aiRefusedHint`)}</p>
      ) : null}

      {showHostReply ? (
        <div className="mt-3 space-y-2">
          <Textarea
            value={answerDraft}
            onChange={(e) => onAnswerDraftChange(e.target.value)}
            placeholder={t(`${prefix}.answerPlaceholder`)}
            rows={2}
          />
          <Button
            size="sm"
            onClick={() => void handleAnswer()}
            disabled={submitting || !answerDraft.trim()}
          >
            {submitting ? t(`${prefix}.saving`) : t(`${prefix}.sendAnswer`)}
          </Button>
        </div>
      ) : null}

      {showFormalPublish ? (
        <div className="mt-3 space-y-3 rounded-md border border-sky-500/20 bg-sky-500/5 p-3">
          <p className="text-xs font-semibold uppercase tracking-wide text-sky-900 dark:text-sky-100">
            {t(`${prefix}.formalPublishTitle`)}
          </p>
          <Textarea
            value={formalAnswer}
            onChange={(e) => setFormalAnswer(e.target.value)}
            placeholder={t(`${prefix}.formalAnswerPlaceholder`)}
            rows={3}
          />
          <div className="space-y-1">
            <Label htmlFor={`formal-schedule-${turn.id}`} className="text-xs">
              {t(`${prefix}.formalScheduleLabel`)}
            </Label>
            <input
              id={`formal-schedule-${turn.id}`}
              type="datetime-local"
              value={formalSchedule}
              onChange={(e) => setFormalSchedule(e.target.value)}
              className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm"
            />
            <p className="text-[11px] text-muted-foreground">{t(`${prefix}.formalScheduleHint`)}</p>
          </div>
          <div className="flex items-center gap-2">
            <Checkbox
              id={`formal-anonymize-${turn.id}`}
              checked={formalAnonymize}
              onCheckedChange={(checked) => setFormalAnonymize(checked === true)}
            />
            <Label htmlFor={`formal-anonymize-${turn.id}`} className="text-sm font-normal">
              {t(`${prefix}.formalAnonymizeLabel`)}
            </Label>
          </div>
          <Button
            size="sm"
            onClick={() => void handleFormalPublish()}
            disabled={formalSubmitting || !formalAnswer.trim()}
          >
            {formalSubmitting
              ? t(`${prefix}.saving`)
              : formalSchedule.trim()
                ? t(`${prefix}.formalScheduleAction`)
                : t(`${prefix}.formalPublishAction`)}
          </Button>
        </div>
      ) : null}

      {canPinFAQ ? (
        <div className="mt-3 space-y-2">
          {suggestPinFAQ && repeatCount ? (
            <p className="text-xs text-amber-800 dark:text-amber-200">
              {t(`${prefix}.repeatPinSuggest`, { count: repeatCount })}
            </p>
          ) : null}
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={pinning}
            onClick={() => void handlePinFAQ()}
          >
            <PushPin size={14} className="mr-1.5" />
            {pinning ? t(`${prefix}.pinningFaq`) : t(`${prefix}.pinFaqAction`)}
          </Button>
        </div>
      ) : null}

      {canUnpinFAQ ? (
        <div className="mt-3 flex flex-wrap items-center gap-2">
          {faqReorder ? (
            <>
              <Button
                type="button"
                variant="outline"
                size="icon"
                className="h-8 w-8"
                disabled={!faqReorder.canMoveUp || faqReorder.moving}
                aria-label={t(`${prefix}.faqMoveUp`)}
                onClick={faqReorder.onMoveUp}
              >
                <ArrowUp size={14} />
              </Button>
              <Button
                type="button"
                variant="outline"
                size="icon"
                className="h-8 w-8"
                disabled={!faqReorder.canMoveDown || faqReorder.moving}
                aria-label={t(`${prefix}.faqMoveDown`)}
                onClick={faqReorder.onMoveDown}
              >
                <ArrowDown size={14} />
              </Button>
            </>
          ) : null}
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={unpinning || faqReorder?.moving}
            onClick={() => void handleUnpinFAQ()}
          >
            <PushPin size={14} className="mr-1.5" weight="fill" />
            {unpinning ? t(`${prefix}.unpinningFaq`) : t(`${prefix}.unpinFaqAction`)}
          </Button>
        </div>
      ) : null}
    </li>
  );
}
