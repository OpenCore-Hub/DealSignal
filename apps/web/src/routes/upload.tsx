import { useTranslation } from "react-i18next";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { Uploader } from "@/components/upload/Uploader";

export function UploadPage() {
  const { t } = useTranslation("documents");
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const [searchParams] = useSearchParams();
  const category = searchParams.get("category") || undefined;

  const redirectPath =
    category === "agreement"
      ? `/${workspaceSlug}/agreement-documents`
      : `/${workspaceSlug}/documents`;

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <div className="space-y-2">
        <h1 className="text-h1">{t("documents:upload.title")}</h1>
        <p className="text-body text-muted-foreground">
          {t("documents:upload.description")}
        </p>
      </div>
      <Uploader category={category} onUploadComplete={() => navigate(redirectPath)} />
    </div>
  );
}
