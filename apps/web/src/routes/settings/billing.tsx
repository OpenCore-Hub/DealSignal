import { CreditCard, Crown } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { UsageBar } from "@/components/common/UsageBar";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { useTranslation } from "react-i18next";
import { useAsyncData } from "@/hooks/useAsyncData";

export function SettingsBillingPage() {
  const { t } = useTranslation("settings");
  const { t: tc } = useTranslation("common");
  const { data: billing, loading, error, refetch } = useAsyncData(() => api.getBillingInfo(), []);

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
              <CreditCard size={20} />
              {t("billing.title")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="rounded-lg border border-error-500/20 bg-error-100 p-4">
              <p className="text-sm font-medium text-error-500">{t("billing.loadFailed")}</p>
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

  if (!billing) {
    return null;
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <CreditCard size={20} />
            {t("billing.title")}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-center justify-between rounded-lg bg-muted p-4">
            <div>
              <p className="text-sm font-medium">{t("billing.plan")}</p>
              <p className="text-caption text-muted-foreground">
                {billing.plan} {billing.period}
              </p>
            </div>
            <Button variant="outline" className="gap-1.5" disabled title={t("billing.upgradeDisabled")}>
              <Crown size={16} />
              {t("billing.upgrade")}
            </Button>
          </div>

          <div className="space-y-4">
            <UsageBar label={t("billing.storage")} current={billing.storageUsed} max={billing.storageLimit} unit="MB" />
            <UsageBar label={t("billing.links")} current={billing.linksUsed} max={billing.linksLimit} />
            <UsageBar label={t("billing.rooms")} current={billing.roomsUsed} max={billing.roomsLimit} />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
