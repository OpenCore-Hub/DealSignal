import { useEffect, useState } from "react";
import { CircleNotch } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import type { DealRoomKnowledgeOpsSummary } from "@/types";
import { cn } from "@/lib/utils";

interface KnowledgeOpsStripProps {
  roomId: string;
  /** Bump to reload ops after gold review / asks. */
  refreshKey?: number | string;
  className?: string;
}

function shortFingerprint(fp?: string): string {
  const s = (fp || "").trim();
  if (!s) return "—";
  if (s.length <= 12) return s;
  return `${s.slice(0, 8)}…${s.slice(-4)}`;
}

/**
 * Compact workspace SLO / cost strip for the knowledge corpus view (ceiling Phase H).
 */
export function KnowledgeOpsStrip({
  roomId,
  refreshKey,
  className,
}: KnowledgeOpsStripProps) {
  const { t } = useTranslation("dealRooms");
  const [ops, setOps] = useState<DealRoomKnowledgeOpsSummary | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    void api
      .getDealRoomKnowledgeOps(roomId, { windowHours: 24 })
      .then((res) => {
        if (!cancelled) setOps(res);
      })
      .catch(() => {
        if (!cancelled) setOps(null);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [roomId, refreshKey]);

  return (
    <div
      className={cn(
        "flex flex-wrap items-center gap-x-4 gap-y-1 rounded-xl border border-border/60 bg-muted/20 px-3 py-2 text-[11px] text-muted-foreground",
        className,
      )}
      data-testid="deal-room-knowledge-ops"
    >
      {loading ? (
        <span className="inline-flex items-center gap-1.5">
          <CircleNotch size={12} className="animate-spin" />
          {t("knowledge.opsLoading")}
        </span>
      ) : ops ? (
        <>
          <span data-testid="deal-room-knowledge-ops-turns">
            {t("knowledge.opsTurns", { count: ops.turnsTotal, hours: ops.windowHours })}
          </span>
          <span data-testid="deal-room-knowledge-ops-latency">
            {t("knowledge.opsAvgLatency", {
              ms: Math.round(ops.avgDurationMs || 0),
            })}
          </span>
          <span data-testid="deal-room-knowledge-ops-p95">
            {t("knowledge.opsP95Latency", {
              ms: Math.round(ops.p95DurationMs || 0),
            })}
          </span>
          <span data-testid="deal-room-knowledge-ops-cost">
            {t("knowledge.opsCostUnits", {
              units: ops.costUnitsTotal ?? 0,
            })}
          </span>
          <span data-testid="deal-room-knowledge-ops-refusals">
            {t("knowledge.opsRefusals", {
              count: Object.values(ops.refusalsByKind ?? {}).reduce(
                (a, b) => a + b,
                0,
              ),
            })}
          </span>
          <span data-testid="deal-room-knowledge-ops-eval-pending">
            {t("knowledge.opsPendingEval", {
              count: ops.pendingEvalCandidates ?? 0,
            })}
          </span>
          <span data-testid="deal-room-knowledge-ops-eval-accepted">
            {t("knowledge.opsAcceptedEval", {
              count: ops.evalCandidatesByStatus?.accepted ?? 0,
            })}
          </span>
          <span data-testid="deal-room-knowledge-ops-quota">
            {t("knowledge.opsQuota", {
              used: ops.answersQuota.used,
              limit: ops.answersQuota.limit > 0 ? ops.answersQuota.limit : "∞",
            })}
          </span>
          <span data-testid="deal-room-knowledge-ops-archives">
            {t("knowledge.opsColdArchives", { count: ops.coldArchiveCount })}
          </span>
          <span
            className="font-mono text-[10px] text-foreground/55"
            data-testid="deal-room-knowledge-ops-fingerprint"
            title={ops.roomCorpusFingerprint || undefined}
          >
            {t("knowledge.opsFingerprint", {
              fingerprint: shortFingerprint(ops.roomCorpusFingerprint),
            })}
          </span>
        </>
      ) : (
        <span>{t("knowledge.opsUnavailable")}</span>
      )}
    </div>
  );
}
