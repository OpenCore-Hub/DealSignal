import { useRef, useState } from "react";
import { Palette, Upload } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import { useAsyncData } from "@/hooks/useAsyncData";
import type { WorkspaceSettings } from "@/types";

const MAX_LOGO_SIZE = 5 * 1024 * 1024;

export function SettingsBrandPage() {
  const { t } = useTranslation("settings");
  const { t: tc } = useTranslation("common");
  const { data, loading, error, refetch } = useAsyncData(
    () => api.getWorkspaceSettings().then((res) => res.data),
    []
  );
  const [draft, setDraft] = useState<WorkspaceSettings | null>(null);
  const [saving, setSaving] = useState(false);
  const [uploading, setUploading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const previewRef = useRef<string | null>(null);

  const settings = draft ?? data ?? null;

  const handleSave = async () => {
    if (!settings) return;
    setSaving(true);
    try {
      const res = await api.updateWorkspaceSettings(settings);
      setDraft(res.data);
      toast.success(t("brand.saved"));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : tc("error.saveFailed"));
    } finally {
      setSaving(false);
    }
  };

  const cleanupPreview = () => {
    if (previewRef.current) {
      URL.revokeObjectURL(previewRef.current);
      previewRef.current = null;
    }
  };

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file || !settings) {
      if (fileInputRef.current) fileInputRef.current.value = "";
      return;
    }

    if (!file.type.startsWith("image/")) {
      toast.error(t("brand.invalidType"));
      if (fileInputRef.current) fileInputRef.current.value = "";
      return;
    }

    if (file.size > MAX_LOGO_SIZE) {
      toast.error(t("brand.tooLarge"));
      if (fileInputRef.current) fileInputRef.current.value = "";
      return;
    }

    const previousUrl = settings.logoUrl;
    const objectUrl = URL.createObjectURL(file);
    cleanupPreview();
    previewRef.current = objectUrl;
    setDraft((prev) => (prev ?? data ? { ...(prev ?? data!), logoUrl: objectUrl } : prev));
    setUploading(true);

    try {
      const res = await api.uploadWorkspaceLogo(file);
      cleanupPreview();
      setDraft((prev) => (prev ?? data ? { ...(prev ?? data!), logoUrl: res.data.logoUrl } : prev));
      toast.success(t("brand.uploadSuccess"));
    } catch (err) {
      cleanupPreview();
      setDraft((prev) => (prev ?? data ? { ...(prev ?? data!), logoUrl: previousUrl } : prev));
      toast.error(err instanceof Error ? err.message : t("brand.uploadFailed"));
    } finally {
      setUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = "";
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

  if (loading || !settings) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-48" />
      </div>
    );
  }

  const updateField = <K extends keyof WorkspaceSettings>(field: K, value: WorkspaceSettings[K]) => {
    setDraft((prev) => (prev ?? data ? { ...(prev ?? data!), [field]: value } : prev));
  };

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <Palette size={20} />
            {t("brand.title")}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label>{t("brand.logo")}</Label>
            <div className="flex items-center gap-4">
              {settings.logoUrl ? (
                <img
                  src={settings.logoUrl}
                  alt={t("brand.logo")}
                  className="h-24 w-24 rounded-md border border-border object-contain bg-background"
                />
              ) : (
                <div className="flex h-24 w-24 items-center justify-center rounded-md border border-dashed border-border bg-muted/50 text-center text-xs text-muted-foreground">
                  {t("brand.noLogo")}
                </div>
              )}
              <input
                ref={fileInputRef}
                type="file"
                accept="image/*"
                aria-label={t("brand.upload")}
                className="hidden"
                onChange={handleFileChange}
              />
              <Button
                type="button"
                variant="outline"
                className="gap-1.5"
                disabled={uploading}
                onClick={() => fileInputRef.current?.click()}
              >
                <Upload size={16} />
                {uploading ? t("brand.uploading") : t("brand.upload")}
              </Button>
            </div>
            <p className="text-caption text-muted-foreground">{t("brand.hint")}</p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="brand-color">{t("brand.brandColor")}</Label>
            <div className="flex items-center gap-2">
              <Input
                id="brand-color"
                value={settings.brandColor}
                onChange={(e) => updateField("brandColor", e.target.value)}
              />
              <span
                className="h-9 w-9 shrink-0 rounded-md border border-border"
                style={{ backgroundColor: settings.brandColor }}
                aria-hidden="true"
              />
            </div>
          </div>
          <div className="space-y-2">
            <Label htmlFor="viewer-domain">{t("brand.viewerDomain")}</Label>
            <Input
              id="viewer-domain"
              placeholder="invest.yourdomain.com"
              value={settings.viewerDomain}
              onChange={(e) => updateField("viewerDomain", e.target.value)}
            />
          </div>
          <Button onClick={handleSave} disabled={saving}>
            {saving ? t("brand.saving") : t("brand.save")}
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
