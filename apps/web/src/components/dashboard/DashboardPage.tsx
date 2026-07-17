import { useMemo } from "react";
import { useParams, useNavigate } from "react-router";
import { motion } from "motion/react";
import { ArrowRight } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { useTranslation } from "react-i18next";
import { useReducedMotion } from "@/hooks/useReducedMotion";
import { useAsyncData } from "@/hooks/useAsyncData";
import { api, type DashboardStats, type InsightsOverview } from "@/lib/api";
import { useSignalStore } from "@/stores/signalStore";
import { sortSignals } from "@/lib/sortSignals";
import type { ActionItem, DealRoom } from "@/types";
import { DashboardHeader } from "./DashboardHeader";
import { MetricsCards } from "./MetricsCards";
import { AttentionZone } from "./AttentionZone";
import { ActiveRoomsSection } from "./ActiveRoomsSection";
import { RecentActivityFeed } from "./RecentActivityFeed";
import { RecentVisitorsFeed } from "./RecentVisitorsFeed";
import { HeatMap } from "./HeatMap";

interface DashboardData {
  stats: DashboardStats;
  rooms: DealRoom[];
  insights: InsightsOverview | null;
}

export function DashboardPage() {
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const navigate = useNavigate();
  const reducedMotion = useReducedMotion();
  const { t } = useTranslation("dashboard");
  const { t: tCommon } = useTranslation("common");

  const { signals, actions, fetchSignals, updateActionStatus } =
    useSignalStore();

  const {
    data,
    loading,
    error,
    refetch,
  } = useAsyncData<DashboardData>(
    async () => {
      const [statsRes, signalsRes, roomsRes, insightsRes] = await Promise.allSettled([
        api.getDashboardStats(),
        fetchSignals(),
        api.getDealRooms(),
        api.getInsightsOverview(),
      ]);

      if (statsRes.status === "rejected") {
        throw statsRes.reason;
      }
      if (signalsRes.status === "rejected") {
        // fetchSignals mutates the signal store; leave signals as-is on failure.
        console.error("failed to fetch signals", signalsRes.reason);
      }

      return {
        stats: statsRes.value,
        rooms: roomsRes.status === "fulfilled" ? roomsRes.value.data : [],
        insights: insightsRes.status === "fulfilled" ? insightsRes.value : null,
      };
    },
    [fetchSignals]
  );

  const sortedSignals = useMemo(() => sortSignals(signals), [signals]);

  const handleActionClick = (action: ActionItem) => {
    if (!workspaceSlug || !action.sourceType || !action.sourceId) return;
    switch (action.sourceType) {
      case "link_access_request":
      case "link_question":
      case "uploaded_file":
      case "expiring_link":
        navigate(`/${workspaceSlug}/links/${action.sourceId}`);
        break;
      case "room_access_request":
      case "room_nda":
      case "expiring_room":
        navigate(`/${workspaceSlug}/deal-rooms/${action.sourceId}`);
        break;
    }
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
      <div className="space-y-10">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-2">
            <Skeleton className="h-8 w-48" />
            <Skeleton className="h-5 w-64" />
          </div>
          <div className="flex gap-2">
            <Skeleton className="h-9 w-28" />
            <Skeleton className="h-9 w-24" />
            <Skeleton className="h-9 w-24" />
          </div>
        </div>
        <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-24" />
          ))}
        </div>
        <div className="grid grid-cols-1 gap-8 lg:grid-cols-3">
          <div className="space-y-8 lg:col-span-2">
            <Skeleton className="h-80" />
            <Skeleton className="h-80" />
          </div>
          <div className="space-y-8">
            <Skeleton className="h-80" />
            <Skeleton className="h-72" />
            <Skeleton className="h-96" />
          </div>
        </div>
      </div>
    );
  }

  const { stats, rooms, insights } = data;
  const slug = workspaceSlug ?? "";
  const activeRoomsCount = rooms.filter((r) => r.status === "active").length;
  const highIntentContacts = insights?.topContacts?.length ?? 0;

  return (
    <motion.div
      initial={reducedMotion ? false : { opacity: 0, y: 12 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.4, ease: [0.16, 1, 0.3, 1] }}
      className="space-y-10"
    >
      <DashboardHeader workspaceSlug={slug} />

      <MetricsCards
        workspaceSlug={slug}
        activeRooms={activeRoomsCount}
        weeklyVisitors={stats.weeklyVisitors}
        pendingQuestions={stats.pendingQuestions}
        highIntentContacts={highIntentContacts}
      />

      <div className="grid grid-cols-1 gap-8 lg:grid-cols-3">
        <div className="space-y-8 lg:col-span-2">
          <AttentionZone
            actions={actions}
            signals={sortedSignals}
            riskAlerts={stats.riskAlerts}
            workspaceSlug={slug}
            onActionStatusChange={updateActionStatus}
            onActionClick={handleActionClick}
          />

          <RecentActivityFeed
            activities={stats.recentActivities}
            workspaceSlug={slug}
          />
        </div>

        <div className="space-y-6">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-body flex items-center gap-2 font-medium text-muted-foreground">
                <ArrowRight size={16} className="text-hot-500" />
                {t("sections.heatMap")}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <HeatMap links={stats.recentLinks} />
            </CardContent>
          </Card>

          <RecentVisitorsFeed insights={insights} workspaceSlug={slug} />

          <ActiveRoomsSection rooms={rooms} workspaceSlug={slug} />
        </div>
      </div>
    </motion.div>
  );
}
