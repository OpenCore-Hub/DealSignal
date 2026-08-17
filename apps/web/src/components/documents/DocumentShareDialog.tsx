import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { motion } from "motion/react";
import { CopySimple } from "@phosphor-icons/react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { BundleSecurityOptions } from "@/components/links/link-bundle/BundleSecurityOptions";
import {
  bundleSecurityGuardI18nKey,
  validateBundleSecurityConfig,
} from "@/components/links/link-bundle/pipelineUtils";
import { ContactSelector } from "@/components/links/smart-link/ContactSelector";
import { api } from "@/lib/api";
import { apiErrorMessage } from "@/lib/apiErrors";
import { copyToClipboard } from "@/lib/clipboard";
import { LIBRARY_DOCUMENT_CATEGORY } from "@/lib/documentCategory";
import { createDefaultLinkPermissionConfig } from "@/lib/defaultLinkPermissionConfig";
import { isDocumentReadyForLibraryShare } from "@/lib/documentsUploadedEvent";
import { sortDocumentsByNewestUpload } from "@/lib/sortDocumentsByUploadTime";
import { cn } from "@/lib/utils";
import type { Document, PermissionConfig } from "@/types";

const enterEase = [0.32, 0.72, 0, 1] as const;

export type ShareDialogDocument = {
  id: string;
  title: string;
  status?: string;
  createdAt?: string;
  fileName?: string;
  fileType?: string;
};

export interface DocumentShareDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Pre-selected files (row Share or this upload batch). */
  documents: ShareDialogDocument[];
  workspaceSlug: string;
  onCreated?: () => void;
}

function fileExtension(name: string): string {
  const i = name.lastIndexOf(".");
  if (i <= 0 || i === name.length - 1) return "";
  return name.slice(i + 1).toUpperCase();
}

function toShareFile(doc: ShareDialogDocument | Document): ShareDialogDocument {
  return {
    id: doc.id,
    title: doc.title,
    status: doc.status,
    createdAt: doc.createdAt,
    fileName: "fileName" in doc ? doc.fileName : undefined,
    fileType: "fileType" in doc ? doc.fileType : undefined,
  };
}

function displayName(doc: ShareDialogDocument): string {
  return doc.fileName?.trim() || doc.title;
}

export function DocumentShareDialog({
  open,
  onOpenChange,
  documents,
  workspaceSlug,
  onCreated,
}: DocumentShareDialogProps) {
  const { t } = useTranslation(["documents", "common", "links"]);
  const [creating, setCreating] = useState(false);
  const [config, setConfig] = useState<PermissionConfig>(createDefaultLinkPermissionConfig);
  const [libraryDocs, setLibraryDocs] = useState<ShareDialogDocument[]>([]);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(
    () => new Set(documents.map((doc) => doc.id)),
  );
  const initialIdsKey = documents.map((doc) => doc.id).join(",");

  useEffect(() => {
    if (!open) return;
    setConfig(createDefaultLinkPermissionConfig());
    setCreating(false);
    setSelectedIds(new Set(documents.map((doc) => doc.id)));
    let cancelled = false;
    void api
      .getDocuments("all", LIBRARY_DOCUMENT_CATEGORY)
      .then((res) => {
        if (cancelled) return;
        setLibraryDocs(
          res.data
            .filter((doc) => doc.status !== "archived" && doc.status !== "failed")
            .map(toShareFile),
        );
      })
      .catch(() => {
        if (!cancelled) setLibraryDocs([]);
      });
    return () => {
      cancelled = true;
    };
  }, [open, initialIdsKey, documents]);

  const fileList = useMemo(() => {
    const byId = new Map<string, ShareDialogDocument>();
    for (const doc of libraryDocs) byId.set(doc.id, doc);
    for (const doc of documents) {
      const current = byId.get(doc.id);
      byId.set(doc.id, {
        ...current,
        ...doc,
        createdAt: current?.createdAt || doc.createdAt,
        status: current?.status || doc.status,
      });
    }
    const all = sortDocumentsByNewestUpload([...byId.values()]);
    const selected = all.filter((doc) => selectedIds.has(doc.id));
    const rest = all.filter((doc) => !selectedIds.has(doc.id));
    return [...selected, ...rest];
  }, [documents, libraryDocs, selectedIds]);

  const selectedDocuments = fileList.filter((doc) => selectedIds.has(doc.id));
  const shareReady =
    selectedDocuments.length > 0 &&
    selectedDocuments.every((doc) => isDocumentReadyForLibraryShare(doc.status ?? "ready"));
  const securityGuard = validateBundleSecurityConfig(config);
  const createBlockedReason =
    selectedDocuments.length === 0
      ? ("noFiles" as const)
      : !shareReady
        ? ("notReady" as const)
        : !securityGuard.ok
          ? securityGuard.reason
          : null;

  const toggleDocument = (id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const handleCreateAndCopy = async () => {
    if (!shareReady || selectedDocuments.length === 0) return;
    const guard = validateBundleSecurityConfig(config);
    if (!guard.ok) {
      toast.error(t(`links:${bundleSecurityGuardI18nKey(guard.reason)}`));
      return;
    }
    setCreating(true);
    try {
      const link = await api.createLink(
        selectedDocuments.map((doc) => doc.id),
        {
          ...config,
          isCustomized: true,
          level: "customized",
        },
      );
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
          "gap-0 overflow-hidden border-0 bg-transparent p-[5px] shadow-none sm:max-w-[34rem]",
          "rounded-[1.65rem]",
          "ring-1 ring-foreground/[0.06]",
          "bg-[color-mix(in_oklch,var(--muted)_72%,var(--background))]",
          "[&_[data-slot=dialog-close]]:top-4 [&_[data-slot=dialog-close]]:right-4",
        )}
      >
        <div
          className={cn(
            "overflow-hidden rounded-[calc(1.65rem-5px)] bg-background",
            "shadow-[inset_0_1px_0_rgba(255,255,255,0.72)]",
            "dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.06)]",
          )}
        >
          <div className="space-y-6 px-6 pb-2 pt-6 sm:px-7 sm:pt-7">
            <DialogHeader className="space-y-4 pr-8 text-left">
              <motion.div
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.55, ease: enterEase }}
                className="space-y-3"
              >
                <p className="font-mono text-[10px] font-medium uppercase tracking-[0.22em] text-muted-foreground/70">
                  {t("documents:share.eyebrow")}
                </p>
                <DialogTitle className="text-[1.35rem] font-semibold leading-[1.15] tracking-[-0.03em]">
                  {t("documents:share.title")}
                </DialogTitle>
                <div
                  data-testid="document-share-file-list"
                  className="max-h-40 space-y-1.5 overflow-y-auto pr-1"
                >
                  {fileList.map((doc) => {
                    const extension = fileExtension(displayName(doc));
                    const checked = selectedIds.has(doc.id);
                    return (
                      <label
                        key={doc.id}
                        data-testid={`document-share-file-${doc.id}`}
                        className={cn(
                          "flex cursor-pointer items-start gap-2.5 rounded-lg px-1 py-1",
                          checked ? "bg-muted/40" : "hover:bg-muted/25",
                        )}
                      >
                        <Checkbox
                          checked={checked}
                          onCheckedChange={() => toggleDocument(doc.id)}
                          className="mt-0.5"
                        />
                        {extension ? (
                          <span className="mt-0.5 shrink-0 rounded-md px-1.5 py-0.5 font-mono text-[10px] tracking-[0.14em] text-muted-foreground ring-1 ring-foreground/[0.08]">
                            {extension}
                          </span>
                        ) : null}
                        <p className="min-w-0 break-words text-[13px] leading-snug text-foreground/90">
                          {displayName(doc)}
                        </p>
                      </label>
                    );
                  })}
                </div>
                <DialogDescription className="space-y-2 text-[13px] leading-relaxed text-muted-foreground">
                  <span className="block max-w-[42ch]">{t("documents:share.lead")}</span>
                  {createBlockedReason === "noFiles" ? (
                    <span className="block text-amber-700 dark:text-amber-400">
                      {t("documents:share.noFilesSelected")}
                    </span>
                  ) : !shareReady ? (
                    <span className="block text-amber-700 dark:text-amber-400">
                      {t("documents:share.notReady")}
                    </span>
                  ) : null}
                </DialogDescription>
              </motion.div>
            </DialogHeader>

            <motion.div
              initial={{ opacity: 0, y: 12 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.6, delay: 0.06, ease: enterEase }}
              data-testid="document-share-advanced-card"
              className={cn(
                "rounded-[1.15rem] p-4 sm:p-5",
                "bg-[color-mix(in_oklch,var(--muted)_38%,var(--background))]",
                "ring-1 ring-foreground/[0.05]",
                "shadow-[inset_0_1px_0_rgba(255,255,255,0.55)]",
                "dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]",
              )}
            >
              <BundleSecurityOptions
                variant="atelier"
                advancedCollapsible={false}
                config={config}
                onChange={setConfig}
                excludeNdaDocumentIds={selectedDocuments.map((doc) => doc.id)}
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
            </motion.div>
          </div>

          <DialogFooter
            className={cn(
              "!mx-0 !mb-0 flex-row items-center justify-center gap-8",
              "border-t border-foreground/[0.06] bg-transparent",
              "px-6 py-5 sm:px-7 sm:!justify-center sm:space-x-0",
            )}
          >
            <Button
              type="button"
              variant="ghost"
              size="lg"
              disabled={creating}
              onClick={() => onOpenChange(false)}
              data-testid="document-share-cancel"
              className={cn(
                "h-11 min-w-[8.5rem] rounded-xl px-6 text-[13px] font-medium tracking-tight",
                "bg-rose-50 text-rose-950 ring-1 ring-rose-200/70",
                "hover:bg-rose-100 hover:text-rose-950 hover:ring-rose-300/80",
                "dark:bg-rose-950/35 dark:text-rose-50 dark:ring-rose-800/50",
                "dark:hover:bg-rose-950/50 dark:hover:text-rose-50",
                "active:scale-[0.98]",
              )}
            >
              {t("common:cancel")}
            </Button>

            <Button
              type="button"
              variant="default"
              size="lg"
              disabled={creating || createBlockedReason != null}
              title={
                createBlockedReason === "noFiles"
                  ? t("documents:share.noFilesSelected")
                  : createBlockedReason === "notReady"
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
                "h-11 min-w-[8.5rem] gap-2 rounded-xl px-6 text-[13px] font-medium tracking-tight",
                "shadow-[inset_0_1px_0_rgba(255,255,255,0.12)]",
                "active:scale-[0.98]",
              )}
            >
              {creating
                ? t("documents:share.creating")
                : t("documents:share.createAndCopy")}
              <CopySimple size={15} weight="light" />
            </Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
  );
}
