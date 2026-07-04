import { useMemo } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  Link as LinkIcon,
  Envelope,
  Lock,
  Shield,
  Warning,
  Check,
  Copy,
  IdentificationBadge,
  Globe,
  Download,
  FileText,
} from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import type { Document, PermissionConfig } from "@/types";

interface LinkPreviewProps {
  selectedDocument?: Document;
  config: PermissionConfig;
  generatedLink: string | null;
  copied: boolean;
  creating: boolean;
  onCopy: () => void;
  onCreate: () => void;
}

interface FeatureLine {
  icon: typeof Envelope;
  active: boolean;
  labelKey: string;
}

export function LinkPreview({
  selectedDocument,
  config,
  generatedLink,
  copied,
  creating,
  onCopy,
  onCreate,
}: LinkPreviewProps) {
  const { t } = useTranslation("links");
  const { t: tc } = useTranslation("common");

  const features: FeatureLine[] = useMemo(
    () => [
      {
        icon: Envelope,
        active: config.requireEmailVerification,
        labelKey: "creator.featureEmailVerification",
      },
      {
        icon: Globe,
        active: config.whitelistEnabled,
        labelKey: "creator.featureWhitelist",
      },
      {
        icon: Lock,
        active: config.passwordEnabled,
        labelKey: "creator.featurePassword",
      },
      {
        icon: FileText,
        active: config.ndaEnabled,
        labelKey: "creator.featureNDA",
      },
      {
        icon: Shield,
        active: config.watermarkEnabled,
        labelKey: "creator.featureWatermark",
      },
      {
        icon: config.allowDownload ? Download : Warning,
        active: true,
        labelKey: config.allowDownload
          ? "creator.featureDownload"
          : "creator.featureNoDownload",
      },
    ],
    [
      config.requireEmailVerification,
      config.whitelistEnabled,
      config.passwordEnabled,
      config.ndaEnabled,
      config.watermarkEnabled,
      config.allowDownload,
    ],
  );

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-h2 flex items-center gap-2">
          <LinkIcon size={20} />
          {t("creator.previewTitle")}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="rounded-md border border-border bg-muted p-3">
          <p className="text-caption text-muted-foreground">
            {t("creator.previewDocumentLabel")}
          </p>
          <p
            data-testid="selected-document"
            className="mt-1 text-sm font-medium"
          >
            {selectedDocument?.title ?? t("creator.noDocumentSelected")}
          </p>
        </div>
        <div className="space-y-2 text-sm">
          {features.map(({ icon: Icon, active, labelKey }) => (
            <div key={labelKey} className="flex items-center gap-2">
              <Icon
                size={16}
                className={active ? "text-success-500" : "text-muted-foreground"}
              />
              <span className={active ? "" : "text-muted-foreground"}>
                {t(labelKey)}
              </span>
            </div>
          ))}
          {config.isCustomized && (
            <div className="flex items-center gap-2 pt-1">
              <IdentificationBadge
                size={16}
                className="text-warm-500"
              />
              <span className="text-warm-500">
                {t("preset.customized")}
              </span>
            </div>
          )}
        </div>

        {generatedLink ? (
          <div className="rounded-md border border-success-500/20 bg-success-500/10 p-3">
            <p className="text-caption flex items-center gap-1 text-success-500">
              <Check size={12} /> {t("creator.generatedLabel")}
            </p>
            <div className="mt-1 flex items-center gap-2">
              <code
                data-testid="generated-link"
                className="flex-1 truncate text-sm"
              >
                {generatedLink}
              </code>
              <Button
                size="icon"
                variant="ghost"
                aria-label={copied ? tc("copied") : tc("copy")}
                onClick={onCopy}
              >
                {copied ? (
                  <Check size={14} className="text-success-500" />
                ) : (
                  <Copy size={14} />
                )}
              </Button>
            </div>
          </div>
        ) : null}

        <Button
          data-testid="create-link-button"
          className="w-full"
          disabled={!selectedDocument || creating}
          onClick={onCreate}
        >
          {creating
            ? t("creator.creating")
            : generatedLink
              ? t("creator.recreate")
              : t("creator.createLink")}
        </Button>
      </CardContent>
    </Card>
  );
}
