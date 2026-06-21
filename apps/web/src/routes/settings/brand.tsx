import { useEffect, useRef, useState } from "react";
import { Palette, Upload } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { toast } from "sonner";
import type { WorkspaceSettings } from "@/types";

export function SettingsBrandPage() {
  const [settings, setSettings] = useState<WorkspaceSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [uploading, setUploading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const previewRef = useRef<string | null>(null);

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
      toast.success("品牌设置已保存");
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
            <Palette size={20} />
            品牌定制
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label>Logo</Label>
            <div className="flex items-center gap-4">
              {settings.logoUrl ? (
                <img
                  src={settings.logoUrl}
                  alt="Workspace logo"
                  className="h-24 w-24 rounded-md border border-border object-contain bg-background"
                />
              ) : (
                <div className="flex h-24 w-24 items-center justify-center rounded-md border border-dashed border-border bg-muted/50 text-center text-xs text-muted-foreground">
                  未上传
                </div>
              )}
              <input
                ref={fileInputRef}
                type="file"
                accept="image/*"
                className="hidden"
                onChange={async (e) => {
                  const file = e.target.files?.[0];
                  if (!file) return;
                  const objectUrl = URL.createObjectURL(file);
                  if (previewRef.current) URL.revokeObjectURL(previewRef.current);
                  previewRef.current = objectUrl;
                  setSettings((s) => (s ? { ...s, logoUrl: objectUrl } : s));

                  setUploading(true);
                  try {
                    const res = await api.uploadWorkspaceLogo(file);
                    setSettings((s) => (s ? { ...s, logoUrl: res.data.logoUrl } : s));
                    if (previewRef.current === objectUrl) {
                      URL.revokeObjectURL(previewRef.current);
                      previewRef.current = null;
                    }
                    toast.success("Logo 上传成功");
                  } catch (err) {
                    toast.error(err instanceof Error ? err.message : "Logo 上传失败");
                  } finally {
                    setUploading(false);
                  }
                }}
              />
              <Button
                type="button"
                variant="outline"
                className="gap-1.5"
                disabled={uploading}
                onClick={() => fileInputRef.current?.click()}
              >
                <Upload size={16} />
                {uploading ? "上传中..." : "上传 Logo"}
              </Button>
            </div>
            <p className="text-caption text-muted-foreground">上传的 Logo 会先保存到文件存储，随后写入品牌设置。</p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="brand-color">主色</Label>
            <div className="flex items-center gap-2">
              <Input
                id="brand-color"
                value={settings.brandColor}
                onChange={(e) => setSettings((s) => (s ? { ...s, brandColor: e.target.value } : s))}
              />
              <span
                className="h-9 w-9 shrink-0 rounded-md border border-border"
                style={{ backgroundColor: settings.brandColor }}
                aria-hidden="true"
              />
            </div>
          </div>
          <div className="space-y-2">
            <Label htmlFor="viewer-domain">自定义访问域名</Label>
            <Input
              id="viewer-domain"
              placeholder="invest.yourdomain.com"
              value={settings.viewerDomain}
              onChange={(e) => setSettings((s) => (s ? { ...s, viewerDomain: e.target.value } : s))}
            />
          </div>
          <Button onClick={handleSave} disabled={saving}>
            {saving ? "保存中..." : "保存品牌设置"}
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
