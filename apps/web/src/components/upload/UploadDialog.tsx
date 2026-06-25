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

  return (
    <Dialog open={uploadDialogOpen} onOpenChange={setUploadDialogOpen}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{t("upload.title")}</DialogTitle>
          <DialogDescription>{t("upload.description")}</DialogDescription>
        </DialogHeader>
        <Uploader onUploadComplete={() => setUploadDialogOpen(false)} />
      </DialogContent>
    </Dialog>
  );
}
