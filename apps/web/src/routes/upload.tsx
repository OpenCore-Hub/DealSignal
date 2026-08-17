import { useTranslation } from "react-i18next";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { Uploader } from "@/components/upload/Uploader";
import { documentsCreateLinkPath } from "@/lib/documentsSharePath";
import {
  isDocumentReadyForLibraryShare,
  isLibraryShareableUpload,
} from "@/lib/documentsUploadedEvent";
import type { Document } from "@/types";

export function UploadPage() {
  const { t } = useTranslation("documents");
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const [searchParams] = useSearchParams();
  const category = searchParams.get("category") || undefined;

  const handleUploadComplete = (documents: Document[]) => {
    if (category === "agreement") {
      navigate(`/${workspaceSlug}/agreement-documents`);
      return;
    }
    const shareable = documents.filter(
      (document) =>
        isDocumentReadyForLibraryShare(document.status) &&
        isLibraryShareableUpload({
          documentId: document.id,
          documentTitle: document.title || document.fileName || document.id,
          status: document.status,
          category: document.category ?? category,
        }),
    );
    if (shareable.length > 0 && workspaceSlug) {
      navigate(
        documentsCreateLinkPath(workspaceSlug, {
          documentIds: shareable.map((document) => document.id),
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
