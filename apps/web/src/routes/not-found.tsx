import { useNavigate, useParams } from "react-router";
import { House } from "@phosphor-icons/react";
import { EmptyState } from "@/components/common/EmptyState";
import { useTranslation } from "react-i18next";

export function NotFoundPage() {
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const { t } = useTranslation("common");

  return (
    <div className="flex h-full items-center justify-center">
      <EmptyState
        icon={<House size={48} />}
        title={t("notFound.title")}
        description={t("notFound.description")}
        action={{ label: t("notFound.backToDashboard"), onClick: () => navigate(`/${workspaceSlug}/dashboard`) }}
        size="large"
      />
    </div>
  );
}
