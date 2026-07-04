import { useTranslation } from "react-i18next";
import { DocumentsTable } from "@/components/documents/DocumentsTable";

export function AgreementDocumentsPage() {
  const { t } = useTranslation("agreementDocuments");

  return (
    <div className="space-y-6">
      <div className="space-y-1">
        <h1 className="text-h1">{t("agreementDocuments:page.title")}</h1>
        <p className="text-body text-muted-foreground">
          {t("agreementDocuments:page.description")}
        </p>
      </div>
      <DocumentsTable category="agreement" />
    </div>
  );
}
