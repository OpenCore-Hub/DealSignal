import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { Link } from "@/types";
import { AccessSummaryCard } from "./AccessSummaryCard";
import { CopyButton } from "./CopyButton";
import { PRESETS, PRESET_NAMES } from "./presets";
import { getPublicUrl } from "./utils";
import type { DraftLink, LinkPreset } from "./types";

interface ShareTabProps {
  draft: DraftLink;
  updateDraft: (patch: Partial<DraftLink>) => void;
  preset: LinkPreset;
  setPreset: (preset: LinkPreset) => void;
  link: Link | null;
  onEditAccess: () => void;
  errors: Record<string, string>;
  slug?: string;
}

export function ShareTab({
  draft,
  updateDraft,
  preset,
  setPreset,
  link,
  onEditAccess,
  errors,
  slug,
}: ShareTabProps) {
  const { t } = useTranslation("linkShare");

  const publicUrl = getPublicUrl(link);

  const handlePresetChange = (value: string | null) => {
    if (!value) return;
    setPreset(value as LinkPreset);
  };

  const expiresEnabled = Boolean(draft.expiresAt);

  return (
    <div className="space-y-6 py-2">
      {link ? (
        <div className="space-y-3">
          <div className="space-y-2">
            <Label>{t("share.linkName")}</Label>
            <Input
              value={draft.name}
              onChange={(e) => updateDraft({ name: e.target.value })}
              placeholder={t("share.linkNamePlaceholder")}
            />
          </div>
          <div className="space-y-2">
            <Label>{t("share.publicLink")}</Label>
            <div className="flex items-center gap-2">
              <Input value={publicUrl} readOnly className="flex-1" />
              <CopyButton
                value={publicUrl}
                label={t("share.copyLink")}
                successLabel={t("share.copied")}
                disabled={!publicUrl}
              />
            </div>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="w-full"
              onClick={() =>
                window.open(publicUrl, "_blank", "noopener,noreferrer")
              }
            >
              {t("share.preview")}
            </Button>
          </div>
        </div>
      ) : (
        <div className="space-y-2">
          <Label>{t("share.linkName")}</Label>
          <Input
            value={draft.name}
            onChange={(e) => updateDraft({ name: e.target.value })}
            placeholder={t("share.linkNamePlaceholder")}
          />
          {errors.name && <p className="text-xs text-destructive">{errors.name}</p>}
        </div>
      )}

      <div className="space-y-2">
        <Label>{t("share.linkPreset")}</Label>
        <Select value={preset} onValueChange={handlePresetChange}>
          <SelectTrigger className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {PRESET_NAMES.map((p) => (
              <SelectItem key={p} value={p}>
                {t(`share.presets.${p}`)}
              </SelectItem>
            ))}
            <SelectItem value="custom">{t("share.presets.custom")}</SelectItem>
          </SelectContent>
        </Select>
        <p className="text-xs text-muted-foreground">{t("share.presetHint")}</p>
      </div>

      <AccessSummaryCard
        requireEmail={draft.requireEmail}
        requireEmailVerification={draft.requireEmailVerification}
        requirePassword={draft.requirePassword}
        watermarkEnabled={draft.watermarkEnabled}
        requireNda={draft.requireNda}
        allowDownloading={draft.allowDownloading}
        enableScreenshotProtection={draft.enableScreenshotProtection}
        allowedViewers={draft.allowedViewers}
        blockedViewers={draft.blockedViewers}
        onEditAccess={onEditAccess}
      />

      <div className="space-y-3">
        <div className="flex items-center justify-between gap-4">
          <Label className="font-normal">{t("share.expiresOn")}</Label>
          <Switch
            aria-label={t("share.expiresOn")}
            checked={expiresEnabled}
            onCheckedChange={(checked) =>
              updateDraft({ expiresAt: checked ? PRESETS.standard.expiresAt : "" })
            }
          />
        </div>
        {expiresEnabled && (
          <Input
            type="datetime-local"
            value={draft.expiresAt}
            onChange={(e) => updateDraft({ expiresAt: e.target.value })}
          />
        )}
        {errors.expiresAt && <p className="text-xs text-destructive">{errors.expiresAt}</p>}
      </div>

      <div className="space-y-2">
        <Label>{t("share.customDomain")}</Label>
        <Input
          value={draft.customDomain}
          onChange={(e) => updateDraft({ customDomain: e.target.value })}
          placeholder={t("share.customDomainPlaceholder")}
        />
        {errors.customDomain && <p className="text-xs text-destructive">{errors.customDomain}</p>}
        <p className="text-xs text-muted-foreground">{t("share.customDomainHint")}</p>
      </div>

      <div className="space-y-2">
        <Label>{t("share.tags")}</Label>
        <Input
          value={draft.tags.join(", ")}
          onChange={(e) =>
            updateDraft({
              tags: e.target.value
                .split(/[,;\n]+/)
                .map((s) => s.trim())
                .filter(Boolean),
            })
          }
          placeholder={t("share.tagsPlaceholder")}
        />
        <p className="text-xs text-muted-foreground">{t("share.tagsHint")}</p>
      </div>

      <div className="flex items-center justify-between gap-4">
        <div className="space-y-0.5">
          <Label className="font-normal">{t("share.notifyOnAccess")}</Label>
          <p className="text-xs text-muted-foreground">{t("share.notifyOnAccessHint")}</p>
        </div>
        <Switch
          aria-label={t("share.notifyOnAccess")}
          checked={draft.notifyOnAccess}
          onCheckedChange={(checked) => updateDraft({ notifyOnAccess: checked })}
        />
      </div>

      {slug && link && (
        <p className="text-xs text-muted-foreground">
          {t("share.oldSlugHint", { slug, token: link.shortUrl.split("/").pop() ?? "" })}
        </p>
      )}
    </div>
  );
}
