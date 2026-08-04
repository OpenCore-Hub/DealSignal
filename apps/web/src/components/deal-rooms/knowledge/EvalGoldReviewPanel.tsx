import { useCallback, useEffect, useState } from "react";
import {
  CircleNotch,
  DownloadSimple,
  SealCheck,
  XCircle,
} from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { downloadEvalSeedExport } from "@/lib/knowledge/downloadEvalSeeds";
import type { DealRoomKnowledgeEvalCandidate } from "@/types";
import { cn } from "@/lib/utils";

interface EvalGoldReviewPanelProps {
  roomId: string;
  /** Bump after feedback / ops refresh so the queue reloads. */
  refreshKey?: number | string;
  onReviewed?: () => void;
  className?: string;
}

/**
 * Human review queue + accepted-gold export (ceiling Phase Q/R / L4).
 * Visible when pending reviews or accepted seeds exist.
 */
export function EvalGoldReviewPanel({
  roomId,
  refreshKey,
  onReviewed,
  className,
}: EvalGoldReviewPanelProps) {
  const { t } = useTranslation("dealRooms");
  const [items, setItems] = useState<DealRoomKnowledgeEvalCandidate[]>([]);
  const [acceptedCount, setAcceptedCount] = useState(0);
  const [loading, setLoading] = useState(true);
  const [exporting, setExporting] = useState(false);
  const [actingId, setActingId] = useState<string | null>(null);
  const [error, setError] = useState(false);
  const [hydrated, setHydrated] = useState(false);

  const load = useCallback(() => {
    let cancelled = false;
    setLoading(true);
    setError(false);
    void Promise.all([
      api.listDealRoomKnowledgeEvalCandidates(roomId, {
        status: "pending",
        limit: 20,
      }),
      api.getDealRoomKnowledgeOps(roomId, { windowHours: 24 * 90 }),
    ])
      .then(([pending, ops]) => {
        if (cancelled) return;
        setItems(pending.items ?? []);
        setAcceptedCount(ops.evalCandidatesByStatus?.accepted ?? 0);
      })
      .catch(() => {
        if (!cancelled) {
          setItems([]);
          setAcceptedCount(0);
          setError(true);
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
          setHydrated(true);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [roomId]);

  useEffect(() => load(), [load, refreshKey]);

  const onReview = async (
    id: string,
    reviewStatus: "accepted" | "rejected",
  ) => {
    if (actingId) return;
    setActingId(id);
    try {
      await api.reviewDealRoomKnowledgeEvalCandidate(roomId, id, {
        reviewStatus,
      });
      setItems((prev) => prev.filter((c) => c.id !== id));
      if (reviewStatus === "accepted") {
        setAcceptedCount((n) => n + 1);
      }
      onReviewed?.();
    } catch {
      setError(true);
    } finally {
      setActingId(null);
    }
  };

  const onExport = async () => {
    if (exporting || acceptedCount < 1) return;
    setExporting(true);
    try {
      const pack = await api.exportDealRoomKnowledgeEvalCandidates(roomId);
      if ((pack.seeds?.length ?? 0) < 1) {
        toast.error(t("knowledge.evalGoldExportEmpty"));
        return;
      }
      downloadEvalSeedExport(pack, `knowledge-eval-seeds-${roomId.slice(0, 8)}.json`);
      toast.success(
        t("knowledge.evalGoldExportSuccess", { count: pack.seeds.length }),
      );
    } catch {
      toast.error(t("knowledge.evalGoldExportFailed"));
    } finally {
      setExporting(false);
    }
  };

  if (!hydrated && loading) {
    return (
      <aside
        className={cn(
          "rounded-xl border border-border/60 bg-muted/[0.15] px-3.5 py-3 text-[11px] text-muted-foreground",
          className,
        )}
        data-testid="knowledge-eval-gold-review"
      >
        <span className="inline-flex items-center gap-1.5">
          <CircleNotch size={12} className="animate-spin" />
          {t("knowledge.evalGoldLoading")}
        </span>
      </aside>
    );
  }

  if (!error && items.length === 0 && acceptedCount < 1) return null;

  return (
    <aside
      className={cn(
        "rounded-xl border border-border/60 bg-muted/[0.15] px-3.5 py-3",
        className,
      )}
      data-testid="knowledge-eval-gold-review"
      aria-label={t("knowledge.evalGoldTitle")}
    >
      <div className="mb-2 flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
            {t("knowledge.evalGoldTitle")}
          </p>
          <p className="mt-0.5 text-[11px] leading-relaxed text-muted-foreground">
            {t("knowledge.evalGoldHint")}
          </p>
        </div>
        {acceptedCount > 0 ? (
          <Button
            type="button"
            size="sm"
            variant="outline"
            className="h-7 gap-1 px-2 text-[11px]"
            disabled={exporting}
            onClick={() => {
              void onExport();
            }}
            data-testid="knowledge-eval-gold-export"
          >
            {exporting ? (
              <CircleNotch size={13} className="animate-spin" />
            ) : (
              <DownloadSimple size={13} weight="bold" />
            )}
            {t("knowledge.evalGoldExport", { count: acceptedCount })}
          </Button>
        ) : null}
      </div>

      {error && items.length === 0 && acceptedCount < 1 ? (
        <p className="text-[11px] text-muted-foreground" data-testid="knowledge-eval-gold-error">
          {t("knowledge.evalGoldLoadFailed")}
        </p>
      ) : null}

      {items.length === 0 && acceptedCount > 0 ? (
        <p
          className="text-[11px] text-muted-foreground"
          data-testid="knowledge-eval-gold-accepted-only"
        >
          {t("knowledge.evalGoldAcceptedReady", { count: acceptedCount })}
        </p>
      ) : null}

      {items.length > 0 ? (
        <ul className="space-y-2.5" data-testid="knowledge-eval-gold-list">
          {items.map((c) => (
            <li
              key={c.id}
              className="rounded-lg border border-border/50 bg-background/80 px-2.5 py-2"
              data-testid={`knowledge-eval-gold-item-${c.id}`}
            >
              <div className="mb-1 text-[10px] uppercase tracking-wide text-muted-foreground">
                <span data-testid="knowledge-eval-gold-kind">
                  {c.feedbackKind === "wrong_citation"
                    ? t("knowledge.evalGoldKindWrongCitation")
                    : t("knowledge.evalGoldKindNotAnswering")}
                </span>
              </div>
              <p className="text-[12px] font-medium leading-snug text-foreground/90">
                {c.question}
              </p>
              {c.answer ? (
                <p className="mt-1 line-clamp-3 text-[11px] leading-snug text-muted-foreground">
                  {c.answer}
                </p>
              ) : null}
              {c.note ? (
                <p className="mt-1 text-[11px] leading-snug text-foreground/70">
                  {t("knowledge.evalGoldNote", { note: c.note })}
                </p>
              ) : null}
              {(c.snapshot?.hits?.length ?? 0) > 0 ? (
                <ul className="mt-1.5 space-y-1" data-testid="knowledge-eval-gold-hits">
                  {c.snapshot!.hits!.slice(0, 3).map((h, i) => (
                    <li
                      key={`${h.chunkId || h.sourceName || i}`}
                      className="rounded-md border border-border/40 bg-muted/20 px-2 py-1 text-[10px] leading-snug text-muted-foreground"
                    >
                      <span className="font-medium text-foreground/70">
                        {h.sourceName || t("knowledge.evalGoldUnknownSource")}
                      </span>
                      {h.excerpt ? (
                        <span className="mt-0.5 block line-clamp-2">{h.excerpt}</span>
                      ) : null}
                    </li>
                  ))}
                </ul>
              ) : null}
              <div className="mt-2 flex flex-wrap gap-1.5">
                <Button
                  type="button"
                  size="sm"
                  variant="default"
                  className="h-7 gap-1 px-2 text-[11px]"
                  disabled={actingId === c.id}
                  onClick={() => {
                    void onReview(c.id, "accepted");
                  }}
                  data-testid={`knowledge-eval-gold-accept-${c.id}`}
                >
                  <SealCheck size={13} weight="bold" />
                  {t("knowledge.evalGoldAccept")}
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  className="h-7 gap-1 px-2 text-[11px]"
                  disabled={actingId === c.id}
                  onClick={() => {
                    void onReview(c.id, "rejected");
                  }}
                  data-testid={`knowledge-eval-gold-reject-${c.id}`}
                >
                  <XCircle size={13} weight="bold" />
                  {t("knowledge.evalGoldReject")}
                </Button>
              </div>
            </li>
          ))}
        </ul>
      ) : null}
    </aside>
  );
}
