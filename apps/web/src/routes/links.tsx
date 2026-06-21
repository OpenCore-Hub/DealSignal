import { useNavigate, useParams } from "react-router";
import { Plus } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { LinksTable } from "@/components/links/LinksTable";

export function LinksPage() {
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const { t } = useTranslation("links");

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="space-y-1">
          <h1 className="text-h1">{t("page.title")}</h1>
          <p className="text-body text-muted-foreground">
            {t("page.description")}
          </p>
        </div>
        <Button onClick={() => navigate(`/${workspaceSlug}/links/new`)}>
          <Plus size={16} className="mr-2" />
          {t("page.createLink")}
        </Button>
      </div>
      <LinksTable />
    </div>
  );
}
