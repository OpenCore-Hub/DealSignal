import { useState } from "react";
import { Link, useParams } from "react-router";
import { Shield, Key, FileText } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button, buttonVariants } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { apiErrorMessage } from "@/lib/apiErrors";
import { useTranslation } from "react-i18next";
import { useAsyncData } from "@/hooks/useAsyncData";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import type { BillingInfo, SecuritySettings } from "@/types";

export function SettingsSecurityPage() {
  const { t } = useTranslation("settings");
  const { t: tc } = useTranslation("common");
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const { data, loading, error, refetch } = useAsyncData(async () => {
    const [settings, billing] = await Promise.all([
      api.getSecuritySettings(),
      api.getBillingInfo(),
    ]);
    return { settings, billing };
  }, []);
  const [draft, setDraft] = useState<SecuritySettings | null>(null);

  const settings = draft ?? data?.settings ?? null;
  const billing: BillingInfo | null = data?.billing ?? null;
  // Grandfather: already-on stays toggable off; only new enable is plan-gated.
  const watermarkEnableBlocked =
    Boolean(settings) &&
    !settings!.watermarkDownloads &&
    billing != null &&
    !billing.watermarkEnabled;
  const accessControlsEnableBlocked =
    Boolean(settings) &&
    !settings!.forceEmailVerification &&
    billing != null &&
    billing.accessControlsEnabled === false;

  const update = async (patch: Partial<SecuritySettings>) => {
    if (!settings) return;
    if (patch.watermarkDownloads === true && watermarkEnableBlocked) {
      return;
    }
    if (patch.forceEmailVerification === true && accessControlsEnableBlocked) {
      return;
    }
    const next = { ...settings, ...patch };
    setDraft(next);
    try {
      const res = await api.updateSecuritySettings(next);
      setDraft(res);
    } catch (e) {
      setDraft(null);
      refetch();
      // Server remains authoritative (stale client billing / race past gate).
      toast.error(apiErrorMessage(e, { fallback: "saveFailed" }));
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
              <p className="text-caption text-muted-foreground">
                {accessControlsEnableBlocked
                  ? t("security.accessControlsPlanRequired")
                  : t("security.forceEmailVerificationDescription")}
              </p>
            </div>
            <Switch
              checked={settings.forceEmailVerification}
              disabled={accessControlsEnableBlocked}
              onCheckedChange={(checked) => update({ forceEmailVerification: checked })}
            />
          </div>
          <div className="flex items-center justify-between gap-4">
            <div>
              <p className="text-sm font-medium">{t("security.watermarkDownloads")}</p>
              <p className="text-caption text-muted-foreground">
                {watermarkEnableBlocked
                  ? t("security.watermarkPlanRequired")
                  : t("security.watermarkDownloadsDescription")}
              </p>
            </div>
            <Switch
              checked={settings.watermarkDownloads}
              disabled={watermarkEnableBlocked}
              onCheckedChange={(checked) => update({ watermarkDownloads: checked })}
            />
          </div>
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">{t("security.twoFactor")}</p>
              <p className="text-caption text-muted-foreground">{t("security.twoFactorDescription")}</p>
            </div>
            <Button
              variant="outline"
              className="gap-1.5"
              disabled={!settings.twoFactorEnabled}
              title={t("security.twoFactorDisabled")}
            >
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
          <Link
            to={`/${workspaceSlug}/insights/access`}
            className={cn(buttonVariants(), "mt-4 inline-flex")}
          >
            {t("security.viewAuditLog")}
          </Link>
        </CardContent>
      </Card>
    </div>
  );
}
