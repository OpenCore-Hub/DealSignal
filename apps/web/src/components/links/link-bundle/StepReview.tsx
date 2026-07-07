import { useCallback, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { useTranslation } from "react-i18next";

import {
  CopyIcon,
  CheckIcon,
  EnvelopeIcon,
  PackageIcon,
  ShieldCheckIcon,
  WarningIcon,
  FileTextIcon,
  DownloadIcon,
  RobotIcon,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useBundlePipeline, clearPipelineDraft } from "./BundlePipelineContext";
import { PipelineProgress } from "./PipelineProgress";
import { copyToClipboard } from "@/lib/clipboard";
import { api } from "@/lib/api";
import { toCreateLinkPayload } from "@/lib/apiAdapters";
import {
  calculateFrictionScore,
  calculateSecurityScore,
  presetDef,
} from "../smart-link/levelConfig";
import { toast } from "sonner";
import { cn } from "@/lib/utils";

const FEATURE_META: {
  key: keyof Omit<ReturnType<typeof useFeatureConfig>, "download">;
  icon: typeof EnvelopeIcon;
  labelKey: string;
  activeClass?: string;
}[] = [
  { key: "email", icon: EnvelopeIcon, labelKey: "creator.featureEmailVerification" },
  { key: "nda", icon: FileTextIcon, labelKey: "creator.featureNDA" },
  { key: "watermark", icon: CopyIcon, labelKey: "creator.featureWatermark" },
  { key: "aiCopilot", icon: RobotIcon, labelKey: "creator.featureAICopilot", activeClass: "bg-primary/10 border-primary/20 text-primary" },
];

function useFeatureConfig(config: ReturnType<typeof useBundlePipeline>["state"]["config"]) {
  return {
    email: config.requireEmailVerification,
    nda: config.ndaEnabled,
    watermark: config.watermarkEnabled,
    aiCopilot: config.aiCopilotEnabled,
    download: config.allowDownload,
  };
}

export function StepReview() {
  const { state, dispatch } = useBundlePipeline();
  const { t } = useTranslation(["links", "common"]);
  const { t: tc } = useTranslation("common");
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();

  const { config, selectedDocuments } = state;
  const features = useFeatureConfig(config);

  const securityScore = calculateSecurityScore(config);
  const frictionScore = calculateFrictionScore(config);
  const presetInfo = presetDef[config.level];

  const isEdit = state.mode === "edit";
  const isSuccess = state.generatedLink !== null;
  const [showSaveConfirm, setShowSaveConfirm] = useState(false);

  const doSave = useCallback(async () => {
    dispatch({ type: "SET_SUBMITTING", isSubmitting: true });
    try {
      const documentIds = selectedDocuments.map((d) => d.id);
      const payload = toCreateLinkPayload(documentIds, config);

      if (isEdit && state.editingLinkId) {
        await api.updateLinkFull(state.editingLinkId, payload);
        toast.success(t("bundle.review.successUpdate"));
        dispatch({ type: "SET_DIRTY", isDirty: false });
        navigate(`/${workspaceSlug}/links`);
      } else {
        const link = await api.createLink(documentIds, config);
        dispatch({ type: "SET_GENERATED_LINK", link: link.shortUrl });
        clearPipelineDraft();
        toast.success(t("bundle.review.successCreate"));
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("creator.createFailed"));
    } finally {
      dispatch({ type: "SET_SUBMITTING", isSubmitting: false });
    }
  }, [selectedDocuments, config, isEdit, state.editingLinkId, dispatch, t, navigate, workspaceSlug]);

  const handleSubmit = useCallback(() => {
    // Client-side guard: email verification requires at least one contact.
    if (config.requireEmailVerification && config.contactIds.length === 0) {
      toast.error(t("creator.contactRequired"));
      return;
    }
    // In edit mode, show a confirmation dialog on the review step so the user
    // understands that the already-distributed link will be updated immediately.
    if (isEdit) {
      setShowSaveConfirm(true);
      return;
    }

    void doSave();
  }, [config, isEdit, doSave, t]);

  const handleCopy = async () => {
    if (!state.generatedLink) return;
    await copyToClipboard(state.generatedLink, t("creator.copySuccess"));
    dispatch({ type: "SET_COPIED", copied: true });
    setTimeout(() => dispatch({ type: "SET_COPIED", copied: false }), 2000);
  };

  const handleCancel = () => {
    if (isEdit && state.isDirty) {
      if (!window.confirm(t("bundle.unsavedConfirmDesc"))) return;
    }
    navigate(`/${workspaceSlug}/links`);
  };

  const handleCreateAnother = () => {
    clearPipelineDraft();
    dispatch({ type: "RESET" });
  };

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex justify-center">
        <PipelineProgress />
      </div>

      {/* Success card (create mode only) */}
      {isSuccess && !isEdit && (
        <Card data-testid="review-success-card" className="overflow-hidden border-success-500/20 bg-success-500/5 shadow-card">
          <CardContent className="space-y-5 p-6">
            <div className="flex items-center gap-2 text-success-500">
              <div className="flex h-8 w-8 items-center justify-center rounded-full bg-success-500/15">
                <CheckIcon size={18} weight="bold" />
              </div>
              <span className="font-semibold">{t("creator.generatedLabel")}</span>
            </div>
            <div className="flex items-center gap-2 rounded-lg border border-border bg-background p-3">
              <code data-testid="generated-link" className="flex-1 truncate text-sm">{state.generatedLink}</code>
              <Button size="sm" variant="ghost" onClick={handleCopy} className="gap-1">
                {state.copied ? <CheckIcon size={14} className="text-success-500" /> : <CopyIcon size={14} />}
                {state.copied ? tc("copied") : tc("copy")}
              </Button>
            </div>
            <div className="flex flex-wrap gap-3">
              <Button size="sm" className="gap-1.5" onClick={handleCopy}>
                <CopyIcon size={14} />
                {t("bundle.review.viewLink")}
              </Button>
              <Button
                size="sm"
                variant="outline"
                className="gap-1.5"
                onClick={() => {
                  window.open(
                    `mailto:?subject=${encodeURIComponent(selectedDocuments[0]?.title || "")}&body=${encodeURIComponent(state.generatedLink || "")}`
                  );
                }}
              >
                <EnvelopeIcon size={14} />
                {t("bundle.review.sendViaEmail")}
              </Button>
              <Button size="sm" variant="outline" onClick={handleCreateAnother}>
                {t("bundle.review.createAnother")}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Review card */}
      <Card className="shadow-card">
        <CardHeader className="border-b border-border pb-4">
          <CardTitle className="text-h2 flex items-center gap-2">
            <PackageIcon size={18} className="text-primary" />
            {t("bundle.review.documentsSection")}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-6 pt-5">
          {/* Documents section */}
          <div className="rounded-lg border border-border">
            <div className="grid grid-cols-[auto_1fr_auto] items-center gap-3 border-b border-border bg-muted/30 px-3 py-2 text-xs font-medium text-muted-foreground">
              <span className="w-6 text-center">#</span>
              <span>{t("bundle.documents.label")}</span>
              <span className="w-12 text-center">{t("creator.previewDocumentLabel")}</span>
            </div>
            {selectedDocuments.map((doc, i) => (
              <div
                key={doc.id}
                className="grid grid-cols-[auto_1fr_auto] items-center gap-3 px-3 py-2.5 transition-colors hover:bg-muted/30"
              >
                <span className="w-6 text-center text-sm text-muted-foreground">{i + 1}</span>
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">{doc.title}</p>
                  <p className="truncate text-xs text-muted-foreground">{doc.fileName}</p>
                </div>
                <span className="w-12 text-center text-xs text-muted-foreground">
                  {doc.sourceType.toUpperCase()}
                </span>
              </div>
            ))}
          </div>

          {/* Security section */}
          <div>
            <h3 className="text-h3 mb-3 flex items-center gap-2">
              <ShieldCheckIcon size={18} className="text-primary" />
              {t("bundle.review.securitySection")}
            </h3>
            <div className="rounded-lg border border-border p-4 space-y-4">
              {/* Preset name */}
              <div className="flex items-center gap-2">
                <span className="text-sm font-semibold">{t(presetInfo.label)}</span>
                {config.isCustomized && (
                  <span className="rounded-full bg-muted px-2 py-0.5 text-caption text-muted-foreground">
                    {t("preset.customized.label")}
                  </span>
                )}
              </div>

              {/* Feature badges */}
              <div className="flex flex-wrap gap-2">
                {FEATURE_META.map(({ key, icon: Icon, labelKey, activeClass }) => {
                  if (!features[key]) return null;
                  return (
                    <span
                      key={key}
                      className={cn(
                        "inline-flex items-center gap-1 rounded-full border px-2.5 py-1 text-xs",
                        activeClass ?? "bg-background"
                      )}
                    >
                      <Icon size={12} />
                      {t(labelKey)}
                    </span>
                  );
                })}
                {config.allowDownload ? (
                  <span className="inline-flex items-center gap-1 rounded-full border bg-background px-2.5 py-1 text-xs">
                    <DownloadIcon size={12} />
                    {t("creator.featureDownload")}
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-1 rounded-full border bg-background px-2.5 py-1 text-xs">
                    <WarningIcon size={12} />
                    {t("creator.featureNoDownload")}
                  </span>
                )}
              </div>

              {/* Scores */}
              <div className="flex gap-6 text-caption text-muted-foreground">
                <span>
                  {t("creator.securityScore")}: <strong className="text-foreground">{securityScore}/10</strong>
                </span>
                <span>
                  {t("creator.frictionScore")}: <strong className="text-foreground">{frictionScore}/10</strong>
                </span>
              </div>
            </div>
          </div>

          {/* AI Copilot toggle */}
          <div className="rounded-lg border border-border p-4">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <RobotIcon size={18} className="text-primary" />
                <div className="space-y-0.5">
                  <p className="text-sm font-medium">{t("creator.aiCopilot")}</p>
                  <p className="text-xs text-muted-foreground">{t("creator.aiCopilotDescription")}</p>
                </div>
              </div>
              <Switch
                checked={config.aiCopilotEnabled}
                onCheckedChange={(checked) =>
                  dispatch({
                    type: "SET_CONFIG",
                    config: {
                      ...config,
                      aiCopilotEnabled: checked,
                      level: "customized",
                      isCustomized: true,
                    },
                  })
                }
              />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Bottom action buttons */}
      {!isSuccess && (
        <div className="flex items-center justify-center gap-3">
          <Button variant="outline" onClick={handleCancel} className="w-28">
            {t("common:cancel")}
          </Button>
          <Button
            data-testid="review-submit-button"
            onClick={handleSubmit}
            disabled={state.isSubmitting}
            className="relative w-28 overflow-hidden bg-slate-700 text-white hover:bg-slate-600 animate-pulse-ring"
          >
            {!state.isSubmitting && (
              <span className="pointer-events-none absolute inset-0 animate-shimmer bg-[linear-gradient(110deg,transparent_25%,rgba(255,255,255,0.2)_50%,transparent_75%)] bg-[length:200%_100%]" />
            )}
            <span className="relative z-10">
              {state.isSubmitting
                ? t("bundle.review.submitting")
                : isEdit
                  ? t("common:save")
                  : t("common:create")}
            </span>
          </Button>
        </div>
      )}

      {/* Edit-mode save confirmation */}
      <Dialog open={showSaveConfirm} onOpenChange={setShowSaveConfirm}>
        <DialogContent showCloseButton={false}>
          <DialogHeader>
            <DialogTitle>{t("bundle.review.saveConfirmTitle")}</DialogTitle>
            <DialogDescription>{t("bundle.review.saveConfirmDesc")}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowSaveConfirm(false)}>
              {t("common:cancel")}
            </Button>
            <Button
              onClick={() => {
                setShowSaveConfirm(false);
                void doSave();
              }}
              disabled={state.isSubmitting}
            >
              {state.isSubmitting ? t("bundle.review.submitting") : t("bundle.review.saveButton")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
