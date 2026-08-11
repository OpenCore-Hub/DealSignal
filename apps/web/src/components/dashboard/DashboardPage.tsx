import { useEffect, useMemo, useState } from "react";
import {
  Link,
  useLocation,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router";
import { motion } from "motion/react";
import { ChartLine, ArrowRight } from "@phosphor-icons/react";
import { toast } from "sonner";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { useTranslation } from "react-i18next";
import { useReducedMotion } from "@/hooks/useReducedMotion";
import { useAsyncData } from "@/hooks/useAsyncData";
import { api } from "@/lib/api";
import { apiErrorMessage } from "@/lib/apiErrors";
import { useRadarStore } from "@/stores/radarStore";
import type { ActionStatus } from "@/types";
import {
  decrementRadarCounts,
  parseRadarCircle,
  withMailtoHrefs,
  type RadarFeed,
  type RadarOutcome,
  type RadarWorkItem,
} from "@/lib/radarQueue";
import { RadarEvidenceRail } from "./RadarEvidenceRail";
import { RadarQueue } from "./RadarQueue";
import type { SnoozeHours } from "./RadarRow";

export function DashboardPage() {
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const circleQuery = searchParams.get("circle");
  const circleExplicit = Boolean(circleQuery && circleQuery.trim());
  const circle = circleExplicit ? parseRadarCircle(circleQuery) : null;
  const reducedMotion = useReducedMotion();
  const { t } = useTranslation("dashboard");
  const { t: tCommon } = useTranslation("common");
  const { t: tInsights } = useTranslation("insights");
  const [feedOverride, setFeedOverride] = useState<RadarFeed | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const setOpenCount = useRadarStore((s) => s.setOpenCount);

  useEffect(() => {
    setFeedOverride(null);
  }, [circleExplicit, circle]);

  const { data, loading, error, refetch } = useAsyncData<RadarFeed>(
    async () =>
      api.getRadar(circleExplicit && circle ? { circle } : undefined),
    [circleExplicit, circle],
  );

  const feed = useMemo(() => {
    const base = feedOverride ?? data;
    if (!base) {
      return {
        nextUp: null,
        strands: [],
        items: [],
        clearedToday: 0,
        counts: { all: 0 },
      } satisfies RadarFeed;
    }
    const items = withMailtoHrefs(base.items, {
      subject: (document) =>
        tInsights("suggestions.emailSubject", { document }),
      body: ({ email, document, action }) =>
        tInsights("suggestions.emailBody", { email, document, action }),
    });
    const byId = new Map(items.map((i) => [i.id, i]));
    const strands = (base.strands ?? []).map((strand) => ({
      ...strand,
      dealName: strand.dealName || t("radar.dealFallback"),
      items: strand.items
        .map((i) => byId.get(i.id) ?? i)
        .map((i) => ({
          ...i,
          dealName: i.dealName || t("radar.dealFallback"),
        })),
    }));
    const nextUpRaw =
      base.nextUp && byId.get(base.nextUp.id)
        ? byId.get(base.nextUp.id)!
        : items[0] ?? null;
    const nextUp = nextUpRaw
      ? { ...nextUpRaw, dealName: nextUpRaw.dealName || t("radar.dealFallback") }
      : null;
    return {
      ...base,
      items: items.map((i) => ({
        ...i,
        dealName: i.dealName || t("radar.dealFallback"),
      })),
      strands,
      nextUp,
    };
  }, [feedOverride, data, tInsights, t]);

  useEffect(() => {
    setOpenCount(feed.items.length);
  }, [feed.items.length, setOpenCount]);

  useEffect(() => {
    if (!selectedId && feed.nextUp) {
      setSelectedId(feed.nextUp.id);
      return;
    }
    if (selectedId && !feed.items.some((i) => i.id === selectedId)) {
      setSelectedId(feed.nextUp?.id ?? feed.items[0]?.id ?? null);
    }
  }, [feed.items, feed.nextUp, selectedId]);

  const selectedItem =
    feed.items.find((i) => i.id === selectedId) ?? feed.nextUp ?? null;

  const handlePrimary = (item: RadarWorkItem) => {
    setSelectedId(item.id);
    if (item.verb === "email" && item.mailtoHref) {
      window.open(item.mailtoHref, "_blank", "noopener,noreferrer");
      return;
    }
    const path = item.navigatePath || item.evidencePath;
    if (path) {
      navigate(path, {
        state: {
          returnTo: location.pathname + location.search,
          returnLabel: tCommon("back"),
        },
      });
      return;
    }
    // Never silent-no-op: host must know the CTA has nowhere to go.
    toast.message(t("radar.toast.primaryUnavailable"));
  };

  const handleStatusChange = (
    id: string,
    status: ActionStatus,
    snoozeHours?: SnoozeHours,
    outcome?: RadarOutcome,
  ) => {
    const previous = feed;
    const removed = feed.items.find((i) => i.id === id);
    const remaining = feed.items.filter((i) => i.id !== id);
    setFeedOverride({
      ...feed,
      items: remaining,
      strands: feed.strands
        .map((s) => ({
          ...s,
          items: s.items.filter((i) => i.id !== id),
        }))
        .filter((s) => s.items.length > 0),
      nextUp:
        feed.nextUp?.id === id
          ? (remaining[0] ?? null)
          : feed.nextUp && remaining.some((i) => i.id === feed.nextUp?.id)
            ? feed.nextUp
            : (remaining[0] ?? null),
      clearedToday:
        status === "done" ? feed.clearedToday + 1 : feed.clearedToday,
      counts: decrementRadarCounts(
        feed.counts,
        feed.items.length,
        removed?.product,
      ),
    });
    void api
      .updateRadarItem(id, status, snoozeHours, outcome)
      .then(() => {
        toast(t(`radar.toast.${status}`), {
          action: {
            label: t("radar.undo"),
            onClick: () => {
              void api
                .updateRadarItem(id, "pending")
                .then(async () => {
                  setFeedOverride(null);
                  await refetch();
                })
                .catch((e) => {
                  toast.error(
                    apiErrorMessage(e) || t("radar.toast.undoFailed"),
                  );
                });
            },
          },
        });
        // Authoritative counts / nextUp from server (product buckets stay correct).
        void refetch().then(() => setFeedOverride(null));
      })
      .catch((e) => {
        setFeedOverride(previous);
        toast.error(apiErrorMessage(e));
      });
  };

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
        <p className="text-body text-muted-foreground">{error}</p>
        <Button onClick={refetch}>{tCommon("retry")}</Button>
      </div>
    );
  }

  if (loading || !data) {
    return (
      <div className="space-y-8">
        <div className="space-y-2">
          <Skeleton className="h-8 w-40" />
          <Skeleton className="h-5 w-56" />
        </div>
        <div className="grid grid-cols-1 gap-8 lg:grid-cols-12">
          <div className="space-y-3 lg:col-span-8">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-96 w-full" />
          </div>
          <div className="space-y-4 lg:col-span-4">
            <Skeleton className="h-40 w-full" />
            <Skeleton className="h-64 w-full" />
          </div>
        </div>
      </div>
    );
  }

  const slug = workspaceSlug ?? "";

  return (
    <motion.div
      initial={reducedMotion ? false : { opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.28, ease: [0.16, 1, 0.3, 1] }}
      className="space-y-8"
    >
      <div className="grid grid-cols-1 gap-8 lg:grid-cols-12">
        <div className="min-w-0 lg:col-span-8">
          <RadarQueue
            workspaceSlug={slug}
            feed={feed}
            selectedId={selectedId}
            onSelect={(item) => setSelectedId(item.id)}
            onPrimary={handlePrimary}
            onStatusChange={handleStatusChange}
          />
        </div>

        <aside className="space-y-6 lg:col-span-4">
          <div
            className="rounded-xl border border-border px-4 py-3"
            data-testid="radar-cleared-today"
          >
            <p className="text-caption text-muted-foreground">
              {t("radar.clearedToday")}
            </p>
            <p className="text-stat mt-1 tabular-nums">{feed.clearedToday}</p>
          </div>

          <RadarEvidenceRail item={selectedItem} workspaceSlug={slug} />

          <Link
            to={`/${slug}/insights/overview`}
            state={{
              returnTo: location.pathname + location.search,
              returnLabel: tCommon("back"),
            }}
            className="flex items-center justify-between rounded-xl border border-border px-4 py-3 text-sm transition-colors hover:bg-muted/40"
            data-testid="radar-insights-link"
          >
            <span className="flex items-center gap-2 font-medium">
              <ChartLine size={16} />
              {t("radar.analyzeInInsights")}
            </span>
            <ArrowRight size={16} className="text-muted-foreground" />
          </Link>
        </aside>
      </div>
    </motion.div>
  );
}
