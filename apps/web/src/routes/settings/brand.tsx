import { useRef, useState } from "react";
import { apiErrorMessage } from "@/lib/apiErrors";
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
import { cn } from "@/lib/utils";
import type { WorkspaceSettings, WorkspaceViewerDomain } from "@/types";

const MAX_LOGO_SIZE = 5 * 1024 * 1024;
const LOGO_ACCEPT = "image/*,.svg,image/svg+xml";

export function isAllowedLogoFile(file: File): boolean {
  if (file.type.startsWith("image/")) return true;
  return /\.(png|jpe?g|webp|gif|svg)$/i.test(file.name);
}
/** Same default as CreateWorkspacePage — native `type=color` requires #rrggbb. */
const DEFAULT_BRAND_COLOR = "#0055ff";

export function toColorInputValue(raw: string | undefined | null): string {
  const value = (raw ?? "").trim();
  const hex = value.startsWith("#") ? value : value ? `#${value}` : "";
  if (/^#[0-9a-fA-F]{6}$/.test(hex)) return hex.toLowerCase();
  if (/^#[0-9a-fA-F]{3}$/.test(hex)) {
    const [, r, g, b] = hex;
    return `#${r}${r}${g}${g}${b}${b}`.toLowerCase();
  }
  return DEFAULT_BRAND_COLOR;
}

export function SettingsBrandPage() {
  const { t } = useTranslation("settings");
  const { t: tc } = useTranslation("common");
  const { data, loading, error, refetch } = useAsyncData(async () => {
    const [settings, viewerDomain] = await Promise.all([
      api.getWorkspaceSettings(),
      api.getWorkspaceViewerDomain(),
    ]);
    return { settings, viewerDomain };
  }, []);
  const [draft, setDraft] = useState<WorkspaceSettings | null>(null);
  const [viewerDomain, setViewerDomain] = useState<WorkspaceViewerDomain | null>(null);
  const [viewerDraftHost, setViewerDraftHost] = useState("");
  const [saving, setSaving] = useState(false);
  const [domainBusy, setDomainBusy] = useState<"add" | "verify" | "remove" | null>(null);
  const [uploading, setUploading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const previewRef = useRef<string | null>(null);

  const settings = draft ?? data?.settings ?? null;
  const domain = viewerDomain ?? data?.viewerDomain ?? null;

  const handleSave = async () => {
    if (!settings) return;
    setSaving(true);
    try {
      const res = await api.updateWorkspaceSettings(settings);
      setDraft(res);
      toast.success(t("brand.saved"));
    } catch (e) {
      toast.error(apiErrorMessage(e, { fallback: "saveFailed" }));
    } finally {
      setSaving(false);
    }
  };

  const handleAddDomain = async () => {
    const hostname = viewerDraftHost.trim();
    if (!hostname) return;
    setDomainBusy("add");
    try {
      const res = await api.putWorkspaceViewerDomain(hostname);
      setViewerDomain(res);
      setViewerDraftHost("");
      toast.success(t("brand.viewerDomainAdded"));
    } catch (e) {
      toast.error(apiErrorMessage(e, { fallback: "saveFailed" }));
    } finally {
      setDomainBusy(null);
    }
  };

  const handleVerifyDomain = async () => {
    setDomainBusy("verify");
    try {
      const res = await api.verifyWorkspaceViewerDomain();
      setViewerDomain(res);
      setDraft((prev) =>
        prev ?? data?.settings
          ? { ...(prev ?? data!.settings), viewerDomain: res.hostname }
          : prev,
      );
      toast.success(t("brand.viewerDomainVerifiedSuccess"));
    } catch (e) {
      toast.error(apiErrorMessage(e, { fallback: "saveFailed" }));
    } finally {
      setDomainBusy(null);
    }
  };

  const handleRemoveDomain = async () => {
    setDomainBusy("remove");
    try {
      await api.deleteWorkspaceViewerDomain();
      setViewerDomain({
        hostname: "",
        status: "",
        cnameHost: "",
        cnameTarget: domain?.cnameTarget ?? "",
      });
      setDraft((prev) =>
        prev ?? data?.settings ? { ...(prev ?? data!.settings), viewerDomain: "" } : prev,
      );
      toast.success(t("brand.viewerDomainRemoved"));
    } catch (e) {
      toast.error(apiErrorMessage(e, { fallback: "deleteFailed" }));
    } finally {
      setDomainBusy(null);
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

    if (!isAllowedLogoFile(file)) {
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
    setDraft((prev) =>
      prev ?? data?.settings ? { ...(prev ?? data!.settings), logoUrl: objectUrl } : prev,
    );
    setUploading(true);

    try {
      const res = await api.uploadWorkspaceLogo(file);
      cleanupPreview();
      setDraft((prev) =>
        prev ?? data?.settings ? { ...(prev ?? data!.settings), logoUrl: res.logoUrl } : prev,
      );
      toast.success(t("brand.uploadSuccess"));
    } catch (err) {
      cleanupPreview();
      setDraft((prev) =>
        prev ?? data?.settings ? { ...(prev ?? data!.settings), logoUrl: previousUrl } : prev,
      );
      toast.error(apiErrorMessage(err, { fallback: "uploadFailed", messageKey: "settings:brand.uploadFailed" }));
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
    setDraft((prev) =>
      prev ?? data?.settings ? { ...(prev ?? data!.settings), [field]: value } : prev,
    );
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
            <button
              type="button"
              disabled={uploading}
              aria-label={uploading ? t("brand.uploading") : t("brand.upload")}
              onClick={() => fileInputRef.current?.click()}
              className={cn(
                "group relative flex h-24 w-24 items-center justify-center overflow-hidden rounded-md border bg-muted/50 text-center text-xs text-muted-foreground transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-60",
                settings.logoUrl ? "border-border" : "border-dashed border-border",
              )}
            >
              {settings.logoUrl ? (
                <>
                  <img
                    src={settings.logoUrl}
                    alt={t("brand.logo")}
                    className="h-full w-full bg-background object-contain"
                  />
                  <span className="absolute inset-0 flex items-center justify-center bg-foreground/45 text-background opacity-0 transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100">
                    <Upload size={20} />
                  </span>
                </>
              ) : (
                <span className="flex flex-col items-center gap-1 px-1">
                  <Upload size={18} />
                  {uploading ? t("brand.uploading") : t("brand.noLogo")}
                </span>
              )}
            </button>
            <input
              ref={fileInputRef}
              type="file"
              accept={LOGO_ACCEPT}
              data-testid="brand-logo-input"
              className="sr-only"
              tabIndex={-1}
              onChange={handleFileChange}
            />
            <p className="text-caption text-muted-foreground">{t("brand.hint")}</p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="brandColor">{t("brand.brandColor")}</Label>
            <Input
              id="brandColor"
              type="color"
              value={toColorInputValue(settings.brandColor)}
              onChange={(e) => updateField("brandColor", e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="viewer-domain">{t("brand.viewerDomain")}</Label>
            {!domain?.status ? (
              <div className="flex flex-col gap-2 sm:flex-row">
                <Input
                  id="viewer-domain"
                  placeholder={t("brand.viewerDomainPlaceholder")}
                  value={viewerDraftHost}
                  onChange={(e) => setViewerDraftHost(e.target.value)}
                />
                <Button
                  type="button"
                  variant="secondary"
                  disabled={domainBusy !== null || !viewerDraftHost.trim()}
                  onClick={() => void handleAddDomain()}
                >
                  {domainBusy === "add" ? t("brand.viewerDomainAdding") : t("brand.viewerDomainAdd")}
                </Button>
              </div>
            ) : (
              <div className="space-y-2">
                <div className="flex flex-wrap items-center gap-2">
                  <Input id="viewer-domain" value={domain.hostname} readOnly />
                  <span className="text-caption text-muted-foreground">
                    {domain.status === "verified"
                      ? t("brand.viewerDomainVerified")
                      : t("brand.viewerDomainPending")}
                  </span>
                </div>
                {domain.status === "pending" && domain.cnameTarget ? (
                  <p className="text-caption text-muted-foreground">
                    {t("brand.viewerDomainCname", {
                      host: domain.cnameHost || domain.hostname,
                      target: domain.cnameTarget,
                    })}
                  </p>
                ) : null}
                <div className="flex flex-wrap gap-2">
                  {domain.status === "pending" ? (
                    <Button
                      type="button"
                      variant="secondary"
                      disabled={domainBusy !== null}
                      onClick={() => void handleVerifyDomain()}
                    >
                      {domainBusy === "verify"
                        ? t("brand.viewerDomainVerifying")
                        : t("brand.viewerDomainVerify")}
                    </Button>
                  ) : null}
                  <Button
                    type="button"
                    variant="outline"
                    disabled={domainBusy !== null}
                    onClick={() => void handleRemoveDomain()}
                  >
                    {domainBusy === "remove"
                      ? t("brand.viewerDomainRemoving")
                      : t("brand.viewerDomainRemove")}
                  </Button>
                </div>
              </div>
            )}
            <p className="text-caption text-muted-foreground">{t("brand.viewerDomainHint")}</p>
          </div>
          <Button onClick={handleSave} disabled={saving}>
            {saving ? t("brand.saving") : t("brand.save")}
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
