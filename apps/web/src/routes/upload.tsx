import { useTranslation } from "react-i18next";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { Uploader } from "@/components/upload/Uploader";
import { isDocumentReadyForLibraryShare } from "@/lib/documentsUploadedEvent";
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
    // still "processing". DocumentsTable must NOT open Share until ready; we pass
    // handoff query params so it can wait on list polling.
    if (document?.id) {
      const status = document.status || "processing";
      const params = new URLSearchParams({
        shareDocumentId: document.id,
        shareDocumentTitle: document.title || document.fileName || document.id,
        // Only claim ready when API says so — never force-ready to open Share early.
        shareDocumentStatus: isDocumentReadyForLibraryShare(status)
          ? "ready"
          : status,
      });
      navigate(`/${workspaceSlug}/documents?${params.toString()}`);
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
