import { useState } from "react";
import { Shield, Key, FileText } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { useTranslation } from "react-i18next";
import { useAsyncData } from "@/hooks/useAsyncData";
import type { SecuritySettings } from "@/types";

export function SettingsSecurityPage() {
  const { t } = useTranslation("settings");
  const { t: tc } = useTranslation("common");
  const { data, loading, error, refetch } = useAsyncData(
    () => api.getSecuritySettings().then((res) => res.data),
    []
  );
  const [draft, setDraft] = useState<SecuritySettings | null>(null);

  const settings = draft ?? data ?? null;

  const update = async (patch: Partial<SecuritySettings>) => {
    if (!settings) return;
    const next = { ...settings, ...patch };
    setDraft(next);
    try {
      const res = await api.updateSecuritySettings(next);
      setDraft(res.data);
    } catch {
      setDraft(null);
      refetch();
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
              <Button variant="outline" size="sm" className="mt-3" onClick={refetch}>
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
