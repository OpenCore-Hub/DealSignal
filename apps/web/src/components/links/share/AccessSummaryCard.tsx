import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Folder, Lock, Envelope, Shield, Download, Drop, CaretDown } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { DealRoomFolderDocs } from "@/types";

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
  folderPaths?: string[];
  documents?: DealRoomFolderDocs[];
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
  folderPaths = [],
  documents = [],
  onEditAccess,
}: AccessSummaryCardProps) {
  const { t } = useTranslation("linkShare");
  const [open, setOpen] = useState(true);
  const toggle = () => setOpen((v) => !v);

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

  const selectedFolderCount = folderPaths.length;
  const scopedDocumentCount = useMemo(() => {
    if (selectedFolderCount === 0) return 0;
    let count = 0;
    for (const folder of documents) {
      const folderPath = folder.folder;
      if (
        folderPaths.some(
          (s) =>
            s === folderPath ||
            (folderPath.length > s.length && folderPath.startsWith(`${s}/`))
        )
      ) {
        count += (folder.documents ?? []).length;
      }
    }
    return count;
  }, [documents, folderPaths, selectedFolderCount]);
  const isScoped = selectedFolderCount > 0;

  const hasRestrictions =
    items.length > 0 || allowedViewers.length > 0 || blockedViewers.length > 0 || isScoped;
  const restrictionCount =
    items.length + allowedViewers.length + blockedViewers.length + (isScoped ? 1 : 0);

  return (
    <div className="rounded-lg border border-border bg-muted/30 p-4">
      <div className="flex items-center justify-between gap-2">
        <button
          type="button"
          onClick={toggle}
          className="flex flex-1 items-center gap-2 text-left"
          aria-expanded={open}
        >
          <h4 className="text-sm font-medium">{t("share.accessSummary")}</h4>
          {restrictionCount > 0 && (
            <span className="inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-background px-1.5 text-xs text-muted-foreground">
              {restrictionCount}
            </span>
          )}
        </button>
        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="link"
            size="sm"
            className="h-auto p-0"
            onClick={(e) => {
              e.stopPropagation();
              onEditAccess();
            }}
          >
            {t("share.editAccessRules")}
          </Button>
          <button
            type="button"
            onClick={toggle}
            className="inline-flex items-center justify-center text-muted-foreground hover:text-foreground"
            aria-label={open ? t("share.collapseAccessSummary") : t("share.expandAccessSummary")}
          >
            <CaretDown
              size={16}
              className={cn("transition-transform", open && "rotate-180")}
            />
          </button>
        </div>
      </div>

      {open && (
        <div className="mt-3">
          {!hasRestrictions ? (
            <p className="text-xs text-muted-foreground">{t("share.accessSummaryEmpty")}</p>
          ) : (
            <div className="flex flex-wrap gap-2">
              {items.map((item, idx) => (
                <span
                  key={idx}
                  className="inline-flex items-center gap-1 rounded-full bg-background px-2 py-1 text-xs"
                >
                  {item.icon}
                  {item.label}
                </span>
              ))}
              {isScoped && (
                <span className="inline-flex items-center gap-1 rounded-full bg-primary/10 px-2 py-1 text-xs text-primary">
                  <Folder size={14} />
                  {t("share.accessSummaryScope", {
                    folders: selectedFolderCount,
                    documents: scopedDocumentCount,
                  })}
                </span>
              )}
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
      )}
    </div>
  );
}
