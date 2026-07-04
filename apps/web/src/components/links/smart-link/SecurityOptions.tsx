import { useCallback, useMemo } from "react";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router";
import type { PermissionConfig } from "@/types";
import { ContactSelector } from "./ContactSelector";
import { enforceCrossOptionConstraints } from "./levelConfig";

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

function isValidEmail(value: string) {
  return EMAIL_RE.test(value.trim());
}

interface SecurityOptionsProps {
  config: PermissionConfig;
  onChange: (config: PermissionConfig) => void;
}

export function SecurityOptions({ config, onChange }: SecurityOptionsProps) {
  const { t } = useTranslation("links");
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();

  const invalidWhitelistEmails = useMemo(() => {
    return config.whitelist.filter(
      (entry) => entry.includes("@") && !entry.startsWith("@") && !isValidEmail(entry)
    );
  }, [config.whitelist]);

  const update = useCallback(
    (patch: Partial<PermissionConfig>) => {
      const merged = { ...config, ...patch };
      // Enforce cross-option constraints (whitelist/NDA → email verification).
      const constrained = enforceCrossOptionConstraints(merged);
      onChange(constrained);
    },
    [config, onChange],
  );

  return (
    <div className="space-y-6">
      <div className="space-y-3">
        <Label>{t("creator.securityOptions")}</Label>
        <div className="space-y-3">
          {/* ---- Email Verification ---- */}
          <div className="flex items-center gap-2">
            <Checkbox
              id="require-email-verification"
              checked={config.requireEmailVerification}
              disabled={
                config.whitelistEnabled || config.ndaEnabled
              }
              onCheckedChange={(checked) =>
                update({
                  requireEmailVerification: checked === true,
                  contactId:
                    checked === true ? config.contactId : undefined,
                })
              }
            />
            <Label
              htmlFor="require-email-verification"
              className="text-sm font-normal"
            >
              {t("creator.requireEmailVerification")}
            </Label>
          </div>
          {config.requireEmailVerification && workspaceSlug && (
            <ContactSelector
              workspaceSlug={workspaceSlug}
              value={config.contactId}
              onChange={(contactId) => update({ contactId })}
            />
          )}
          {/* ---- Whitelist ---- */}
          <div className="flex items-center gap-2">
            <Checkbox
              id="whitelist"
              checked={config.whitelistEnabled}
              onCheckedChange={(checked) =>
                update({ whitelistEnabled: checked === true })
              }
            />
            <Label htmlFor="whitelist" className="text-sm font-normal">
              {t("creator.whitelist")}
            </Label>
          </div>
          {config.whitelistEnabled && (
            <>
              <Input
                placeholder={t("creator.whitelistPlaceholder")}
                value={config.whitelist.join(", ")}
                onChange={(e) =>
                  update({
                    whitelist: e.target.value
                      .split(",")
                      .map((s) => s.trim())
                      .filter(Boolean),
                  })
                }
                className="ml-6"
              />
              {invalidWhitelistEmails.length > 0 && (
                <p className="ml-6 text-xs text-destructive">
                  {t("creator.whitelistInvalidEmail", {
                    emails: invalidWhitelistEmails.join(", "),
                  })}
                </p>
              )}
            </>
          )}
          {/* ---- Password ---- */}
          <div className="flex items-center gap-2">
            <Checkbox
              id="password"
              checked={config.passwordEnabled}
              onCheckedChange={(checked) =>
                update({ passwordEnabled: checked === true })
              }
            />
            <Label htmlFor="password" className="text-sm font-normal">
              {t("creator.password")}
            </Label>
          </div>
          {config.passwordEnabled && (
            <Input
              type="password"
              placeholder={t("creator.passwordPlaceholder")}
              value={config.password ?? ""}
              onChange={(e) => update({ password: e.target.value })}
              className="ml-6"
            />
          )}
          {/* ---- NDA ---- */}
          <div className="flex items-center gap-2">
            <Checkbox
              id="nda"
              checked={config.ndaEnabled}
              onCheckedChange={(checked) =>
                update({ ndaEnabled: checked === true })
              }
            />
            <Label htmlFor="nda" className="text-sm font-normal">
              {t("creator.nda")}
            </Label>
          </div>
          {/* ---- Download ---- */}
          <div className="flex items-center gap-2">
            <Checkbox
              id="download"
              checked={config.allowDownload}
              onCheckedChange={(checked) =>
                update({ allowDownload: checked === true })
              }
            />
            <Label htmlFor="download" className="text-sm font-normal">
              {t("creator.allowDownload")}
            </Label>
          </div>
          {/* ---- Watermark ---- */}
          <div className="flex items-center gap-2">
            <Checkbox
              id="watermark"
              checked={config.watermarkEnabled}
              onCheckedChange={(checked) =>
                update({ watermarkEnabled: checked === true })
              }
            />
            <Label htmlFor="watermark" className="text-sm font-normal">
              {t("creator.watermark")}
            </Label>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div className="space-y-2">
          <Label>{t("creator.expiry")}</Label>
          <Select
            value={String(config.expiryDays)}
            onValueChange={(value) =>
              update({
                expiryDays:
                  value === "custom" ? "custom" : Number(value),
              })
            }
          >
            <SelectTrigger>
              <SelectValue placeholder={t("creator.expiryPlaceholder")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="7">{t("creator.expiryDays.7")}</SelectItem>
              <SelectItem value="30">{t("creator.expiryDays.30")}</SelectItem>
              <SelectItem value="90">{t("creator.expiryDays.90")}</SelectItem>
              <SelectItem value="custom">
                {t("creator.expiryDays.custom")}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label>{t("creator.maxViews")}</Label>
          <Select
            value={String(config.maxViews)}
            onValueChange={(value) =>
              update({
                maxViews:
                  value === "unlimited" ? "unlimited" : Number(value),
              })
            }
          >
            <SelectTrigger>
              <SelectValue
                placeholder={t("creator.maxViewsPlaceholder")}
              />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="unlimited">
                {t("creator.maxViewsOptions.unlimited")}
              </SelectItem>
              <SelectItem value="10">
                {t("creator.maxViewsOptions.10")}
              </SelectItem>
              <SelectItem value="50">
                {t("creator.maxViewsOptions.50")}
              </SelectItem>
              <SelectItem value="100">
                {t("creator.maxViewsOptions.100")}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
    </div>
  );
}
