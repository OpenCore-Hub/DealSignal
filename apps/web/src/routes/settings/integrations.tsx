import { useEffect, useState } from "react";
import { Plug, CloudArrowUp, Database } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import type { IntegrationStatus } from "@/types";

export function SettingsIntegrationsPage() {
  const { t } = useTranslation("settings");
  const { t: tc } = useTranslation("common");
  const integrationsConfig = [
    { id: "slack" as const, name: "Slack", description: t("integrations.slackDescription"), icon: CloudArrowUp },
    { id: "hubspot" as const, name: "HubSpot", description: t("integrations.hubspotDescription"), icon: Database },
    { id: "zapier" as const, name: "Zapier", description: t("integrations.zapierDescription"), icon: Plug },
  ];

  const [status, setStatus] = useState<IntegrationStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function load() {
      try {
        setLoading(true);
        setError(null);
        const res = await api.getIntegrations();
        setStatus(res.data);
      } catch (e) {
        setError(e instanceof Error ? e.message : tc("error.loadFailed"));
      } finally {
        setLoading(false);
      }
    }
    load();
  }, [tc, t]);

  const toggle = async (id: keyof IntegrationStatus) => {
    if (!status) return;
    const previous = status;
    const next = { ...status, [id]: !status[id] };
    setStatus(next);
    try {
      const res = await api.updateIntegrations(next);
      setStatus(res.data);
      const name = integrationsConfig.find((c) => c.id === id)?.name ?? id;
      const state = res.data[id] ? t("integrations.connected") : t("integrations.disconnected");
      toast.success(`${name} ${state}`);
    } catch (e) {
      setStatus(previous);
      toast.error(e instanceof Error ? e.message : t("integrations.updateFailed"));
    }
  };

  if (error) {
    return (
      <div className="space-y-6">
        <Card>
          <CardContent className="flex flex-col items-center justify-center gap-4 p-12 text-center">
            <p className="text-body text-muted-foreground">{error}</p>
            <Button onClick={() => window.location.reload()}>{tc("retry")}</Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (loading || !status) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-48" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <Plug size={20} />
            {t("integrations.title")}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <ul className="divide-y divide-border">
            {integrationsConfig.map((integration) => {
              const connected = status[integration.id];
              return (
                <li key={integration.id} className="flex items-center justify-between py-4">
                  <div className="flex items-center gap-3">
                    <div className="flex h-10 w-10 items-center justify-center rounded-md bg-muted">
                      <integration.icon size={20} className="text-muted-foreground" />
                    </div>
                    <div>
                      <p className="text-sm font-medium">{integration.name}</p>
                      <p className="text-caption text-muted-foreground">{integration.description}</p>
                    </div>
                  </div>
                  <div className="flex items-center gap-3">
                    <Switch checked={connected} onCheckedChange={() => toggle(integration.id)} />
                    <Button variant="outline" size="sm" disabled={!connected}>
                      {connected ? t("integrations.configure") : t("integrations.connect")}
                    </Button>
                  </div>
                </li>
              );
            })}
          </ul>
        </CardContent>
      </Card>
    </div>
  );
}
