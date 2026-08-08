import { useEffect, useMemo, useState } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import {
  CaretDown,
  CaretRight,
  ChartLine,
  CheckCircle,
} from "@phosphor-icons/react";
import { EmptyState } from "@/components/common/EmptyState";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { documentsSharePath } from "@/lib/documentsSharePath";
import {
  countRadarFilters,
  defaultOutcomeForProduct,
  filterRadarItems,
  filterServerStrands,
  flatRadarOrder,
  isEditableKeyboardTarget,
  parseRadarCircle,
  parseRadarFilter,
  RADAR_CIRCLES,
  RADAR_FILTERS,
  type RadarFilter,
  type RadarFeed,
  type RadarOutcome,
  type RadarWorkItem,
} from "@/lib/radarQueue";
import { RadarRow, type SnoozeHours } from "./RadarRow";
import type { ActionStatus } from "@/types";

interface RadarQueueProps {
  workspaceSlug: string;
  feed: RadarFeed;
  selectedId: string | null;
  onSelect: (item: RadarWorkItem) => void;
  onPrimary: (item: RadarWorkItem) => void;
  onStatusChange: (
    actionId: string,
    status: ActionStatus,
    snoozeHours?: SnoozeHours,
    outcome?: RadarOutcome,
  ) => void;
}

export function RadarQueue({
  workspaceSlug,
  feed,
  selectedId,
  onSelect,
  onPrimary,
  onStatusChange,
}: RadarQueueProps) {
  const { t } = useTranslation("dashboard");
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const filter = parseRadarFilter(searchParams.get("filter"));
  const circle = parseRadarCircle(
    searchParams.get("circle") ?? feed.lens ?? "founder",
  );
  const counts = useMemo(
    () => countRadarFilters(feed.items, feed.counts),
    [feed.items, feed.counts],
  );
  const visible = useMemo(
    () => filterRadarItems(feed.items, filter),
    [feed.items, filter],
  );
  const nextUpId =
    filter === "all" && feed.nextUp
      ? (visible.find((i) => i.id === feed.nextUp?.id)?.id ?? visible[0]?.id)
      : visible[0]?.id;
  const nextUp = nextUpId
    ? visible.find((i) => i.id === nextUpId)
    : undefined;

  const strands = useMemo(
    () => filterServerStrands(feed.strands ?? [], filter, nextUpId),
    [feed.strands, filter, nextUpId],
  );

  const ordered = useMemo(
    () => flatRadarOrder(nextUp, strands),
    [nextUp, strands],
  );

  const strandSignature = strands
    .map((s) => `${s.dealKey}:${s.items.map((i) => i.id).join(",")}`)
    .join("|");

  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});

  useEffect(() => {
    // Expand strand that holds the selected / next-up item; collapse the rest when many.
    const next: Record<string, boolean> = {};
    for (const strand of strands) {
      const focusId = selectedId || nextUpId;
      const hasFocus = focusId
        ? strand.items.some((i) => i.id === focusId)
        : false;
      next[strand.dealKey] = strands.length > 1 ? !hasFocus : false;
    }
    setCollapsed(next);
    // strandSignature captures strand membership without depending on array identity.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- intentional
  }, [strandSignature, selectedId, nextUpId]);

  const setFilter = (next: RadarFilter) => {
    const params = new URLSearchParams(searchParams);
    if (next === "all") params.delete("filter");
    else params.set("filter", next);
    setSearchParams(params, { replace: true });
  };

  const setCircle = (next: ReturnType<typeof parseRadarCircle>) => {
    const params = new URLSearchParams(searchParams);
    if (next === "founder") params.delete("circle");
    else params.set("circle", next);
    setSearchParams(params, { replace: true });
  };

  useEffect(() => {
    if (ordered.length === 0) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.metaKey || event.ctrlKey || event.altKey) return;
      if (isEditableKeyboardTarget(event.target)) return;
      const key = event.key.toLowerCase();
      const currentId = selectedId ?? ordered[0]?.id;
      const idx = Math.max(
        0,
        ordered.findIndex((i) => i.id === currentId),
      );
      const focus = ordered[idx] ?? ordered[0];
      if (!focus) return;

      if (key === "j" || key === "arrowdown") {
        event.preventDefault();
        const next = ordered[Math.min(ordered.length - 1, idx + 1)];
        if (next) onSelect(next);
        return;
      }
      if (key === "k" || key === "arrowup") {
        event.preventDefault();
        const prev = ordered[Math.max(0, idx - 1)];
        if (prev) onSelect(prev);
        return;
      }
      if (key === "e" || key === "d") {
        event.preventDefault();
        onStatusChange(
          focus.actionId,
          "done",
          undefined,
          defaultOutcomeForProduct(focus.product),
        );
        return;
      }
      if (key === "s") {
        event.preventDefault();
        onStatusChange(focus.actionId, "snoozed", 24);
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [ordered, selectedId, onSelect, onStatusChange]);

  return (
    <section data-testid="radar-queue" className="min-w-0">
      <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h2 className="text-h2">{t("radar.upNext")}</h2>
          <p className="text-caption mt-0.5 text-muted-foreground">
            {t("radar.openCount", { count: feed.items.length })}
          </p>
        </div>
        <div className="flex flex-col items-stretch gap-2 sm:items-end">
          <div
            className="flex flex-wrap gap-1.5"
            role="group"
            aria-label={t("radar.lensLabel")}
            data-testid="radar-lens"
          >
            {RADAR_CIRCLES.map((key) => {
              const active = circle === key;
              return (
                <button
                  key={key}
                  type="button"
                  data-testid={`radar-lens-${key}`}
                  aria-pressed={active}
                  onClick={() => setCircle(key)}
                  className={cn(
                    "inline-flex items-center rounded-md border px-2.5 py-1 text-xs font-medium transition-colors",
                    active
                      ? "border-foreground/40 bg-muted text-foreground"
                      : "border-border bg-background text-muted-foreground hover:border-muted-foreground/40 hover:text-foreground",
                  )}
                >
                  {t(`radar.lens.${key}`)}
                </button>
              );
            })}
          </div>
          <div
            className="flex flex-wrap gap-1.5"
            role="tablist"
            aria-label={t("radar.filtersLabel")}
          >
            {RADAR_FILTERS.map((key) => {
              const active = filter === key;
              return (
                <button
                  key={key}
                  type="button"
                  role="tab"
                  aria-selected={active}
                  data-testid={`radar-filter-${key}`}
                  onClick={() => setFilter(key)}
                  className={cn(
                    "inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1 text-xs font-medium transition-colors",
                    active
                      ? "border-foreground bg-foreground text-background"
                      : "border-border bg-background text-muted-foreground hover:border-muted-foreground/40 hover:text-foreground",
                  )}
                >
                  {t(`radar.filters.${key}`)}
                  <span
                    className={cn(
                      "tabular-nums",
                      active ? "text-background/80" : "text-muted-foreground",
                    )}
                  >
                    {counts[key]}
                  </span>
                </button>
              );
            })}
          </div>
        </div>
      </div>

      {feed.noiseHints && feed.noiseHints.length > 0 ? (
        <p
          className="mb-3 text-caption text-muted-foreground"
          data-testid="radar-noise-hints"
        >
          {t("radar.noiseHint", {
            product: t(`radar.products.${feed.noiseHints[0].product}`),
            rate: Math.round(feed.noiseHints[0].falsePositiveRate * 100),
            sample: feed.noiseHints[0].sample,
          })}
        </p>
      ) : null}

      {visible.length === 0 ? (
        filter === "all" && feed.clearedToday > 0 ? (
          <div
            data-testid="radar-inbox-zero"
            className="flex flex-col items-center justify-center rounded-xl border border-border bg-card px-6 py-12 text-center"
          >
            <CheckCircle
              size={40}
              weight="duotone"
              className="text-muted-foreground"
            />
            <h3 className="text-h3 mt-4 text-foreground">
              {t("radar.inboxZero.title")}
            </h3>
            <p className="text-body mt-2 max-w-md text-muted-foreground">
              {t("radar.inboxZero.description", {
                count: feed.clearedToday,
              })}
            </p>
            <div className="mt-6 flex flex-wrap items-center justify-center gap-2">
              <Button
                className="gap-1.5"
                onClick={() =>
                  navigate(`/${workspaceSlug}/insights/overview`)
                }
              >
                <ChartLine size={16} />
                {t("radar.analyzeInInsights")}
              </Button>
              <Button
                variant="ghost"
                onClick={() => navigate(documentsSharePath(workspaceSlug))}
              >
                {t("empty.actions.shareCta")}
              </Button>
            </div>
          </div>
        ) : (
          <EmptyState
            size="default"
            icon={<CheckCircle size={40} weight="duotone" />}
            title={
              filter === "all"
                ? t("empty.actions.title")
                : t("empty.actions.filteredTitle")
            }
            description={
              filter === "all"
                ? t("empty.actions.description")
                : t("empty.actions.filteredDescription")
            }
            action={
              filter === "all"
                ? {
                    label: t("empty.actions.shareCta"),
                    onClick: () =>
                      navigate(documentsSharePath(workspaceSlug)),
                  }
                : {
                    label: t("radar.filters.all"),
                    onClick: () => setFilter("all"),
                  }
            }
            className="border border-border bg-card"
          />
        )
      ) : (
        <div className="space-y-4">
          {nextUp ? (
            <div
              data-testid="radar-next-up"
              className="overflow-hidden rounded-xl border border-foreground/20 bg-card"
            >
              <div className="flex items-center justify-between gap-2 border-b border-border px-3 py-2">
                <p className="text-caption font-medium text-muted-foreground">
                  {t("radar.nextUpLabel")}
                </p>
                <p className="text-caption text-muted-foreground">
                  {t("radar.shortcuts.hint")}
                </p>
              </div>
              <RadarRow
                item={nextUp}
                emphasized
                selected={selectedId === nextUp.id}
                onPrimary={onPrimary}
                onSelect={onSelect}
                onEvidence={onSelect}
                onStatusChange={onStatusChange}
              />
            </div>
          ) : null}

          {strands.length > 0 ? (
            <div className="space-y-3" data-testid="radar-strands">
              <h3 className="text-sm font-semibold">{t("radar.byDeal")}</h3>
              {strands.map((strand) => {
                const isCollapsed = collapsed[strand.dealKey] ?? false;
                const preview = isCollapsed
                  ? strand.items.slice(0, 1)
                  : strand.items;
                return (
                  <div
                    key={strand.dealKey}
                    data-testid="radar-strand"
                    data-deal-key={strand.dealKey}
                    className="overflow-hidden rounded-xl border border-border bg-card"
                  >
                    <button
                      type="button"
                      className="flex w-full items-center gap-2 border-b border-border px-3 py-2.5 text-left hover:bg-muted/40"
                      onClick={() =>
                        setCollapsed((prev) => ({
                          ...prev,
                          [strand.dealKey]: !isCollapsed,
                        }))
                      }
                      aria-expanded={!isCollapsed}
                    >
                      {isCollapsed ? (
                        <CaretRight size={14} className="text-muted-foreground" />
                      ) : (
                        <CaretDown size={14} className="text-muted-foreground" />
                      )}
                      <span className="min-w-0 flex-1 truncate text-sm font-medium">
                        {strand.dealName}
                      </span>
                      <span className="text-caption tabular-nums text-muted-foreground">
                        {t("radar.strandCount", { count: strand.items.length })}
                      </span>
                    </button>
                    <div role="list">
                      {preview.map((item) => (
                        <RadarRow
                          key={item.id}
                          item={item}
                          selected={selectedId === item.id}
                          onPrimary={onPrimary}
                          onSelect={onSelect}
                          onEvidence={onSelect}
                          onStatusChange={onStatusChange}
                        />
                      ))}
                    </div>
                    {isCollapsed && strand.items.length > 1 ? (
                      <button
                        type="button"
                        className="w-full border-t border-border px-3 py-2 text-left text-caption text-muted-foreground hover:bg-muted/40"
                        onClick={() =>
                          setCollapsed((prev) => ({
                            ...prev,
                            [strand.dealKey]: false,
                          }))
                        }
                      >
                        {t("radar.showMoreInStrand", {
                          count: strand.items.length - 1,
                        })}
                      </button>
                    ) : null}
                  </div>
                );
              })}
            </div>
          ) : null}
        </div>
      )}
    </section>
  );
}
