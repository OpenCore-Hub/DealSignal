import { useState } from "react";
import { Building, Globe } from "@phosphor-icons/react";
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

export function SettingsGeneralPage() {
  const { t } = useTranslation("settings");
  const { t: tc } = useTranslation("common");
  const { data, loading, error, refetch } = useAsyncData(
    () => api.getWorkspaceSettings().then((res) => res.data),
    []
  );
  const [draft, setDraft] = useState<WorkspaceSettings | null>(null);
  const [saving, setSaving] = useState(false);

  const settings = draft ?? data ?? null;

  const updateField = <K extends keyof WorkspaceSettings>(field: K, value: WorkspaceSettings[K]) => {
    setDraft((prev) => (prev ?? data ? { ...(prev ?? data!), [field]: value } : prev));
  };

  const handleSave = async () => {
    if (!settings) return;
    setSaving(true);
    try {
      const res = await api.updateWorkspaceSettings(settings);
      setDraft(res.data);
      toast.success(t("general.saved"));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : tc("error.saveFailed"));
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

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <Building size={20} />
            {t("general.title")}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="workspace-name">{t("general.name")}</Label>
            <Input
              id="workspace-name"
              value={settings.name}
              onChange={(e) => updateField("name", e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="workspace-slug">{t("general.slug")}</Label>
            <div className="flex items-center gap-2">
              <Globe size={16} className="text-muted-foreground" />
              <Input
                id="workspace-slug"
                value={settings.slug}
                onChange={(e) => updateField("slug", e.target.value)}
              />
            </div>
          </div>
          <Button onClick={handleSave} disabled={saving}>
            {saving ? t("general.saving") : t("general.save")}
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
