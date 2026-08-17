import { useEffect, useMemo, useState } from "react";
import {
  ArrowsClockwise,
  CaretDown,
  CaretUp,
  FileText,
  WarningCircle,
} from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DealRoomMetricCard,
  MetricStatusDot,
} from "@/components/deal-rooms/DealRoomMetricCard";
import { formatRelativeTime } from "@/lib/formatters";
import type { DealRoomKnowledgeCorpus, KnowledgeQuotaPair } from "@/types";
import { cn } from "@/lib/utils";

const DEFAULT_QUOTA_LIMITS = {
  knowledgeBases: 100,
  documents: 5000,
  answers: 10_000,
};

export type CorpusAttentionStage = "empty" | "building" | "attention" | "ready";

/** Visible rows before the list scrolls (≈10 document rows). */
export const CORPUS_DOC_LIST_VISIBLE = 10;

function isKnowledgeBusy(status?: string, jobStatus?: string) {
  if (status === "syncing" || status === "provisioning") return true;
  return jobStatus === "pending" || jobStatus === "running";
}

/**
 * Heal stuck provisioning/syncing badges when every document row already settled.
 * Mirrors apps/api reconcileCorpusStatus (ingest_doc historically left corpus at provisioning).
 */
export function displayCorpusStatus(corpus: DealRoomKnowledgeCorpus): string {
  const status = corpus.status;
  if (status !== "provisioning" && status !== "syncing") return status;
  const docs = corpus.documents.filter((d) => d.status !== "deleted");
  if (docs.length === 0) return status;
  if (docs.some((d) => d.status === "pending" || d.status === "syncing")) return status;
  if (docs.some((d) => d.status === "failed")) return "degraded";
  if (docs.every((d) => d.status === "synced")) return "ready";
  return status;
}

/** Stage drives prominence: building/attention auto-expand; ready stays collapsed. */
export function resolveCorpusAttentionStage(
  corpus: DealRoomKnowledgeCorpus,
): CorpusAttentionStage {
  if (corpus.documents.length === 0) return "empty";

  const status = displayCorpusStatus(corpus);
  const hasFailed =
    status === "degraded" ||
    status === "failed" ||
    corpus.documents.some((d) => d.status === "failed");
  if (hasFailed) return "attention";

  const docsInFlight = corpus.documents.some(
    (d) => d.status === "pending" || d.status === "syncing",
  );
  const busy =
    docsInFlight ||
    (status !== "ready" && isKnowledgeBusy(status, corpus.progress?.jobStatus));
  if (busy || status === "none" || status === "provisioning") {
    return "building";
  }

  return "ready";
}

export interface KnowledgeRoomMetrics {
  documentCount: number;
  /** Member-excluded link_opened count (align GetDealRoomAnalytics.totalViews). */
  viewCount: number;
  /** Live shares: status=active and not past-due (align GetDealRoomAnalytics.activeLinkCount). */
  activeLinkCount: number;
}

export interface CorpusIntegrityRailProps {
  corpus: DealRoomKnowledgeCorpus;
  metrics: KnowledgeRoomMetrics;
  syncing: boolean;
  onSync?: () => void;
}

/**
 * Corpus integrity metric card.
 * "View details" toggles the document list; >10 docs scroll inside a fixed viewport.
 */
export function CorpusIntegrityRail({
  corpus,
  metrics,
  syncing,
  onSync,
}: CorpusIntegrityRailProps) {
  const { t } = useTranslation("dealRooms");
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const stage = resolveCorpusAttentionStage(corpus);
  const needsAttention = stage !== "ready";
  const [expanded, setExpanded] = useState(needsAttention);
  const [quotaOpen, setQuotaOpen] = useState(false);

  useEffect(() => {
    if (needsAttention) setExpanded(true);
    else setExpanded(false);
  }, [needsAttention, corpus.status, corpus.progress?.jobStatus]);

  const quotaRows = useMemo(() => {
    const q = corpus.quota;
    const pair = (
      used: number,
      limit: number | undefined,
      fallbackLimit: number,
    ): KnowledgeQuotaPair => ({
      used: Math.max(0, used),
      limit: limit && limit > 0 ? limit : fallbackLimit,
    });
    return {
      knowledgeBases: pair(
        q?.knowledgeBases.used ?? 0,
        q?.knowledgeBases.limit,
        DEFAULT_QUOTA_LIMITS.knowledgeBases,
      ),
      documents: pair(
        q?.documents.used ?? metrics.documentCount,
        q?.documents.limit,
        DEFAULT_QUOTA_LIMITS.documents,
      ),
      answers: pair(
        // Never fall back to visitor Ask Host metrics — those are a different product.
        q?.answers.used ?? 0,
        q?.answers.limit,
        DEFAULT_QUOTA_LIMITS.answers,
      ),
    };
  }, [corpus.quota, metrics.documentCount]);

  const status = displayCorpusStatus(corpus);
  const docsInFlight = corpus.documents.some(
    (d) => d.status === "pending" || d.status === "syncing",
  );
  const busy =
    docsInFlight ||
    (status !== "ready" && isKnowledgeBusy(status, corpus.progress?.jobStatus));
  const statusLabel = t(`knowledge.status.${status}`, {
    defaultValue: status,
  });
  const spinning = syncing || busy;
  const ready = stage === "ready";

  const stageCopy =
    stage === "empty"
      ? t("knowledge.corpusStageEmpty")
      : stage === "building"
        ? t("knowledge.corpusStageBuilding")
        : stage === "attention"
          ? t("knowledge.corpusStageAttention")
          : corpus.lastSyncedAt
            ? t("knowledge.lastSynced", {
                time: formatRelativeTime(corpus.lastSyncedAt),
              })
            : t("knowledge.corpusStageReady");

  const toggleDetails = () => setExpanded((v) => !v);

  const formatQuota = (p: KnowledgeQuotaPair) =>
    t("knowledge.quotaUsage", { used: p.used, limit: p.limit });

  const details =
    quotaOpen || expanded ? (
      <div className="space-y-4" data-testid="deal-room-knowledge-corpus-details">
        {quotaOpen ? (
          <div
            className="space-y-2 rounded-lg border border-border/60 bg-muted/20 px-3 py-3"
            data-testid="deal-room-knowledge-quota-panel"
          >
            <div className="flex items-center justify-between text-sm">
              <span className="text-muted-foreground">{t("knowledge.quotaKnowledgeBases")}</span>
              <span className="font-mono text-[13px] tabular-nums font-medium">
                {formatQuota(quotaRows.knowledgeBases)}
              </span>
            </div>
            <div className="flex items-center justify-between text-sm">
              <span className="text-muted-foreground">{t("knowledge.quotaDocuments")}</span>
              <span className="font-mono text-[13px] tabular-nums font-medium">
                {formatQuota(quotaRows.documents)}
              </span>
            </div>
            <div className="flex items-center justify-between text-sm">
              <span className="text-muted-foreground">{t("knowledge.quotaAnswers")}</span>
              <span className="font-mono text-[13px] tabular-nums font-medium">
                {formatQuota(quotaRows.answers)}
              </span>
            </div>
            <div className="flex items-center justify-between border-t border-border/50 pt-2">
              <span className="text-caption text-muted-foreground">
                {corpus.quota?.planCode
                  ? t("knowledge.quotaPlan", { plan: corpus.quota.planCode })
                  : t("knowledge.quotaPlanUnknown")}
              </span>
              <Button
                size="sm"
                variant="ghost"
                className="h-7 px-2 text-xs"
                data-testid="deal-room-knowledge-quota-upgrade"
                onClick={() => {
                  if (workspaceSlug) {
                    navigate(`/${workspaceSlug}/settings/billing`);
                  }
                }}
              >
                {t("knowledge.quotaUpgrade")}
              </Button>
            </div>
          </div>
        ) : null}

        {expanded ? (
          corpus.documents.length === 0 ? (
            <p className="px-1 py-6 text-center text-sm text-muted-foreground">
              {t("knowledge.emptyDocuments")}
            </p>
          ) : (
            <ul
              className="divide-y divide-border/50 overflow-y-auto overscroll-contain"
              style={{ maxHeight: `${CORPUS_DOC_LIST_VISIBLE * 3.5}rem` }}
              data-testid="deal-room-knowledge-doc-list"
            >
              {corpus.documents.map((doc) => (
                <li
                  key={doc.documentId}
                  className="flex min-h-14 items-start justify-between gap-3 py-3 text-sm first:pt-0"
                  data-testid="deal-room-knowledge-doc-row"
                >
                  <div className="flex min-w-0 items-start gap-2.5">
                    <span className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-border/60 bg-muted/30 text-foreground/60">
                      <FileText size={15} weight="duotone" />
                    </span>
                    <div className="min-w-0">
                      <p className="truncate font-medium tracking-tight">
                        {doc.title || doc.documentId}
                      </p>
                      {doc.status === "failed" ? (
                        <p className="text-xs text-destructive">
                          {t("knowledge.docSyncFailed")}
                        </p>
                      ) : (
                        <p className="font-mono text-[11px] text-muted-foreground">
                          {t("knowledge.chunkCount", { count: doc.chunkCount })}
                        </p>
                      )}
                    </div>
                  </div>
                  <Badge
                    variant="outline"
                    className={cn(
                      "shrink-0 border-border/70 font-normal",
                      doc.status === "synced" &&
                        "bg-emerald-50 text-emerald-800 border-emerald-200/80",
                      doc.status === "failed" &&
                        "bg-destructive/5 text-destructive border-destructive/20",
                      (doc.status === "pending" || doc.status === "syncing") &&
                        "bg-amber-50 text-amber-900 border-amber-200/80",
                    )}
                  >
                    {t(`knowledge.docStatus.${doc.status}`, {
                      defaultValue: doc.status,
                    })}
                  </Badge>
                </li>
              ))}
            </ul>
          )
        ) : null}
      </div>
    ) : null;

  return (
    <div data-testid="deal-room-knowledge-corpus" data-corpus-stage={stage}>
      <DealRoomMetricCard
        title={t("knowledge.corpusTitle")}
        status={
          <MetricStatusDot
            active={ready}
            activeLabel={statusLabel}
            inactiveLabel={statusLabel}
          />
        }
        metrics={[
          { label: t("stats.documents"), value: metrics.documentCount },
          { label: t("stats.views"), value: metrics.viewCount },
          { label: t("stats.activeLinks"), value: metrics.activeLinkCount },
        ]}
        footerNote={
          <span className="flex flex-col gap-0.5">
            <span>{stageCopy}</span>
            {stage === "attention" ? (
              <span className="flex items-center gap-1 text-destructive">
                <WarningCircle size={12} weight="fill" />
                {t("knowledge.syncIssue")}
              </span>
            ) : null}
            {busy ? (
              <span>
                {t("knowledge.syncProgress", {
                  synced: corpus.progress?.synced ?? 0,
                  total: corpus.progress?.total ?? 0,
                  pending: corpus.progress?.pending ?? 0,
                  failed: corpus.progress?.failed ?? 0,
                })}
              </span>
            ) : null}
          </span>
        }
        footerActions={
          <>
            <Button
              size="sm"
              variant="outline"
              className="h-8"
              onClick={() => setQuotaOpen((v) => !v)}
              aria-expanded={quotaOpen}
              data-testid="deal-room-knowledge-quota-toggle"
            >
              {t("knowledge.quotaToggle")}
              {quotaOpen ? (
                <CaretUp size={14} className="ml-1.5" weight="bold" />
              ) : (
                <CaretDown size={14} className="ml-1.5" weight="bold" />
              )}
            </Button>
            {onSync ? (
            <Button
              size="sm"
              variant="outline"
              className="h-8"
              disabled={syncing}
              onClick={onSync}
              data-testid="deal-room-knowledge-sync"
            >
              <ArrowsClockwise
                size={14}
                className={cn("mr-1.5", spinning && "animate-spin")}
              />
              {syncing ? t("knowledge.syncing") : t("knowledge.sync")}
            </Button>
            ) : null}
            <Button
              size="sm"
              variant="outline"
              className="h-8"
              onClick={toggleDetails}
              aria-expanded={expanded}
              data-testid="deal-room-knowledge-corpus-expand"
            >
              {t("knowledge.viewDetails")}
              {expanded ? (
                <CaretUp size={14} className="ml-1.5" weight="bold" />
              ) : (
                <CaretDown size={14} className="ml-1.5" weight="bold" />
              )}
            </Button>
          </>
        }
        details={details}
      />
    </div>
  );
}
