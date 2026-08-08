import { useEffect, useState } from "react";
import { apiErrorMessage } from "@/lib/apiErrors";
import { Plug, CloudArrowUp, ChartLineUp, Database, Envelope, FileMagnifyingGlass } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router";
import { useAsyncData } from "@/hooks/useAsyncData";
import type { OutboundWebhookConfig } from "@/types";

type Provider = "slack" | "hubspot";

export function SettingsIntegrationsPage() {
  const { t } = useTranslation("settings");
  const { t: tc } = useTranslation("common");
  const [searchParams, setSearchParams] = useSearchParams();

  const integrationsConfig = [
    { id: "slack" as const, name: "Slack", description: t("integrations.slackDescription"), icon: CloudArrowUp },
    { id: "hubspot" as const, name: "HubSpot", description: t("integrations.hubspotDescription"), icon: Database },
  ];

  const toggleEmailNotifications = async () => {
    if (!status) return;
    setSavingEmail(true);
    try {
      await api.updateIntegrations({ ...status, emailEnabled: !status.emailEnabled });
      toast.success(t("integrations.emailNotificationsSaved"));
      refetch();
    } catch (e) {
      toast.error(apiErrorMessage(e, { messageKey: "settings:integrations.emailNotificationsSaveFailed" }));
    } finally {
      setSavingEmail(false);
    }
  };

  const toggleDailyDigest = async () => {
    if (!status) return;
    setSavingDigest(true);
    try {
      await api.updateIntegrations({
        ...status,
        dailyDigestEnabled: !status.dailyDigestEnabled,
      });
      toast.success(t("integrations.dailyDigestSaved"));
      refetch();
    } catch (e) {
      toast.error(apiErrorMessage(e, { messageKey: "settings:integrations.dailyDigestSaveFailed" }));
    } finally {
      setSavingDigest(false);
    }
  };

  const toggleKeyPageSlack = async () => {
    if (!status) return;
    if (!status.slack && !status.keyPageSlackEnabled) {
      toast.error(t("integrations.keyPageSlackNeedsSlack"));
      return;
    }
    setSavingKeyPageSlack(true);
    try {
      await api.updateIntegrations({
        ...status,
        keyPageSlackEnabled: !status.keyPageSlackEnabled,
      });
      toast.success(t("integrations.keyPageSlackSaved"));
      refetch();
    } catch (e) {
      toast.error(apiErrorMessage(e, { messageKey: "settings:integrations.keyPageSlackSaveFailed" }));
    } finally {
      setSavingKeyPageSlack(false);
    }
  };

  const { data: status, loading, error, refetch } = useAsyncData(() => api.getIntegrations(), []);
  const {
    data: webhook,
    loading: webhookLoading,
    error: webhookError,
    refetch: refetchWebhook,
  } = useAsyncData(() => api.getOutboundWebhook(), []);
  const [connecting, setConnecting] = useState<Provider | null>(null);
  const [savingEmail, setSavingEmail] = useState(false);
  const [savingDigest, setSavingDigest] = useState(false);
  const [savingKeyPageSlack, setSavingKeyPageSlack] = useState(false);
  const [webhookURL, setWebhookURL] = useState("");
  const [webhookEnabled, setWebhookEnabled] = useState(false);
  const [savingWebhook, setSavingWebhook] = useState(false);
  const [revealedSecret, setRevealedSecret] = useState<string | null>(null);

  useEffect(() => {
    if (!webhook) return;
    setWebhookURL(webhook.url ?? "");
    setWebhookEnabled(webhook.enabled);
  }, [webhook]);

  useEffect(() => {
    const provider = searchParams.get("provider") as Provider | null;
    const result = searchParams.get("status");
    if (!provider || !result) return;

    if (result === "connected") {
      toast.success(t("integrations.connectedSuccess", { provider: provider.charAt(0).toUpperCase() + provider.slice(1) }));
      refetch();
    } else if (result === "error") {
      toast.error(t("integrations.connectionFailed", {
        provider: provider.charAt(0).toUpperCase() + provider.slice(1),
      }));
    }

    // Clean query params so a refresh does not re-trigger the toast.
    const next = new URLSearchParams(searchParams);
    next.delete("provider");
    next.delete("status");
    next.delete("message");
    setSearchParams(next, { replace: true });
  }, [searchParams, setSearchParams, t, refetch]);

  const connect = async (id: Provider) => {
    setConnecting(id);
    try {
      const res = id === "slack" ? await api.connectSlack() : await api.connectHubSpot();
      window.open(res.url, "_blank", "noopener,noreferrer");
    } catch (e) {
      toast.error(apiErrorMessage(e, { messageKey: "settings:integrations.connectionFailed", messageKeyParams: { provider: id } }));
    } finally {
      setConnecting(null);
    }
  };

  const disconnect = async (id: Provider) => {
    try {
      if (id === "slack") {
        await api.disconnectSlack();
      } else {
        await api.disconnectHubSpot();
      }
      toast.success(t("integrations.disconnectedSuccess", { provider: id }));
      refetch();
    } catch (e) {
      toast.error(apiErrorMessage(e, { messageKey: "settings:integrations.disconnectFailed", messageKeyParams: { provider: id } }));
    }
  };

  const applyWebhookResult = (cfg: OutboundWebhookConfig) => {
    setWebhookURL(cfg.url ?? "");
    setWebhookEnabled(cfg.enabled);
    if (cfg.secret) {
      setRevealedSecret(cfg.secret);
    }
  };

  const saveWebhook = async (rotateSecret = false) => {
    setSavingWebhook(true);
    try {
      const cfg = await api.saveOutboundWebhook({
        url: webhookURL.trim(),
        enabled: webhookEnabled,
        eventTypes: ["key_page", "repeat_key_page"],
        rotateSecret,
      });
      applyWebhookResult(cfg);
      toast.success(
        rotateSecret
          ? t("integrations.webhookSecretRotated")
          : t("integrations.webhookSaved"),
      );
      refetchWebhook();
    } catch (e) {
      toast.error(apiErrorMessage(e, { messageKey: "settings:integrations.webhookSaveFailed" }));
    } finally {
      setSavingWebhook(false);
    }
  };

  const deleteWebhook = async () => {
    setSavingWebhook(true);
    try {
      await api.deleteOutboundWebhook();
      setWebhookURL("");
      setWebhookEnabled(false);
      setRevealedSecret(null);
      toast.success(t("integrations.webhookDeleted"));
      refetchWebhook();
    } catch (e) {
      toast.error(apiErrorMessage(e, { messageKey: "settings:integrations.webhookDeleteFailed" }));
    } finally {
      setSavingWebhook(false);
    }
  };

  const copySecret = async () => {
    if (!revealedSecret) return;
    try {
      await navigator.clipboard.writeText(revealedSecret);
      toast.success(t("integrations.webhookSecretCopied"));
    } catch {
      toast.error(t("integrations.webhookSecretCopyFailed"));
    }
  };

  if (error) {
    return (
      <div className="space-y-6">
        <Card>
          <CardContent className="flex flex-col items-center justify-center gap-4 p-12 text-center">
            <p className="text-body text-muted-foreground">{error}</p>
            <Button onClick={refetch}>{tc("retry")}</Button>
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
            <li className="flex items-center justify-between py-4">
              <div className="flex items-center gap-3">
                <div className="flex h-10 w-10 items-center justify-center rounded-md bg-muted">
                  <Envelope size={20} className="text-muted-foreground" />
                </div>
                <div>
                  <p className="text-sm font-medium">{t("integrations.emailNotifications")}</p>
                  <p className="text-caption text-muted-foreground">{t("integrations.emailNotificationsDescription")}</p>
                </div>
              </div>
              <Switch
                checked={status?.emailEnabled ?? true}
                disabled={savingEmail || loading}
                onCheckedChange={toggleEmailNotifications}
                aria-label={t("integrations.emailNotifications")}
              />
            </li>
            <li className="flex items-center justify-between py-4">
              <div className="flex items-center gap-3">
                <div className="flex h-10 w-10 items-center justify-center rounded-md bg-muted">
                  <ChartLineUp size={20} className="text-muted-foreground" />
                </div>
                <div>
                  <p className="text-sm font-medium">{t("integrations.dailyDigest")}</p>
                  <p className="text-caption text-muted-foreground">
                    {t("integrations.dailyDigestDescription")}
                  </p>
                </div>
              </div>
              <Switch
                checked={status?.dailyDigestEnabled ?? false}
                disabled={savingDigest || loading}
                onCheckedChange={toggleDailyDigest}
                aria-label={t("integrations.dailyDigest")}
              />
            </li>
            <li className="flex items-center justify-between py-4">
              <div className="flex items-center gap-3">
                <div className="flex h-10 w-10 items-center justify-center rounded-md bg-muted">
                  <FileMagnifyingGlass size={20} className="text-muted-foreground" />
                </div>
                <div>
                  <p className="text-sm font-medium">{t("integrations.keyPageSlack")}</p>
                  <p className="text-caption text-muted-foreground">
                    {t("integrations.keyPageSlackDescription")}
                  </p>
                </div>
              </div>
              <Switch
                checked={status?.keyPageSlackEnabled ?? false}
                disabled={savingKeyPageSlack || loading || (!status.slack && !status.keyPageSlackEnabled)}
                onCheckedChange={toggleKeyPageSlack}
                aria-label={t("integrations.keyPageSlack")}
              />
            </li>
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

      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <Plug size={20} />
            {t("integrations.webhookTitle")}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-caption text-muted-foreground">{t("integrations.webhookDescription")}</p>
          {webhookError ? (
            <div className="flex flex-col items-start gap-2">
              <p className="text-body text-muted-foreground">{webhookError}</p>
              <Button variant="outline" size="sm" onClick={refetchWebhook}>{tc("retry")}</Button>
            </div>
          ) : webhookLoading && !webhook ? (
            <Skeleton className="h-24 w-full" />
          ) : (
            <>
              <div className="space-y-2">
                <label className="text-sm font-medium" htmlFor="outbound-webhook-url">
                  {t("integrations.webhookURL")}
                </label>
                <Input
                  id="outbound-webhook-url"
                  type="url"
                  value={webhookURL}
                  onChange={(e) => setWebhookURL(e.target.value)}
                  placeholder={t("integrations.webhookURLPlaceholder")}
                  disabled={savingWebhook}
                />
              </div>
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium">{t("integrations.webhookEnabled")}</p>
                  <p className="text-caption text-muted-foreground">{t("integrations.webhookEnabledDescription")}</p>
                </div>
                <Switch
                  checked={webhookEnabled}
                  disabled={savingWebhook}
                  onCheckedChange={setWebhookEnabled}
                  aria-label={t("integrations.webhookEnabled")}
                />
              </div>
              {(revealedSecret || webhook?.secretHint) && (
                <div className="rounded-md border border-border bg-muted/40 p-3 space-y-2">
                  <p className="text-sm font-medium">{t("integrations.webhookSecret")}</p>
                  {revealedSecret ? (
                    <>
                      <p className="text-caption text-muted-foreground">{t("integrations.webhookSecretRevealOnce")}</p>
                      <code className="block break-all text-caption font-mono">{revealedSecret}</code>
                      <Button variant="outline" size="sm" onClick={copySecret}>
                        {t("integrations.webhookCopySecret")}
                      </Button>
                    </>
                  ) : (
                    <p className="text-caption text-muted-foreground">
                      {t("integrations.webhookSecretHint", { hint: webhook?.secretHint })}
                    </p>
                  )}
                </div>
              )}
              <div className="flex flex-wrap gap-2">
                <Button
                  size="sm"
                  disabled={savingWebhook || !webhookURL.trim()}
                  onClick={() => saveWebhook(false)}
                >
                  {savingWebhook ? t("integrations.webhookSaving") : t("integrations.webhookSave")}
                </Button>
                {webhook?.configured && (
                  <>
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={savingWebhook}
                      onClick={() => saveWebhook(true)}
                    >
                      {t("integrations.webhookRotateSecret")}
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={savingWebhook}
                      onClick={deleteWebhook}
                    >
                      {t("integrations.webhookDelete")}
                    </Button>
                  </>
                )}
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
