import { useEffect, useState } from "react";
import { CreditCard, Crown } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { UsageBar } from "@/components/common/UsageBar";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import type { BillingInfo } from "@/types";

export function SettingsBillingPage() {
  const [billing, setBilling] = useState<BillingInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [retryKey, setRetryKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      setError(null);
      try {
        const res = await api.getBillingInfo();
        if (!cancelled) setBilling(res.data);
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : "加载失败");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [retryKey]);

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
              订阅与用量
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="rounded-lg border border-error-500/20 bg-error-100 p-4">
              <p className="text-sm font-medium text-error-500">加载订阅信息失败</p>
              <p className="text-caption mt-1 text-error-500/80">{error}</p>
              <Button variant="outline" size="sm" className="mt-3" onClick={() => setRetryKey((k) => k + 1)}>
                重试
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
            订阅与用量
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-center justify-between rounded-lg bg-muted p-4">
            <div>
              <p className="text-sm font-medium">当前方案</p>
              <p className="text-caption text-muted-foreground">
                {billing.plan} {billing.period}
              </p>
            </div>
            <Button variant="outline" className="gap-1.5" disabled title="升级订阅需后端支付支持">
              <Crown size={16} />
              升级
            </Button>
          </div>

          <div className="space-y-4">
            <UsageBar label="文档存储" current={billing.storageUsed} max={billing.storageLimit} unit="MB" />
            <UsageBar label="分享链接" current={billing.linksUsed} max={billing.linksLimit} />
            <UsageBar label="数据室" current={billing.roomsUsed} max={billing.roomsLimit} />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
