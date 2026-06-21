import { useTranslation } from "react-i18next";
import { DocumentsTable } from "@/components/documents/DocumentsTable";

export function DocumentsPage() {
  const { t } = useTranslation("documents");

  return (
    <div className="space-y-6">
      <div className="space-y-1">
        <h1 className="text-h1">{t("documents:page.title")}</h1>
        <p className="text-body text-muted-foreground">
          {t("documents:page.description")}
        </p>
      </div>
      <DocumentsTable />
    </div>
  );
}
