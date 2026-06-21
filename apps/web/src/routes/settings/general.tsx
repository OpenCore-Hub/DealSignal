import { useEffect, useState } from "react";
import { Building, Globe } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { toast } from "sonner";
import type { WorkspaceSettings } from "@/types";

export function SettingsGeneralPage() {
  const [settings, setSettings] = useState<WorkspaceSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    async function load() {
      try {
        setLoading(true);
        setError(null);
        const res = await api.getWorkspaceSettings();
        setSettings(res.data);
      } catch (e) {
        setError(e instanceof Error ? e.message : "加载失败");
      } finally {
        setLoading(false);
      }
    }
    load();
  }, []);

  const handleSave = async () => {
    if (!settings) return;
    setSaving(true);
    try {
      const res = await api.updateWorkspaceSettings(settings);
      setSettings(res.data);
      toast.success("工作区信息已保存");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "保存失败");
    } finally {
      setSaving(false);
    }
  };

  if (error) {
    return (
      <div className="space-y-6">
        <Card>
          <CardContent className="flex flex-col items-center justify-center gap-4 p-12 text-center">
            <p className="text-body text-muted-foreground">{error}</p>
            <Button onClick={() => window.location.reload()}>重试</Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (loading || !settings) {
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
            <Building size={20} />
            工作区信息
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="workspace-name">工作区名称</Label>
            <Input
              id="workspace-name"
              value={settings.name}
              onChange={(e) => setSettings((s) => (s ? { ...s, name: e.target.value } : s))}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="workspace-slug">Slug</Label>
            <div className="flex items-center gap-2">
              <Globe size={16} className="text-muted-foreground" />
              <Input
                id="workspace-slug"
                value={settings.slug}
                onChange={(e) => setSettings((s) => (s ? { ...s, slug: e.target.value } : s))}
              />
            </div>
          </div>
          <Button onClick={handleSave} disabled={saving}>
            {saving ? "保存中..." : "保存更改"}
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
