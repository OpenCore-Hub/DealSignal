import { useCallback, useState } from "react";
import { apiErrorMessage } from "@/lib/apiErrors";
import { useNavigate, useParams } from "react-router";
import { useTranslation } from "react-i18next";

import {
  CopyIcon,
  CheckIcon,
  EnvelopeIcon,
  FileTextIcon,
  DownloadIcon,
  WarningIcon,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
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
import { PipelinePaper } from "./PipelinePaper";
import { copyToClipboard } from "@/lib/clipboard";
import { api } from "@/lib/api";
import { toCreateLinkPayload } from "@/lib/apiAdapters";
import { documentsSharePath } from "@/lib/documentsSharePath";
import {
  bundleSecurityGuardI18nKey,
  resolveShareDocumentReadiness,
  validateBundleSecurityConfig,
} from "./pipelineUtils";
import {
  calculateFrictionScore,
  calculateSecurityScore,
  presetDef,
} from "../smart-link/levelConfig";
import { ScoreBar } from "../smart-link/ScoreBar";
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
];

function useFeatureConfig(config: ReturnType<typeof useBundlePipeline>["state"]["config"]) {
  return {
    email: config.requireEmailVerification,
    nda: config.ndaEnabled,
    watermark: config.watermarkEnabled,
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
  const documentReadiness = resolveShareDocumentReadiness(selectedDocuments);

  const securityScore = calculateSecurityScore(config);
  const frictionScore = calculateFrictionScore(config);
  const presetInfo = presetDef[config.level];

  const isEdit = state.mode === "edit";
  const isSuccess = state.generatedLink !== null;
  const [showSaveConfirm, setShowSaveConfirm] = useState(false);

  const doSave = useCallback(async () => {
    const readiness = resolveShareDocumentReadiness(selectedDocuments);
    if (!readiness.ready) {
      toast.error(
        t(
          readiness.reason === "failed"
            ? "bundle.review.ingestionFailed"
            : "bundle.review.notReady",
        ),
      );
      return;
    }
    dispatch({ type: "SET_SUBMITTING", isSubmitting: true });
    try {
      const documentIds = selectedDocuments.map((d) => d.id);
      const payload = toCreateLinkPayload(documentIds, config);

      if (isEdit && state.editingLinkId) {
        await api.updateLinkFull(state.editingLinkId, payload);
        toast.success(t("bundle.review.successUpdate"));
        dispatch({ type: "SET_DIRTY", isDirty: false });
        navigate(documentsSharePath(workspaceSlug!));
      } else {
        const link = await api.createLink(documentIds, config);
        dispatch({ type: "SET_GENERATED_LINK", link: link.shortUrl });
        clearPipelineDraft();
        toast.success(t("bundle.review.successCreate"));
      }
    } catch (e) {
      toast.error(apiErrorMessage(e, { messageKey: "links:creator.createFailed" }));
    } finally {
      dispatch({ type: "SET_SUBMITTING", isSubmitting: false });
    }
  }, [selectedDocuments, config, isEdit, state.editingLinkId, dispatch, t, navigate, workspaceSlug]);

  const handleSubmit = useCallback(() => {
    if (!documentReadiness.ready) {
      toast.error(
        t(
          documentReadiness.reason === "failed"
            ? "bundle.review.ingestionFailed"
            : "bundle.review.notReady",
        ),
      );
      return;
    }
    const guard = validateBundleSecurityConfig(config);
    if (!guard.ok) {
      toast.error(t(bundleSecurityGuardI18nKey(guard.reason)));
      return;
    }
    // In edit mode, show a confirmation dialog on the review step so the user
    // understands that the already-distributed link will be updated immediately.
    if (isEdit) {
      setShowSaveConfirm(true);
      return;
    }

    void doSave();
  }, [config, documentReadiness, isEdit, doSave, t]);

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
    navigate(documentsSharePath(workspaceSlug!));
  };

  return (
    <div className="mx-auto w-full max-w-3xl space-y-6">
      <div className="flex justify-center">
        <PipelineProgress />
      </div>

      {isSuccess && !isEdit ? (
        <PipelinePaper>
          <div data-testid="review-success-card" className="space-y-4 px-6 py-5 sm:px-7 sm:py-6">
            <div className="flex items-center gap-2">
              <CheckIcon size={16} weight="light" className="text-foreground" />
              <span className="text-[15px] font-semibold tracking-[-0.02em]">
                {t("creator.generatedLabel")}
              </span>
            </div>
            <div className="flex items-center gap-2 rounded-xl px-3 py-2.5 ring-1 ring-foreground/[0.08]">
              <code data-testid="generated-link" className="min-w-0 flex-1 truncate font-mono text-[12px]">
                {state.generatedLink}
              </code>
              <Button size="sm" variant="ghost" onClick={handleCopy} className="h-8 gap-1.5 rounded-lg px-2.5">
                {state.copied ? <CheckIcon size={14} weight="light" /> : <CopyIcon size={14} weight="light" />}
                {state.copied ? tc("copied") : tc("copy")}
              </Button>
            </div>
          </div>
        </PipelinePaper>
      ) : null}

      <PipelinePaper>
        <div className="space-y-8 px-6 py-5 sm:px-7 sm:py-6">
          <section className="space-y-3">
            <h3 className="flex items-baseline gap-2.5">
              <span className="font-mono text-[10px] tracking-[0.16em] text-muted-foreground/45">
                01
              </span>
              <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-foreground/75">
                {t("bundle.review.documentsSection")}
              </span>
            </h3>
            <div className="divide-y divide-foreground/[0.06]">
              {selectedDocuments.map((doc, i) => (
                <div key={doc.id} className="grid grid-cols-[auto_1fr_auto] items-center gap-3 py-2.5">
                  <span className="w-5 font-mono text-[10px] tabular-nums text-muted-foreground/50">
                    {String(i + 1).padStart(2, "0")}
                  </span>
                  <div className="min-w-0">
                    <p className="truncate text-[13px] font-medium">{doc.title}</p>
                    <p className="truncate text-[12px] text-muted-foreground">{doc.fileName}</p>
                    {doc.status && doc.status !== "ready" ? (
                      <p className="mt-0.5 text-[12px] text-amber-700 dark:text-amber-400">
                        {t(`bundle.documents.status.${doc.status}`)}
                      </p>
                    ) : null}
                  </div>
                  <span className="rounded-md px-1.5 py-0.5 font-mono text-[10px] tracking-[0.14em] text-muted-foreground ring-1 ring-foreground/[0.08]">
                    {doc.sourceType.toUpperCase()}
                  </span>
                </div>
              ))}
            </div>
          </section>

          <section className="space-y-4">
            <h3 className="flex items-baseline gap-2.5">
              <span className="font-mono text-[10px] tracking-[0.16em] text-muted-foreground/45">
                02
              </span>
              <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-foreground/75">
                {t("bundle.review.securitySection")}
              </span>
            </h3>

            <div className="flex items-baseline gap-2">
              <span className="text-[13px] font-semibold tracking-[-0.01em]">{t(presetInfo.label)}</span>
              {config.isCustomized ? (
                <span className="font-mono text-[10px] tracking-[0.12em] text-muted-foreground/70">
                  {t("preset.customized.label")}
                </span>
              ) : null}
            </div>

            <div className="flex flex-wrap gap-2">
              {FEATURE_META.map(({ key, icon: Icon, labelKey }) => {
                if (!features[key]) return null;
                return (
                  <span
                    key={key}
                    className="inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[12px] ring-1 ring-foreground/[0.08]"
                  >
                    <Icon size={13} weight="light" />
                    {t(labelKey)}
                  </span>
                );
              })}
              {config.allowDownload ? (
                <span className="inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[12px] ring-1 ring-foreground/[0.08]">
                  <DownloadIcon size={13} weight="light" />
                  {t("creator.featureDownload")}
                </span>
              ) : (
                <span className="inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[12px] ring-1 ring-foreground/[0.08]">
                  <WarningIcon size={13} weight="light" />
                  {t("creator.featureNoDownload")}
                </span>
              )}
            </div>

            <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
              <ScoreBar
                label={t("creator.securityScore")}
                score={securityScore}
                variant="security"
                layout="card"
              />
              <ScoreBar
                label={t("creator.frictionScore")}
                score={frictionScore}
                variant="friction"
                layout="card"
              />
            </div>
          </section>
        </div>
      </PipelinePaper>

      {!isSuccess ? (
        <div className="flex flex-col items-center justify-center gap-3">
          {documentReadiness.reason === "processing" ? (
            <p className="text-[13px] text-amber-700 dark:text-amber-400">
              {t("bundle.review.notReady")}
            </p>
          ) : null}
          {documentReadiness.reason === "failed" ? (
            <p className="text-[13px] text-amber-700 dark:text-amber-400">
              {t("bundle.review.ingestionFailed")}
            </p>
          ) : null}
          <div className="flex items-center justify-center gap-8">
          <Button
            variant="ghost"
            onClick={handleCancel}
            className={cn(
              "h-11 min-w-[8.5rem] rounded-xl px-6 text-[13px] font-medium tracking-tight",
              "bg-rose-50 text-rose-950 ring-1 ring-rose-200/70",
              "hover:bg-rose-100 hover:text-rose-950 hover:ring-rose-300/80",
              "dark:bg-rose-950/35 dark:text-rose-50 dark:ring-rose-800/50",
              "dark:hover:bg-rose-950/50",
              "active:scale-[0.98]",
            )}
          >
            {t("common:cancel")}
          </Button>
          <Button
            data-testid="review-submit-button"
            onClick={handleSubmit}
            disabled={state.isSubmitting || !documentReadiness.ready}
            className={cn(
              "h-11 min-w-[8.5rem] rounded-xl px-6 text-[13px] font-medium tracking-tight",
              "shadow-[inset_0_1px_0_rgba(255,255,255,0.12)]",
              "active:scale-[0.98]",
            )}
          >
            {state.isSubmitting
              ? t("bundle.review.submitting")
              : isEdit
                ? t("common:save")
                : t("common:create")}
          </Button>
          </div>
        </div>
      ) : null}

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
