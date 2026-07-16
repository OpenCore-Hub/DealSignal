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
}

export function AttentionZone({
  actions,
  signals,
  riskAlerts,
  workspaceSlug,
  onActionStatusChange,
}: AttentionZoneProps) {
  const { t } = useTranslation("dashboard");

  const pendingActions = useMemo(
    () => actions.filter((a) => a.status === "pending"),
    [actions]
  );
  const hotSignals = useMemo(
    () => signals.filter((s) => s.type === "hot_signal"),
    [signals]
  );

  const defaultTab =
    pendingActions.length > 0
      ? "actions"
      : hotSignals.length > 0
        ? "signals"
        : riskAlerts.length > 0
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
              {pendingActions.length > 0 && (
                <span className="rounded-full bg-primary px-1.5 py-0.5 text-xs text-primary-foreground">
                  {pendingActions.length}
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
              {riskAlerts.length > 0 && (
                <span className="rounded-full bg-risk-500 px-1.5 py-0.5 text-xs text-white">
                  {riskAlerts.length}
                </span>
              )}
            </TabsTrigger>
          </TabsList>

          <TabsContent value="actions">
            <ActionList
              actions={actions}
              onStatusChange={onActionStatusChange}
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
                      action={actions.find((a) => a.signalId === signal.id)}
                      onActionStatusChange={onActionStatusChange}
                    />
                  ))}
                </div>
              </div>
            )}
          </TabsContent>

          <TabsContent value="risks">
            {riskAlerts.length === 0 ? (
              <EmptyState
                size="compact"
                icon={<Warning size={32} />}
                title={t("riskAlerts.title")}
                description={t("empty.risks.description")}
              />
            ) : (
              <RiskAlertList alerts={riskAlerts} workspaceSlug={workspaceSlug} />
            )}
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  );
}
