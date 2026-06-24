import { useCallback, useEffect, useState } from "react";
import { Plug, CloudArrowUp, Database } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router";
import type { IntegrationStatus } from "@/types";

type Provider = "slack" | "hubspot" | "zapier";

export function SettingsIntegrationsPage() {
  const { t } = useTranslation("settings");
  const { t: tc } = useTranslation("common");
  const [searchParams, setSearchParams] = useSearchParams();

  const integrationsConfig = [
    { id: "slack" as const, name: "Slack", description: t("integrations.slackDescription"), icon: CloudArrowUp },
    { id: "hubspot" as const, name: "HubSpot", description: t("integrations.hubspotDescription"), icon: Database },
    { id: "zapier" as const, name: "Zapier", description: t("integrations.zapierDescription"), icon: Plug },
  ];

  const [status, setStatus] = useState<IntegrationStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [connecting, setConnecting] = useState<Provider | null>(null);

  const loadStatus = useCallback(async () => {
    try {
      const res = await api.getIntegrations();
      setStatus(res.data);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("integrations.updateFailed"));
    }
  }, [t]);

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

  useEffect(() => {
    const provider = searchParams.get("provider") as Provider | null;
    const result = searchParams.get("status");
    if (!provider || !result) return;

    if (result === "connected") {
      toast.success(t("integrations.connectedSuccess", { provider: provider.charAt(0).toUpperCase() + provider.slice(1) }));
      void (async () => {
        await loadStatus();
      })();
    } else if (result === "error") {
      const message = searchParams.get("message") || "";
      toast.error(t("integrations.connectionFailed", { provider, message }));
    }

    // Clean query params so a refresh does not re-trigger the toast.
    const next = new URLSearchParams(searchParams);
    next.delete("provider");
    next.delete("status");
    next.delete("message");
    setSearchParams(next, { replace: true });
  }, [searchParams, setSearchParams, t, loadStatus]);

  const connect = async (id: Provider) => {
    if (id === "zapier") {
      toast.info(t("integrations.comingSoon"));
      return;
    }
    setConnecting(id);
    try {
      const res = id === "slack" ? await api.connectSlack() : await api.connectHubSpot();
      window.open(res.url, "_blank", "noopener,noreferrer");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("integrations.connectionFailed", { provider: id }));
    } finally {
      setConnecting(null);
    }
  };

  const disconnect = async (id: Provider) => {
    if (id === "zapier") return;
    try {
      if (id === "slack") {
        await api.disconnectSlack();
      } else {
        await api.disconnectHubSpot();
      }
      toast.success(t("integrations.disconnectedSuccess", { provider: id }));
      await loadStatus();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("integrations.disconnectFailed", { provider: id }));
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
                    {connected && (
                      <span className="text-caption text-green-600">{t("integrations.connected")}</span>
                    )}
                    {connected ? (
                      <Button variant="outline" size="sm" onClick={() => disconnect(integration.id)}>
                        {t("integrations.disconnect")}
                      </Button>
                    ) : (
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={connecting === integration.id}
                        onClick={() => connect(integration.id)}
                      >
                        {connecting === integration.id
                          ? t("integrations.connecting")
                          : t("integrations.connect")}
                      </Button>
                    )}
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
