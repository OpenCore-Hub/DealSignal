import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { AnimatePresence, motion } from "motion/react";
import { ShareNetwork } from "@phosphor-icons/react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { BundleSecurityOptions } from "@/components/links/link-bundle/BundleSecurityOptions";
import { validateBundleSecurityConfig } from "@/components/links/link-bundle/pipelineUtils";
import { ContactSelector } from "@/components/links/smart-link/ContactSelector";
import { api } from "@/lib/api";
import { apiErrorMessage } from "@/lib/apiErrors";
import { copyToClipboard } from "@/lib/clipboard";
import { createDefaultLinkPermissionConfig } from "@/lib/defaultLinkPermissionConfig";
import { cn } from "@/lib/utils";
import type { PermissionConfig } from "@/types";

export interface DocumentShareDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  documentId: string;
  documentTitle: string;
  workspaceSlug: string;
  /** When not ready, Create & copy stays disabled (processing uploads). */
  documentStatus?: string;
  onCreated?: () => void;
}

export function DocumentShareDialog({
  open,
  onOpenChange,
  documentId,
  documentTitle,
  workspaceSlug,
  documentStatus = "ready",
  onCreated,
}: DocumentShareDialogProps) {
  const { t } = useTranslation(["documents", "common", "links"]);
  const [creating, setCreating] = useState(false);
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [config, setConfig] = useState<PermissionConfig>(createDefaultLinkPermissionConfig);
  const shareReady = documentStatus === "ready";
  const securityGuard = validateBundleSecurityConfig(config);
  const createBlockedReason =
    !shareReady
      ? ("notReady" as const)
      : !securityGuard.ok
        ? securityGuard.reason
        : null;

  // Reset panel + defaults whenever the dialog opens for a document.
  useEffect(() => {
    if (!open) return;
    setAdvancedOpen(false);
    setConfig(createDefaultLinkPermissionConfig());
    setCreating(false);
  }, [open, documentId]);

  const handleCreateAndCopy = async () => {
    if (!shareReady) return;
    const guard = validateBundleSecurityConfig(config);
    if (!guard.ok) {
      toast.error(
        guard.reason === "ndaDocumentRequired"
          ? t("links:creator.ndaDocumentRequired")
          : t("links:creator.contactRequired"),
      );
      if (!advancedOpen) setAdvancedOpen(true);
      return;
    }
    setCreating(true);
    try {
      const link = await api.createLink([documentId], {
        ...config,
        isCustomized: true,
        level: "customized",
      });
      const url = link.shortUrl?.trim();
      if (!url) {
        toast.error(t("documents:share.createFailed"));
        return;
      }
      await copyToClipboard(url, t("documents:share.copied"));
      onOpenChange(false);
      onCreated?.();
    } catch (e) {
      toast.error(apiErrorMessage(e, { messageKey: "documents:share.createFailed" }));
    } finally {
      setCreating(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(next) => !creating && onOpenChange(next)}>
      <DialogContent
        data-testid="document-share-dialog"
        className={cn(
          "gap-0 overflow-hidden p-0 sm:max-w-md",
          advancedOpen && "sm:max-w-lg",
        )}
      >
        <div className="space-y-5 p-5 sm:p-6">
          <DialogHeader className="space-y-3 text-left">
            <DialogTitle className="flex items-center gap-2.5 text-[1.2rem] font-semibold tracking-tight">
              <span className="flex size-9 items-center justify-center rounded-xl bg-foreground text-background shadow-[inset_0_1px_0_rgba(255,255,255,0.18)]">
                <ShareNetwork size={18} weight="bold" />
              </span>
              {t("documents:share.title")}
            </DialogTitle>
            <DialogDescription className="space-y-2 break-words text-[13px] leading-relaxed">
              <span className="block text-foreground/85">
                {t("documents:share.description", { name: documentTitle })}
              </span>
              {!advancedOpen ? (
                <span className="block text-muted-foreground">
                  {t("documents:share.defaultsHint")}
                </span>
              ) : null}
              {!shareReady ? (
                <span className="block text-amber-700 dark:text-amber-400">
                  {t("documents:share.notReady")}
                </span>
              ) : null}
            </DialogDescription>
          </DialogHeader>

          <AnimatePresence initial={false}>
            {advancedOpen ? (
              <motion.div
                key="advanced-card"
                initial={{ opacity: 0, y: -6, height: 0 }}
                animate={{ opacity: 1, y: 0, height: "auto" }}
                exit={{ opacity: 0, y: -4, height: 0 }}
                transition={{ duration: 0.28, ease: [0.22, 1, 0.36, 1] }}
                className="overflow-hidden"
              >
                <div
                  data-testid="document-share-advanced-card"
                  className={cn(
                    "rounded-2xl border border-border/70 bg-card/90 p-4",
                    "shadow-[0_18px_40px_-28px_rgba(15,23,42,0.45),inset_0_1px_0_rgba(255,255,255,0.55)]",
                    "ring-1 ring-foreground/[0.03]",
                  )}
                >
                  <BundleSecurityOptions
                    config={config}
                    onChange={setConfig}
                    excludeNdaDocumentIds={[documentId]}
                    contactSelector={
                      (config.requireEmailVerification || config.ndaEnabled) &&
                      workspaceSlug ? (
                        <div className="animate-in fade-in-0 slide-in-from-top-1 duration-200">
                          <ContactSelector
                            workspaceSlug={workspaceSlug}
                            value={config.contactIds}
                            onChange={(contactIds) =>
                              setConfig((prev) => ({ ...prev, contactIds }))
                            }
                          />
                        </div>
                      ) : null
                    }
                  />
                </div>
              </motion.div>
            ) : null}
          </AnimatePresence>
        </div>

        <DialogFooter
          className={cn(
            "!mx-0 !mb-0 !grid grid-cols-3 items-center gap-2",
            "border-t border-border/60 bg-gradient-to-b from-muted/30 via-muted/45 to-muted/60",
            "px-5 py-4 sm:px-6 sm:!justify-stretch sm:space-x-0",
          )}
        >
          <div className="justify-self-start">
            <Button
              type="button"
              variant="default"
              size="lg"
              disabled={creating || createBlockedReason != null}
              title={
                createBlockedReason === "notReady"
                  ? t("documents:share.notReady")
                  : createBlockedReason === "ndaDocumentRequired"
                    ? t("links:creator.ndaDocumentRequired")
                    : createBlockedReason === "contactRequired"
                      ? t("links:creator.contactRequired")
                      : undefined
              }
              onClick={() => {
                void handleCreateAndCopy();
              }}
              data-testid="document-share-create"
              className={cn(
                "min-w-[6.5rem] rounded-xl px-5 font-semibold tracking-tight",
                "border border-border bg-rose-50 text-rose-950",
                "shadow-[inset_0_1px_0_rgba(255,255,255,0.7)]",
                "hover:border-foreground/20 hover:bg-rose-100 hover:text-rose-950",
                "active:translate-y-px",
                "animate-pulse-ring",
                "disabled:animate-none",
              )}
            >
              {creating
                ? t("documents:share.creating")
                : t("documents:share.createAndCopy")}
            </Button>
          </div>

          <div className="justify-self-center">
            <Button
              type="button"
              variant="outline"
              size="lg"
              disabled={creating}
              aria-expanded={advancedOpen}
              aria-pressed={advancedOpen}
              onClick={() => setAdvancedOpen((v) => !v)}
              data-testid="document-share-advanced"
              className={cn(
                "min-w-[6.5rem] rounded-xl px-4 font-medium tracking-tight active:translate-y-px",
                !advancedOpen &&
                  "border border-border bg-background text-muted-foreground shadow-none",
                !advancedOpen &&
                  "hover:border-foreground/20 hover:bg-muted/60 hover:text-foreground",
                // Solid fill when open — distinct from Create's pale rose wash.
                advancedOpen &&
                  "border border-rose-500 !bg-rose-500 font-semibold !text-white",
                advancedOpen &&
                  "shadow-[0_1px_0_rgba(255,255,255,0.2)_inset,0_0_0_3px_rgba(244,63,94,0.2)]",
                advancedOpen &&
                  "hover:border-rose-600 hover:!bg-rose-600 hover:!text-white",
              )}
            >
              {t("documents:share.advanced")}
            </Button>
          </div>

          <div className="justify-self-end">
            <Button
              type="button"
              variant="ghost"
              size="lg"
              disabled={creating}
              onClick={() => onOpenChange(false)}
              data-testid="document-share-cancel"
              className={cn(
                "min-w-[5.5rem] rounded-xl px-4 font-medium tracking-tight",
                "text-muted-foreground hover:bg-background/70 hover:text-foreground",
                "active:translate-y-px",
              )}
            >
              {t("common:cancel")}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
