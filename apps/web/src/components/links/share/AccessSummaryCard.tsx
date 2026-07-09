import { useTranslation } from "react-i18next";
import { Lock, Envelope, Shield, Download, Drop } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";

interface AccessSummaryCardProps {
  requireEmail: boolean;
  requireEmailVerification: boolean;
  requirePassword: boolean;
  watermarkEnabled: boolean;
  requireNda: boolean;
  allowDownloading: boolean;
  enableScreenshotProtection: boolean;
  allowedViewers: string[];
  blockedViewers: string[];
  onEditAccess: () => void;
}

export function AccessSummaryCard({
  requireEmail,
  requireEmailVerification,
  requirePassword,
  watermarkEnabled,
  requireNda,
  allowDownloading,
  enableScreenshotProtection,
  allowedViewers,
  blockedViewers,
  onEditAccess,
}: AccessSummaryCardProps) {
  const { t } = useTranslation("linkShare");

  const items: { icon: React.ReactNode; label: string }[] = [];

  if (requirePassword) {
    items.push({ icon: <Lock size={14} />, label: t("accessRules.authentication.requirePassword") });
  }
  if (requireEmailVerification) {
    items.push({ icon: <Envelope size={14} />, label: t("accessRules.authentication.requireVerification") });
  } else if (requireEmail) {
    items.push({ icon: <Envelope size={14} />, label: t("accessRules.authentication.requireEmail") });
  }
  if (watermarkEnabled) {
    items.push({ icon: <Drop size={14} />, label: t("accessRules.additionalProtections.watermark") });
  }
  if (requireNda) {
    items.push({ icon: <Shield size={14} />, label: t("accessRules.additionalProtections.requireNda") });
  }
  if (allowDownloading) {
    items.push({ icon: <Download size={14} />, label: t("accessRules.additionalProtections.allowDownloading") });
  }
  if (enableScreenshotProtection) {
    items.push({ icon: <Shield size={14} />, label: t("accessRules.additionalProtections.screenshotProtection") });
  }

  const hasRestrictions =
    items.length > 0 || allowedViewers.length > 0 || blockedViewers.length > 0;

  return (
    <div className="rounded-lg border border-border bg-muted/30 p-4">
      <div className="flex items-center justify-between">
        <h4 className="text-sm font-medium">{t("share.accessSummary")}</h4>
        <Button type="button" variant="link" size="sm" className="h-auto p-0" onClick={onEditAccess}>
          {t("share.editAccessRules")}
        </Button>
      </div>

      {!hasRestrictions ? (
        <p className="mt-2 text-xs text-muted-foreground">{t("share.accessSummaryEmpty")}</p>
      ) : (
        <div className="mt-2 flex flex-wrap gap-2">
          {items.map((item, idx) => (
            <span
              key={idx}
              className="inline-flex items-center gap-1 rounded-full bg-background px-2 py-1 text-xs"
            >
              {item.icon}
              {item.label}
            </span>
          ))}
          {allowedViewers.slice(0, 3).map((value) => (
            <span
              key={value}
              className="inline-flex items-center rounded-full bg-success-500/10 px-2 py-1 text-xs text-success-600"
            >
              {value}
            </span>
          ))}
          {allowedViewers.length > 3 && (
            <span className="inline-flex items-center rounded-full bg-success-500/10 px-2 py-1 text-xs text-success-600">
              +{allowedViewers.length - 3}
            </span>
          )}
          {blockedViewers.slice(0, 3).map((value) => (
            <span
              key={value}
              className="inline-flex items-center rounded-full bg-destructive/10 px-2 py-1 text-xs text-destructive"
            >
              {value}
            </span>
          ))}
          {blockedViewers.length > 3 && (
            <span className="inline-flex items-center rounded-full bg-destructive/10 px-2 py-1 text-xs text-destructive">
              +{blockedViewers.length - 3}
            </span>
          )}
        </div>
      )}
    </div>
  );
}
