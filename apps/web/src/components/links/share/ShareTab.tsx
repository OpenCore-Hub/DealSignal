import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ConfirmDialog } from "@/components/common/ConfirmDialog";
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
import { cn } from "@/lib/utils";
import { AccessSummaryCard } from "./AccessSummaryCard";

import { CopyButton } from "./CopyButton";
import { PRESET_NAMES } from "./presets";
import { getPublicUrl, toDateTimeLocal, isValidCustomDomain } from "./utils";
import type { DraftLink, LinkPreset } from "./types";

function highlightClass(field: string, fields: string[]) {
  return fields.includes(field)
    ? "rounded-md bg-primary/10 motion-safe:transition-colors motion-safe:duration-200"
    : "";
}

interface ShareTabProps {
  draft: DraftLink;
  updateDraft: (patch: Partial<DraftLink>) => void;
  preset: LinkPreset;
  setPreset: (preset: LinkPreset) => void;
  link: Link | null;
  onEditAccess: () => void;
  errors: Record<string, string>;
  slug?: string;
  highlightedFields?: string[];
  /** Workspace-configured custom domains available for this link. */
  availableDomains?: string[];
}

const CUSTOM_DOMAIN_VALUE = "__custom__";

export function ShareTab({
  draft,
  updateDraft,
  preset,
  setPreset,
  link,
  onEditAccess,
  errors,
  slug,
  highlightedFields = [],
  availableDomains = [],
}: ShareTabProps) {
  const { t } = useTranslation("linkShare");

  const publicUrl = getPublicUrl(link);
  const [pendingPreset, setPendingPreset] = useState<LinkPreset | null>(null);
  const isCustomValue =
    draft.customDomain !== "" && !availableDomains.includes(draft.customDomain);
  const [customDomainMode, setCustomDomainMode] = useState(isCustomValue);
  const [customDomainInput, setCustomDomainInput] = useState(
    isCustomValue ? draft.customDomain : ""
  );
  const selectedDomainValue = customDomainMode
    ? CUSTOM_DOMAIN_VALUE
    : draft.customDomain;

  const handlePresetChange = (value: string | null) => {
    if (!value) return;
    const next = value as LinkPreset;
    if (preset === "custom" && next !== "custom") {
      setPendingPreset(next);
      return;
    }
    setPreset(next);
  };

  const expiresEnabled = Boolean(draft.expiresAt);

  const expiresMin = toDateTimeLocal(new Date().toISOString());

  const defaultExpiresAt = () => {
    const d = new Date();
    d.setDate(d.getDate() + 30);
    return toDateTimeLocal(d.toISOString());
  };

  const customDomainInvalid =
    customDomainMode && customDomainInput.length > 0 && !isValidCustomDomain(customDomainInput);

  return (
    <div className="space-y-6 py-2">
      {link ? (
        <div className="space-y-3">
          <div className="space-y-2">
            <Label>{t("share.linkName")}<span className="ml-1 text-destructive">*</span></Label>
            <Input
              value={draft.name}
              onChange={(e) => updateDraft({ name: e.target.value })}
              placeholder={t("share.linkNamePlaceholder")}
              aria-required="true"
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
          <Label>{t("share.linkName")}<span className="ml-1 text-destructive">*</span></Label>
          <Input
            value={draft.name}
            onChange={(e) => updateDraft({ name: e.target.value })}
            placeholder={t("share.linkNamePlaceholder")}
            aria-required="true"
          />
          {errors.name && <p className="text-xs text-destructive">{errors.name}</p>}
        </div>
      )}

      <div className="space-y-2">
        <Label>{t("share.linkPreset")}</Label>
        <Select value={preset} onValueChange={handlePresetChange}>
          <SelectTrigger aria-label={t("share.linkPreset")} className="w-full">
            <SelectValue>
              {PRESET_NAMES.includes(preset as Exclude<LinkPreset, "custom">)
                ? t(`share.presets.${preset}`)
                : preset === "custom"
                  ? t("share.presets.custom")
                  : ""}
            </SelectValue>
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
        <p className="text-xs text-muted-foreground">
          {PRESET_NAMES.includes(preset as Exclude<LinkPreset, "custom">)
            ? t(`share.presetDescriptions.${preset}`)
            : t("share.presetHint")}
        </p>
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

      <div className={cn("space-y-3", highlightClass("expiresAt", highlightedFields))}>
        <div className="flex items-center justify-between gap-4">
          <Label className="font-normal">{t("share.expiresOn")}</Label>
          <Switch
            aria-label={t("share.expiresOn")}
            checked={expiresEnabled}
            onCheckedChange={(checked) =>
              updateDraft({ expiresAt: checked ? defaultExpiresAt() : "" })
            }
          />
        </div>
        {expiresEnabled && (
          <Input
            type="datetime-local"
            value={draft.expiresAt}
            min={expiresMin}
            onChange={(e) => updateDraft({ expiresAt: e.target.value })}
          />
        )}
        {errors.expiresAt && <p className="text-xs text-destructive">{errors.expiresAt}</p>}
      </div>

      <div className="space-y-2">
        <Label>{t("share.customDomain")}</Label>
        <Select
          value={selectedDomainValue}
          onValueChange={(value) => {
            if (value === CUSTOM_DOMAIN_VALUE) {
              setCustomDomainMode(true);
              updateDraft({ customDomain: customDomainInput });
            } else {
              setCustomDomainMode(false);
              updateDraft({ customDomain: value ?? "" });
            }
          }}
        >
          <SelectTrigger aria-label={t("share.customDomain")} className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="">{t("share.customDomainDefault")}</SelectItem>
            {availableDomains.map((domain) => (
              <SelectItem key={domain} value={domain}>
                {domain}
              </SelectItem>
            ))}
            <SelectItem value={CUSTOM_DOMAIN_VALUE}>{t("share.customDomainCustom")}</SelectItem>
          </SelectContent>
        </Select>
        {selectedDomainValue === CUSTOM_DOMAIN_VALUE && (
          <Input
            value={customDomainInput}
            onChange={(e) => {
              setCustomDomainInput(e.target.value);
              updateDraft({ customDomain: e.target.value });
            }}
            placeholder={t("share.customDomainPlaceholder")}
          />
        )}
        {errors.customDomain && <p className="text-xs text-destructive">{errors.customDomain}</p>}
        {customDomainInvalid && !errors.customDomain && (
          <p className="text-xs text-destructive">{t("share.customDomainInvalid")}</p>
        )}
        <p className="text-xs text-muted-foreground">{t("share.customDomainHint")}</p>
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

      <ConfirmDialog
        open={pendingPreset !== null}
        title={t("share.presetOverwriteTitle")}
        description={
          pendingPreset
            ? t("share.presetOverwriteDescription", { preset: t(`share.presets.${pendingPreset}`) })
            : ""
        }
        confirmLabel={t("share.presetOverwriteConfirm")}
        cancelLabel={t("share.presetOverwriteCancel")}
        onConfirm={() => {
          if (pendingPreset) setPreset(pendingPreset);
          setPendingPreset(null);
        }}
        onCancel={() => setPendingPreset(null)}
      />
    </div>
  );
}
