import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
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
import type { PermissionConfig } from "@/types";
import type { PermissionLevel } from "./smart-link/types";

const DEFAULT_CONFIG: PermissionConfig = {
  level: "low",
  requireEmail: false,
  whitelistEnabled: false,
  whitelist: [],
  passwordEnabled: false,
  allowDownload: false,
  watermarkEnabled: false,
  expiryDays: 7,
  maxViews: "unlimited",
};

export function SmartLinkCreator() {
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const [searchParams] = useSearchParams();
  const reducedMotion = useReducedMotion();
  const { t } = useTranslation("links");
  const [documents, setDocuments] = useState<import("@/types").Document[]>([]);
  const [loadingDocs, setLoadingDocs] = useState(true);
  const [selectedDocumentId, setSelectedDocumentId] = useState<string>("");
  const [level, setLevel] = useState<PermissionLevel>("low");
  const [config, setConfig] = useState<PermissionConfig>(DEFAULT_CONFIG);
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
        const initialId = queryDocId && res.data.some((d) => d.id === queryDocId) ? queryDocId : res.data[0]?.id;
        if (initialId) setSelectedDocumentId(initialId);
      })
      .catch((e) => {
        if (!cancelled) toast.error(e instanceof Error ? e.message : t("creator.loadDocsFailed"));
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
    [documents, selectedDocumentId]
  );

  const handleLevelChange = (newLevel: PermissionLevel) => {
    setLevel(newLevel);
    setConfig((prev) => ({
      ...prev,
      level: newLevel,
      requireEmail: newLevel !== "low" || prev.requireEmail,
      whitelistEnabled: newLevel === "high" || (newLevel === "medium" && prev.whitelistEnabled),
      passwordEnabled: newLevel === "high" || (newLevel === "medium" && prev.passwordEnabled),
      watermarkEnabled: newLevel === "high" || prev.watermarkEnabled,
    }));
  };

  const createLink = async () => {
    if (!selectedDocumentId) return;
    setCreating(true);
    try {
      const link = await api.createLink(selectedDocumentId, config);
      setGeneratedLink(link.shortUrl);
      toast.success(t("creator.createSuccess"));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("creator.createFailed"));
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
        <Button variant="ghost" size="sm" onClick={() => navigate(`/${workspaceSlug}/links`)}>
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
            onUpload={() => navigate(`/${workspaceSlug}/documents/upload`)}
          />

          <Card>
            <CardHeader>
              <CardTitle className="text-h2 flex items-center gap-2">
                <ShieldCheck size={20} />
                {t("creator.securityTitle")}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">
              <PermissionPanel level={level} onLevelChange={handleLevelChange} />
              <SecurityOptions config={config} onChange={setConfig} />
            </CardContent>
          </Card>
        </div>

        <div className="space-y-6">
          <ScoreDisplay level={level} config={config} />
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
