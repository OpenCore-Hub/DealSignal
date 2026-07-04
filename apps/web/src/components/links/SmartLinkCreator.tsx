import { useEffect, useMemo, useState } from "react";
import { useLocation, useNavigate, useParams, useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { motion } from "motion/react";
import { CaretLeft } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ShieldCheck } from "@phosphor-icons/react";
import { api } from "@/lib/api";
import { copyToClipboard } from "@/lib/clipboard";
import { useReducedMotion } from "@/hooks/useReducedMotion";
import { toast } from "sonner";
import { DocumentSelector } from "./smart-link/DocumentSelector";
import { PermissionPanel } from "./smart-link/PermissionPanel";
import { SecurityOptions } from "./smart-link/SecurityOptions";
import { ScoreDisplay } from "./smart-link/ScoreDisplay";
import { LinkPreview } from "./smart-link/LinkPreview";
import {
  PRESET_TEMPLATES,
  classifyPresetFromConfig,
} from "./smart-link/levelConfig";
import type { PermissionConfig, PermissionPreset } from "@/types";

const DEFAULT_PRESET: PermissionPreset = "standard";

function buildConfigFromPreset(
  preset: PermissionPreset,
  overrides?: Partial<PermissionConfig>,
): PermissionConfig {
  const template = PRESET_TEMPLATES[preset];
  return {
    level: preset,
    isCustomized: false,
    requireEmailVerification: template.requireEmailVerification,
    whitelistEnabled: template.whitelistEnabled,
    whitelist: template.whitelist,
    passwordEnabled: template.passwordEnabled,
    ndaEnabled: template.ndaEnabled,
    allowDownload: template.allowDownload,
    watermarkEnabled: template.watermarkEnabled,
    expiryDays: template.expiryDays,
    maxViews: template.maxViews,
    ...overrides,
  };
}

export function SmartLinkCreator() {
  const navigate = useNavigate();
  const location = useLocation();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const [searchParams] = useSearchParams();
  const reducedMotion = useReducedMotion();
  const { t } = useTranslation("links");
  const [documents, setDocuments] = useState<
    import("@/types").Document[]
  >([]);
  const [loadingDocs, setLoadingDocs] = useState(true);
  const [selectedDocumentId, setSelectedDocumentId] = useState<string>("");

  const [config, setConfig] = useState<PermissionConfig>(() => {
    const newContactId = (
      location.state as { newContactId?: string } | null
    )?.newContactId;
    if (newContactId) {
      return buildConfigFromPreset(DEFAULT_PRESET, {
        requireEmailVerification: true,
        contactId: newContactId,
      });
    }
    return buildConfigFromPreset(DEFAULT_PRESET);
  });

  const [generatedLink, setGeneratedLink] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    let cancelled = false;
    api
      .getDocuments()
      .then((res) => {
        if (cancelled) return;
        setDocuments(res.data);
        const queryDocId = searchParams.get("documentId");
        const initialId =
          queryDocId && res.data.some((d) => d.id === queryDocId)
            ? queryDocId
            : res.data[0]?.id;
        if (initialId) setSelectedDocumentId(initialId);
      })
      .catch((e) => {
        if (!cancelled)
          toast.error(
            e instanceof Error ? e.message : t("creator.loadDocsFailed"),
          );
      })
      .finally(() => {
        if (!cancelled) setLoadingDocs(false);
      });
    return () => {
      cancelled = true;
    };
  }, [searchParams, t]);

  const selectedDocument = useMemo(
    () => documents.find((d) => d.id === selectedDocumentId),
    [documents, selectedDocumentId],
  );

  // Switching preset: reset config to the preset template, preserving contactId
  // only when the new preset requires email verification.
  const handleLevelChange = (newPreset: PermissionPreset) => {
    const template = PRESET_TEMPLATES[newPreset];
    setConfig((prev) =>
      buildConfigFromPreset(newPreset, {
        contactId: template.requireEmailVerification ? prev.contactId : undefined,
      }),
    );
  };

  // Manual option toggle: apply and re-classify.
  const handleConfigChange = (next: PermissionConfig) => {
    const { level, isCustomized } = classifyPresetFromConfig(next);
    setConfig({ ...next, level, isCustomized });
  };

  useEffect(() => {
    const newContactId = (
      location.state as { newContactId?: string } | null
    )?.newContactId;
    if (newContactId) {
      navigate(location.pathname + location.search, {
        replace: true,
        state: {},
      });
    }
  }, [location.state, location.pathname, location.search, navigate]);

  const createLink = async () => {
    if (!selectedDocumentId) return;
    if (config.requireEmailVerification && !config.contactId) {
      toast.error(t("creator.contactRequired"));
      return;
    }
    if (
      config.passwordEnabled &&
      (!config.password || config.password.trim() === "")
    ) {
      toast.error(t("creator.passwordEmpty"));
      return;
    }
    setCreating(true);
    try {
      const link = await api.createLink(selectedDocumentId, config);
      setGeneratedLink(link.shortUrl);
      toast.success(t("creator.createSuccess"));
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : t("creator.createFailed"),
      );
    } finally {
      setCreating(false);
    }
  };

  const handleCopy = async () => {
    if (!generatedLink) return;
    await copyToClipboard(generatedLink, t("creator.copySuccess"));
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <motion.div
      initial={reducedMotion ? false : { opacity: 0, y: 12 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.4, ease: [0.16, 1, 0.3, 1] }}
      className="mx-auto max-w-4xl space-y-6"
    >
      <div className="flex items-center gap-2">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => navigate(`/${workspaceSlug}/links`)}
        >
          <CaretLeft size={16} className="mr-1" />
          {t("creator.backToLinks")}
        </Button>
      </div>

      <div className="space-y-1">
        <h1 className="text-h1">{t("creator.title")}</h1>
        <p className="text-body text-muted-foreground">
          {t("creator.subtitle")}
        </p>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <div className="space-y-6 lg:col-span-2">
          <DocumentSelector
            documents={documents}
            loading={loadingDocs}
            selectedId={selectedDocumentId}
            onSelect={setSelectedDocumentId}
            onUpload={() =>
              navigate(`/${workspaceSlug}/documents/upload`)
            }
          />

          <Card>
            <CardHeader>
              <CardTitle className="text-h2 flex items-center gap-2">
                <ShieldCheck size={20} />
                {t("creator.securityTitle")}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">
              <PermissionPanel
                level={config.level}
                isCustomized={config.isCustomized}
                onLevelChange={handleLevelChange}
              />
              <SecurityOptions
                config={config}
                onChange={handleConfigChange}
              />
            </CardContent>
          </Card>
        </div>

        <div className="space-y-6">
          <ScoreDisplay level={config.level} config={config} />
          <LinkPreview
            selectedDocument={selectedDocument}
            config={config}
            generatedLink={generatedLink}
            copied={copied}
            creating={creating}
            onCopy={handleCopy}
            onCreate={createLink}
          />
        </div>
      </div>
    </motion.div>
  );
}
