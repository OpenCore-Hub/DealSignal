import { useState } from "react";
import { useNavigate } from "react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { api } from "@/lib/api";
import { toast } from "sonner";
import type { Document } from "@/types";

interface DeleteDocumentDialogProps {
  doc: Document;
  workspaceSlug: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function DeleteDocumentDialog({ doc, workspaceSlug, open, onOpenChange }: DeleteDocumentDialogProps) {
  const navigate = useNavigate();
  const { t } = useTranslation(["documents", "common"]);
  const [isDeleting, setIsDeleting] = useState(false);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("documents:detail.deleteDialog.title")}</DialogTitle>
          <DialogDescription>
            {t("documents:detail.deleteDialog.description", { title: doc.title })}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={isDeleting}>
            {t("common:cancel")}
          </Button>
          <Button
            variant="destructive"
            disabled={isDeleting}
            onClick={async () => {
              setIsDeleting(true);
              try {
                await api.deleteDocument(doc.id);
                toast.success(t("documents:detail.deleted"));
                navigate(`/${workspaceSlug}/documents`);
              } catch (e) {
                toast.error(e instanceof Error ? e.message : t("common:error.deleteFailed"));
                setIsDeleting(false);
              }
            }}
          >
            {isDeleting ? t("common:deleting") : t("common:delete")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
