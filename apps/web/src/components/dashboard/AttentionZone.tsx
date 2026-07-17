import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Fire, Warning, ListChecks } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { EmptyState } from "@/components/common/EmptyState";
import { ActionList } from "./ActionList";
import { SignalCard } from "./SignalCard";
import { RiskAlertList } from "./RiskAlertList";
import type { Signal, ActionItem, ActionStatus, RiskAlert } from "@/types";

interface AttentionZoneProps {
  actions: ActionItem[];
  signals: Signal[];
  riskAlerts: RiskAlert[];
  workspaceSlug?: string;
  onActionStatusChange: (id: string, status: ActionStatus) => void;
  onActionClick?: (action: ActionItem) => void;
}

function itemTimestamp(item: unknown): number | null {
  const createdAt = (item as { createdAt?: string }).createdAt;
  if (!createdAt) return null;
  const ts = new Date(createdAt).getTime();
  return Number.isNaN(ts) ? null : ts;
}

function keepLatestByKey<T>(items: T[], keyFn: (item: T) => string): T[] {
  const map = new Map<string, T>();
  for (const item of items) {
    const key = keyFn(item);
    const existing = map.get(key);
    const itemTs = itemTimestamp(item);
    const existingTs = existing ? itemTimestamp(existing) : null;
    if (!existing || (itemTs !== null && (existingTs === null || itemTs > existingTs))) {
      map.set(key, item);
    }
  }
  return Array.from(map.values());
}

export function AttentionZone({
  actions,
  signals,
  riskAlerts,
  workspaceSlug,
  onActionStatusChange,
  onActionClick,
}: AttentionZoneProps) {
  const { t } = useTranslation("dashboard");

  // Deduplicate signals by the business identity of a signal:
  // same link + same subtype + same title means the same underlying insight.
  const dedupedSignals = useMemo(
    () =>
      keepLatestByKey(
        signals,
        (s) => `${s.linkId ?? "no-link"}|${s.subtype ?? "no-subtype"}|${s.title}`
      ),
    [signals]
  );

  const keptSignalIds = useMemo(
    () => new Set(dedupedSignals.map((s) => s.id)),
    [dedupedSignals]
  );

  // Actions are either tied to a signal or sourced from an operational event.
  // Keep signal-backed actions only when their signal survived deduplication;
  // operational actions are always retained. Drop residual duplicates by key.
  const dedupedActions = useMemo(() => {
    const retained = actions.filter(
      (a) => a.sourceType || (a.signalId && keptSignalIds.has(a.signalId))
    );
    return keepLatestByKey(
      retained,
      (a) => `${a.signalId ?? a.sourceId ?? a.id}|${a.title}`
    );
  }, [actions, keptSignalIds]);

  const hotSignals = useMemo(
    () => dedupedSignals.filter((s) => s.type === "hot_signal"),
    [dedupedSignals]
  );

  // Risk alerts come from a separate endpoint but represent the same
  // underlying risk signals; dedupe by link + type + title + description.
  const dedupedRiskAlerts = useMemo(
    () =>
      keepLatestByKey(
        riskAlerts,
        (r) =>
          `${r.linkId ?? "no-link"}|${r.type}|${r.title}|${r.description}`
      ),
    [riskAlerts]
  );

  const defaultTab =
    dedupedActions.filter((a) => a.status === "pending").length > 0
      ? "actions"
      : hotSignals.length > 0
        ? "signals"
        : dedupedRiskAlerts.length > 0
          ? "risks"
          : "actions";

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-h2 flex items-center gap-2">
          <Warning size={20} />
          {t("sections.attention")}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <Tabs defaultValue={defaultTab}>
          <TabsList className="mb-4 grid w-full grid-cols-3">
            <TabsTrigger value="actions" title={t("attention.actions")} className="gap-1.5">
              <ListChecks size={16} />
              <span className="truncate">{t("attention.actions")}</span>
              {dedupedActions.filter((a) => a.status === "pending").length > 0 && (
                <span className="rounded-full bg-primary px-1.5 py-0.5 text-xs text-primary-foreground">
                  {dedupedActions.filter((a) => a.status === "pending").length}
                </span>
              )}
            </TabsTrigger>
            <TabsTrigger value="signals" title={t("attention.signals")} className="gap-1.5">
              <Fire size={16} />
              <span className="truncate">{t("attention.signals")}</span>
              {hotSignals.length > 0 && (
                <span className="rounded-full bg-hot-500 px-1.5 py-0.5 text-xs text-white">
                  {hotSignals.length}
                </span>
              )}
            </TabsTrigger>
            <TabsTrigger value="risks" title={t("attention.risks")} className="gap-1.5">
              <Warning size={16} />
              <span className="truncate">{t("attention.risks")}</span>
              {dedupedRiskAlerts.length > 0 && (
                <span className="rounded-full bg-risk-500 px-1.5 py-0.5 text-xs text-white">
                  {dedupedRiskAlerts.length}
                </span>
              )}
            </TabsTrigger>
          </TabsList>

          <TabsContent value="actions">
            <ActionList
              actions={dedupedActions}
              onStatusChange={onActionStatusChange}
              onActionClick={onActionClick}
            />
          </TabsContent>

          <TabsContent value="signals">
            {hotSignals.length === 0 ? (
              <EmptyState
                size="compact"
                icon={<Fire size={32} />}
                title={t("empty.signals.title")}
                description={t("empty.signals.description")}
              />
            ) : (
              <div className="max-h-[520px] overflow-y-auto pr-1">
                <div className="space-y-4">
                  {hotSignals.map((signal) => (
                    <SignalCard
                      key={signal.id}
                      signal={signal}
                      action={dedupedActions.find((a) => a.signalId === signal.id)}
                      onActionStatusChange={onActionStatusChange}
                    />
                  ))}
                </div>
              </div>
            )}
          </TabsContent>

          <TabsContent value="risks">
            {dedupedRiskAlerts.length === 0 ? (
              <EmptyState
                size="compact"
                icon={<Warning size={32} />}
                title={t("riskAlerts.title")}
                description={t("empty.risks.description")}
              />
            ) : (
              <RiskAlertList alerts={dedupedRiskAlerts} workspaceSlug={workspaceSlug} />
            )}
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  );
}
