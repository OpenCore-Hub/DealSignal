import { useTranslation } from "react-i18next";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { Uploader } from "@/components/upload/Uploader";
import { documentsLibraryShareHandoffPath } from "@/lib/documentsLibraryShareHandoff";
import type { Document } from "@/types";

export function UploadPage() {
  const { t } = useTranslation("documents");
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const [searchParams] = useSearchParams();
  const category = searchParams.get("category") || undefined;

  const handleUploadComplete = (document?: Document) => {
    if (category === "agreement") {
      navigate(`/${workspaceSlug}/agreement-documents`);
      return;
    }
    // POST /documents returns as soon as the object is stored — status is usually
    // still "processing". DocumentsTable must NOT open Share until ready; handoff
    // params encode status (normalized) so the library can wait on list polling.
    if (document?.id && workspaceSlug) {
      navigate(
        documentsLibraryShareHandoffPath(workspaceSlug, {
          documentId: document.id,
          documentTitle: document.title || document.fileName || document.id,
          documentStatus: document.status || "processing",
        }),
      );
      return;
    }
    navigate(`/${workspaceSlug}/documents`);
  };

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <div className="space-y-2">
        <h1 className="text-h1">{t("documents:upload.title")}</h1>
        <p className="text-body text-muted-foreground">
          {t("documents:upload.description")}
        </p>
      </div>
      <Uploader category={category} onUploadComplete={handleUploadComplete} />
    </div>
  );
}
