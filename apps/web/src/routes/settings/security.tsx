import { useEffect, useState } from "react";
import { Shield, Key, FileText } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { useTranslation } from "react-i18next";
import type { SecuritySettings } from "@/types";

export function SettingsSecurityPage() {
  const { t } = useTranslation("settings");
  const { t: tc } = useTranslation("common");
  const [settings, setSettings] = useState<SecuritySettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [retryKey, setRetryKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      setError(null);
      try {
        const res = await api.getSecuritySettings();
        if (!cancelled) setSettings(res.data);
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : tc("error.loadFailed"));
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [retryKey, tc]);

  const update = async (patch: Partial<SecuritySettings>) => {
    if (!settings) return;
    const next = { ...settings, ...patch };
    setSettings(next);
    try {
      const res = await api.updateSecuritySettings(next);
      setSettings(res.data);
    } catch (e) {
      setError(e instanceof Error ? e.message : tc("error.saveFailed"));
      setRetryKey((k) => k + 1);
    }
  };

  if (loading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-48" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle className="text-h2 flex items-center gap-2">
              <Shield size={20} />
              {t("security.title")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="rounded-lg border border-error-500/20 bg-error-100 p-4">
              <p className="text-sm font-medium text-error-500">{t("security.loadFailed")}</p>
              <p className="text-caption mt-1 text-error-500/80">{error}</p>
              <Button variant="outline" size="sm" className="mt-3" onClick={() => setRetryKey((k) => k + 1)}>
                {tc("retry")}
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (!settings) {
    return null;
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <Shield size={20} />
            {t("security.title")}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">{t("security.forceEmailVerification")}</p>
              <p className="text-caption text-muted-foreground">{t("security.forceEmailVerificationDescription")}</p>
            </div>
            <Switch
              checked={settings.forceEmailVerification}
              onCheckedChange={(checked) => update({ forceEmailVerification: checked })}
            />
          </div>
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">{t("security.watermarkDownloads")}</p>
              <p className="text-caption text-muted-foreground">{t("security.watermarkDownloadsDescription")}</p>
            </div>
            <Switch
              checked={settings.watermarkDownloads}
              onCheckedChange={(checked) => update({ watermarkDownloads: checked })}
            />
          </div>
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">{t("security.twoFactor")}</p>
              <p className="text-caption text-muted-foreground">{t("security.twoFactorDescription")}</p>
            </div>
            <Button variant="outline" className="gap-1.5" disabled={!settings.twoFactorEnabled}>
              <Key size={16} />
              {t("security.configure")}
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <FileText size={20} />
            {t("security.auditLog")}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-body text-muted-foreground">{t("security.auditLogDescription")}</p>
          <Button className="mt-4" disabled title={t("security.auditLogDisabled")}>
            {t("security.viewAuditLog")}
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
