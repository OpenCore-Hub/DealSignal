import { useEffect, useState } from "react";
import {
  CaretDown,
  CaretUp,
  CheckCircle,
  Circle,
  ListChecks,
} from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { api } from "@/lib/api";
import type {
  DealRoomKnowledgeMissionPack,
  DealRoomKnowledgeMissionProgress,
} from "@/types";
import { cn } from "@/lib/utils";

/** Show ~5 compact rows, then scroll. */
const RAIL_LIST_SCROLL =
  "max-h-[calc(5*2.625rem+4*0.375rem)] overflow-y-auto overscroll-contain";

interface MissionProgressRailProps {
  roomId: string;
  sessionId?: string | null;
  /** Bump after each ask so coverage reloads against latest state. */
  refreshKey?: number | string;
  onAskItem?: (prompt: string) => void;
  className?: string;
}

/**
 * Mission pack checklist progress against audited session state (ceiling Phase N / L3).
 * Pack is soft-defaulted from the room creation template; switching is a secondary action.
 */
export function MissionProgressRail({
  roomId,
  sessionId,
  refreshKey,
  onAskItem,
  className,
}: MissionProgressRailProps) {
  const { t } = useTranslation("dealRooms");
  const [progress, setProgress] = useState<DealRoomKnowledgeMissionProgress | null>(
    null,
  );
  const [catalog, setCatalog] = useState<DealRoomKnowledgeMissionPack[]>([]);
  const [loading, setLoading] = useState(true);
  const [switching, setSwitching] = useState(false);
  const [expanded, setExpanded] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    void Promise.all([
      api.getDealRoomKnowledgeMissionProgress(roomId, {
        sessionId: sessionId || undefined,
      }),
      api.listDealRoomKnowledgeMissions(roomId),
    ])
      .then(([prog, cat]) => {
        if (cancelled) return;
        setProgress(prog);
        setCatalog(cat.items ?? []);
      })
      .catch(() => {
        if (!cancelled) {
          setProgress(null);
          setCatalog([]);
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [roomId, sessionId, refreshKey]);

  const uncoveredCount = progress
    ? progress.items.filter((it) => !it.covered).length
    : 0;

  // Expand when there are uncovered checklist items; collapse when complete.
  useEffect(() => {
    if (!progress) return;
    setExpanded(uncoveredCount > 0);
  }, [progress?.packId, progress?.covered, progress?.total, uncoveredCount]);

  const onPackChange = async (packId: string) => {
    if (!packId || packId === progress?.packId || switching) return;
    setSwitching(true);
    try {
      await api.setDealRoomKnowledgeMission(roomId, { packId });
      const prog = await api.getDealRoomKnowledgeMissionProgress(roomId, {
        sessionId: sessionId || undefined,
      });
      setProgress(prog);
    } catch {
      /* keep prior progress */
    } finally {
      setSwitching(false);
    }
  };

  if (loading && !progress) {
    return (
      <aside
        className={cn(
          "rounded-xl border border-border/60 bg-muted/[0.15] px-3.5 py-3 text-[11px] text-muted-foreground",
          className,
        )}
        data-testid="knowledge-mission-progress-rail"
      >
        {t("knowledge.missionProgressLoading")}
      </aside>
    );
  }

  if (!progress || progress.total < 1) return null;

  const uncovered = progress.items.filter((it) => !it.covered);
  const pct =
    progress.total > 0
      ? Math.round((progress.covered / progress.total) * 100)
      : 0;
  const otherPacks = catalog.filter((p) => p.packId !== progress.packId);

  return (
    <aside
      className={cn(
        "rounded-xl border border-border/60 bg-muted/[0.15] px-3.5 py-3",
        className,
      )}
      data-testid="knowledge-mission-progress-rail"
      data-expanded={expanded ? "true" : "false"}
      data-source={progress.source}
      aria-label={t("knowledge.missionProgressTitle")}
    >
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="text-[12px] font-medium text-foreground/85">
            {progress.title}
          </p>
        </div>
        <div className="flex shrink-0 flex-wrap items-center justify-end gap-1">
          {otherPacks.length > 0 ? (
            <DropdownMenu>
              <DropdownMenuTrigger
                render={
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    className="h-7 shrink-0 px-2 text-[11px] text-muted-foreground"
                    disabled={switching}
                    data-testid="knowledge-mission-pack-change"
                  >
                    {t("knowledge.missionProgressChange")}
                  </Button>
                }
              />
              <DropdownMenuContent
                align="end"
                className="min-w-[14rem]"
                data-testid="knowledge-mission-pack-menu"
              >
                {catalog.map((p) => (
                  <DropdownMenuItem
                    key={p.packId}
                    disabled={switching || p.packId === progress.packId}
                    data-testid={`knowledge-mission-pack-option-${p.packId}`}
                    onClick={() => {
                      void onPackChange(p.packId);
                    }}
                  >
                    {p.title}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
          ) : null}
          <Button
            type="button"
            size="sm"
            variant="ghost"
            className="h-7 shrink-0 px-2 text-[11px] text-muted-foreground"
            onClick={() => setExpanded((v) => !v)}
            aria-expanded={expanded}
            data-testid="knowledge-mission-progress-toggle"
          >
            {expanded
              ? t("knowledge.missionProgressCollapse")
              : t("knowledge.missionProgressExpand")}
            {expanded ? (
              <CaretUp size={12} className="ml-1" weight="bold" />
            ) : (
              <CaretDown size={12} className="ml-1" weight="bold" />
            )}
          </Button>
        </div>
      </div>

      <div className="mt-2 flex items-center gap-2 text-[12px] text-foreground/80">
        <ListChecks size={13} weight="duotone" />
        <span data-testid="knowledge-mission-progress-count">
          {t("knowledge.missionProgressCount", {
            covered: progress.covered,
            total: progress.total,
          })}
        </span>
        <span className="text-muted-foreground">· {pct}%</span>
      </div>
      <div
        className="mt-2 h-1.5 overflow-hidden rounded-full bg-border/60"
        role="progressbar"
        aria-valuenow={progress.covered}
        aria-valuemin={0}
        aria-valuemax={progress.total}
      >
        <div
          className="h-full rounded-full bg-foreground/55 transition-[width]"
          style={{ width: `${pct}%` }}
        />
      </div>

      {expanded ? (
        <div className="mt-3" data-testid="knowledge-mission-progress-details">
          <p className="mb-2 text-[11px] leading-relaxed text-muted-foreground">
            {t("knowledge.missionProgressHint")}
          </p>

          <ul
            className={cn("space-y-1.5", RAIL_LIST_SCROLL)}
            data-testid="knowledge-mission-progress-items"
          >
            {progress.items.map((it) => (
              <li
                key={it.id}
                className="flex flex-wrap items-start justify-between gap-2 rounded-lg border border-border/50 bg-background/80 px-2.5 py-1.5"
                data-covered={it.covered ? "true" : "false"}
              >
                <div className="flex min-w-0 flex-1 items-start gap-1.5">
                  {it.covered ? (
                    <CheckCircle
                      size={14}
                      weight="fill"
                      className="mt-0.5 shrink-0 text-foreground/55"
                      aria-hidden
                    />
                  ) : (
                    <Circle
                      size={14}
                      weight="regular"
                      className="mt-0.5 shrink-0 text-muted-foreground"
                      aria-hidden
                    />
                  )}
                  <p
                    className={cn(
                      "text-[12px] leading-snug",
                      it.covered
                        ? "text-muted-foreground line-through decoration-border"
                        : "text-foreground/80",
                    )}
                  >
                    {it.prompt}
                  </p>
                </div>
                {!it.covered && onAskItem ? (
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    className="h-7 shrink-0 px-2 text-[11px]"
                    onClick={() => onAskItem(it.prompt)}
                  >
                    {t("knowledge.missionProgressAsk")}
                  </Button>
                ) : null}
              </li>
            ))}
          </ul>

          {uncovered.length === 0 ? (
            <p
              className="mt-2 text-[11px] text-muted-foreground"
              data-testid="knowledge-mission-progress-complete"
            >
              {t("knowledge.missionProgressComplete")}
            </p>
          ) : null}
        </div>
      ) : null}
    </aside>
  );
}
