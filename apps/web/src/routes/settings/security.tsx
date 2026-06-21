import { useEffect, useState } from "react";
import { Shield, Key, FileText } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import type { SecuritySettings } from "@/types";

export function SettingsSecurityPage() {
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

  const update = async (patch: Partial<SecuritySettings>) => {
    if (!settings) return;
    const next = { ...settings, ...patch };
    setSettings(next);
    try {
      const res = await api.updateSecuritySettings(next);
      setSettings(res.data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "保存失败");
      // 回滚到上一状态
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
              安全
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="rounded-lg border border-error-500/20 bg-error-100 p-4">
              <p className="text-sm font-medium text-error-500">加载安全设置失败</p>
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

  if (!settings) {
    return null;
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <Shield size={20} />
            安全
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">强制邮箱验证</p>
              <p className="text-caption text-muted-foreground">访问链接前必须验证邮箱</p>
            </div>
            <Switch
              checked={settings.forceEmailVerification}
              onCheckedChange={(checked) => update({ forceEmailVerification: checked })}
            />
          </div>
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">水印下载</p>
              <p className="text-caption text-muted-foreground">下载 PDF 时附加访客邮箱水印</p>
            </div>
            <Switch
              checked={settings.watermarkDownloads}
              onCheckedChange={(checked) => update({ watermarkDownloads: checked })}
            />
          </div>
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">双因素认证</p>
              <p className="text-caption text-muted-foreground">为管理员账号启用 2FA</p>
            </div>
            <Button variant="outline" className="gap-1.5" disabled={!settings.twoFactorEnabled}>
              <Key size={16} />
              配置
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <FileText size={20} />
            审计日志
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-body text-muted-foreground">查看最近 30 天的工作区关键操作记录。</p>
          <Button className="mt-4" disabled title="审计日志需后端支持">
            查看审计日志
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
