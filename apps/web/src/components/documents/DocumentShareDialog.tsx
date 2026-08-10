import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { toast } from "sonner";
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
import { api } from "@/lib/api";
import { apiErrorMessage } from "@/lib/apiErrors";
import { copyToClipboard } from "@/lib/clipboard";
import { documentsCreateLinkPath } from "@/lib/documentsSharePath";
import type { PermissionConfig } from "@/types";

/** Matches BundlePipelineContext create defaults for `/links/new`. */
const LIBRARY_SHARE_DEFAULTS: PermissionConfig = {
  level: "customized",
  isCustomized: true,
  requireEmailVerification: false,
  whitelistEnabled: false,
  whitelist: [],
  passwordEnabled: false,
  ndaEnabled: false,
  ndaDocumentId: "",
  ndaTemplateId: "",
  allowDownload: true,
  watermarkEnabled: true,
  fileRequestsEnabled: false,
  indexFileEnabled: false,
  expiryDays: 30,
  maxViews: "unlimited",
  contactIds: [],
};

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
  const { t } = useTranslation(["documents", "common"]);
  const navigate = useNavigate();
  const [creating, setCreating] = useState(false);
  const shareReady = documentStatus === "ready";

  const handleCreateAndCopy = async () => {
    if (!shareReady) return;
    setCreating(true);
    try {
      const link = await api.createLink([documentId], LIBRARY_SHARE_DEFAULTS);
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
      <DialogContent className="sm:max-w-md" data-testid="document-share-dialog">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <ShareNetwork size={20} className="text-muted-foreground" />
            {t("documents:share.title")}
          </DialogTitle>
          <DialogDescription className="space-y-2 break-words">
            <span className="block">
              {t("documents:share.description", { name: documentTitle })}
            </span>
            <span className="block text-muted-foreground">
              {t("documents:share.defaultsHint")}
            </span>
            {!shareReady ? (
              <span className="block text-amber-700 dark:text-amber-400">
                {t("documents:share.notReady")}
              </span>
            ) : null}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter className="gap-2 sm:justify-between">
          <Button
            variant="ghost"
            disabled={creating}
            onClick={() => onOpenChange(false)}
          >
            {t("common:cancel")}
          </Button>
          <div className="flex flex-col-reverse gap-2 sm:flex-row">
            <Button
              variant="outline"
              disabled={creating}
              onClick={() => {
                onOpenChange(false);
                navigate(documentsCreateLinkPath(workspaceSlug, { documentId }));
              }}
              data-testid="document-share-advanced"
            >
              {t("documents:share.advanced")}
            </Button>
            <Button
              disabled={creating || !shareReady}
              title={!shareReady ? t("documents:share.notReady") : undefined}
              onClick={() => {
                void handleCreateAndCopy();
              }}
              data-testid="document-share-create"
            >
              {creating ? t("documents:share.creating") : t("documents:share.createAndCopy")}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
