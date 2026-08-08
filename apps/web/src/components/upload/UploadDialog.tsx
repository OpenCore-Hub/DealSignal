import { useCallback, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useUIStore } from "@/stores/uiStore";
import { useTranslation } from "react-i18next";
import { Uploader } from "./Uploader";

export function UploadDialog() {
  const { t } = useTranslation("documents");
  const { uploadDialogOpen, setUploadDialogOpen } = useUIStore();
  const [awaitingConflict, setAwaitingConflict] = useState(false);

  const handleOpenChange = useCallback(
    (open: boolean) => {
      // Nested replace ConfirmDialog must not dismiss this host dialog —
      // otherwise the prompt unmounts and the user never sees 覆盖/放弃.
      if (!open && awaitingConflict) return;
      setUploadDialogOpen(open);
    },
    [awaitingConflict, setUploadDialogOpen],
  );

  return (
    <Dialog open={uploadDialogOpen} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{t("upload.title")}</DialogTitle>
          <DialogDescription>{t("upload.description")}</DialogDescription>
        </DialogHeader>
        <Uploader
          onAwaitingConflictChange={setAwaitingConflict}
          onUploadComplete={() => setUploadDialogOpen(false)}
        />
      </DialogContent>
    </Dialog>
  );
}
