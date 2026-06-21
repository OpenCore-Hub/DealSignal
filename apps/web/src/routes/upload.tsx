import { useTranslation } from "react-i18next";
import { Uploader } from "@/components/upload/Uploader";

export function UploadPage() {
  const { t } = useTranslation("documents");

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <div className="space-y-2">
        <h1 className="text-h1">{t("documents:upload.title")}</h1>
        <p className="text-body text-muted-foreground">
          {t("documents:upload.description")}
        </p>
      </div>
      <Uploader />
    </div>
  );
}
