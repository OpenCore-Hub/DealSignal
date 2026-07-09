import { useMemo } from "react";
import { useNavigate, useParams } from "react-router";
import { motion } from "motion/react";
import {
  Fire,
  Warning,
  CheckCircle,
  Link as LinkIcon,
  FileText,
  ArrowRight,
} from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { useTranslation } from "react-i18next";

import { useReducedMotion } from "@/hooks/useReducedMotion";
import { useAsyncData } from "@/hooks/useAsyncData";
import { api, type DashboardStats } from "@/lib/api";
import { useSignalStore } from "@/stores/signalStore";
import { sortSignals } from "@/lib/sortSignals";
import { SignalCard } from "./SignalCard";
import { ActionList } from "./ActionList";
import { HeatMap } from "./HeatMap";
import { EmptyState } from "@/components/common/EmptyState";

function SummaryCards({
  stats,
  pendingActions,
}: {
  stats: DashboardStats;
  pendingActions: number;
}) {
  const { t } = useTranslation("dashboard");
  const items = [
    {
      label: t("summary.hotSignals"),
      count: stats.hotCount,
      icon: Fire,
      color: "text-hot-500 bg-hot-500/10",
    },
    {
      label: t("summary.pendingActions"),
      count: pendingActions,
      icon: CheckCircle,
      color: "text-success-500 bg-success-500/10",
    },
  ];

  return (
    <div className="grid grid-cols-2 gap-4 stagger-children" role="region" aria-label={t("summary.title")}>
      {items.map((item) => {
        const Icon = item.icon;
        return (
          <Card key={item.label} aria-label={`${item.label}: ${item.count}`}>
            <CardContent className="flex items-center gap-3 p-4">
              <div className={`flex h-10 w-10 items-center justify-center rounded-md ${item.color}`}>
                <Icon size={20} weight="fill" />
              </div>
              <div>
                <p className="text-stat tabular-nums">{item.count}</p>
                <p className="text-caption text-muted-foreground">{item.label}</p>
              </div>
            </CardContent>
          </Card>
        );
      })}
    </div>
  );
}

function RiskAlerts({ alerts, workspaceSlug }: { alerts: DashboardStats["riskAlerts"]; workspaceSlug?: string }) {
  const { t } = useTranslation("dashboard");
  const navigate = useNavigate();
  if (alerts.length === 0) return null;

  return (
    <Card className="border-risk-500/20 bg-risk-500/5">
      <CardHeader className="pb-3">
        <CardTitle className="text-h2 flex items-center gap-2 text-risk-600">
          <Warning size={20} weight="fill" />
          {t("riskAlerts.title")}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <ul className="space-y-3">
          {alerts.map((alert) => {
            const handleClick = () => {
              if (alert.documentId) navigate(`/${workspaceSlug}/documents/${alert.documentId}`);
              else if (alert.linkId) navigate(`/${workspaceSlug}/links/${alert.linkId}`);
            };
            return (
              <li
                key={alert.id}
                role="link"
                tabIndex={0}
                aria-label={`${t(alert.title)}: ${t(alert.description)}`}
                className="flex cursor-pointer items-start gap-3 rounded-lg border border-border bg-card p-3 transition-colors hover:bg-muted"
                onClick={handleClick}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    handleClick();
                  }
                }}
              >
              <div className="mt-0.5 h-2 w-2 rounded-full bg-risk-500" />
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium">{t(alert.title)}</p>
                <p className="text-body text-muted-foreground">{t(alert.description)}</p>
              </div>
              <ArrowRight size={16} className="text-muted-foreground" />
            </li>
          ); })}
        </ul>
      </CardContent>
    </Card>
  );
}

export function DashboardPage() {
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const reducedMotion = useReducedMotion();
  const { t } = useTranslation("dashboard");
  const { t: tCommon } = useTranslation("common");

  const { signals, actions, fetchSignals, updateActionStatus } = useSignalStore();
  const {
    data: stats,
    loading,
    error,
    refetch,
  } = useAsyncData<DashboardStats>(
    async () => {
      const [dashboardStats] = await Promise.all([api.getDashboardStats(), fetchSignals()]);
      return dashboardStats;
    },
    [fetchSignals]
  );

  const pendingActions = actions.filter((a) => a.status === "pending").length;
  const sortedSignals = useMemo(() => sortSignals(signals), [signals]);

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
        <p className="text-body text-muted-foreground">{error}</p>
        <Button onClick={refetch}>{tCommon("retry")}</Button>
      </div>
    );
  }

  if (loading || !stats) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-48" />
        <div className="grid grid-cols-2 gap-4">
          {Array.from({ length: 2 }).map((_, i) => (
            <Skeleton key={i} className="h-24" />
          ))}
        </div>
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
          <Skeleton className="h-96 lg:col-span-2" />
          <Skeleton className="h-96" />
        </div>
      </div>
    );
  }

  return (
    <motion.div
      initial={reducedMotion ? false : { opacity: 0, y: 12 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.4, ease: [0.16, 1, 0.3, 1] }}
      className="space-y-8"
    >
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="space-y-1">
          <h1 className="text-h1">{t("title")}</h1>
          <p className="text-body text-muted-foreground">
            {t("subtitle")}
          </p>
        </div>

      </div>

      <SummaryCards stats={stats} pendingActions={pendingActions} />

      {stats.riskAlerts.length > 0 && (
        <RiskAlerts alerts={stats.riskAlerts} workspaceSlug={workspaceSlug} />
      )}

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3 stagger-children">
        <div className="space-y-6 lg:col-span-2 stagger-children">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-h2 flex items-center gap-2">
                <Fire size={20} className="text-hot-500" />
                {t("sections.signals")}
              </CardTitle>
            </CardHeader>
            <CardContent>
              {signals.length === 0 ? (
                <EmptyState
                  icon={<Fire size={48} />}
                  title={t("empty.signals.title")}
                  description={t("empty.signals.description")}
                  action={{
                    label: t("empty.signals.action"),
                    onClick: () => navigate(`/${workspaceSlug}/documents/upload`),
                  }}
                />
              ) : (
                <div className="space-y-4">
                  {sortedSignals.map((signal) => (
                    <SignalCard
                      key={signal.id}
                      signal={signal}
                      action={actions.find((a) => a.signalId === signal.id)}
                      onActionStatusChange={updateActionStatus}
                    />
                  ))}
                </div>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-h2 flex items-center gap-2">
                <FileText size={20} />
                {t("sections.recentDocuments")}
              </CardTitle>
            </CardHeader>
            <CardContent>
              {stats.recentDocuments.length === 0 ? (
                <EmptyState
                  icon={<FileText size={48} />}
                  title={t("empty.documents.title")}
                  description={t("empty.documents.description")}
                />
              ) : (
                <div className="space-y-3">
                  <ul className="space-y-3">
                    {stats.recentDocuments.slice(0, 3).map((doc) => {
                      const handleClick = () => navigate(`/${workspaceSlug}/documents/${doc.id}`);
                      return (
                        <li
                          key={doc.id}
                          role="link"
                          tabIndex={0}
                          className="flex cursor-pointer items-center justify-between rounded-md border border-border p-3 transition-colors hover:bg-muted"
                          onClick={handleClick}
                          onKeyDown={(e) => {
                            if (e.key === "Enter" || e.key === " ") {
                              e.preventDefault();
                              handleClick();
                            }
                          }}
                        >
                        <div className="min-w-0 flex-1">
                          <p className="truncate text-sm font-medium">{doc.title}</p>
                          <p className="text-caption text-muted-foreground">
                            {t("document.pageCount", { count: doc.pageCount })} · {tCommon(`status.${doc.status}`)}
                          </p>
                        </div>
                        <ArrowRight size={16} className="text-muted-foreground" />
                      </li>
                    ); })}
                  </ul>
                  {stats.recentDocuments.length > 3 && (
                    <div className="flex justify-end">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => navigate(`/${workspaceSlug}/documents`)}
                      >
                        {tCommon("viewAll")}
                        <ArrowRight size={16} className="ml-1" />
                      </Button>
                    </div>
                  )}
                </div>
              )}
            </CardContent>
          </Card>
        </div>

        <div className="space-y-6 stagger-children">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-h2 flex items-center gap-2">
                <CheckCircle size={20} />
                {t("sections.actions")}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <ActionList actions={actions} onStatusChange={updateActionStatus} />
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-h2 flex items-center gap-2">
                <LinkIcon size={20} />
                {t("sections.heatMap")}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <HeatMap links={stats.recentLinks} />
            </CardContent>
          </Card>
        </div>
      </div>
    </motion.div>
  );
}
