import { useTranslation } from "react-i18next";
import { useUIStore } from "@/stores/uiStore";

interface DashboardHeaderProps {
  workspaceSlug: string;
}

function displayWorkspaceName(slug: string, name?: string | null): string {
  if (name) return name;
  return slug
    .split("-")
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

export function DashboardHeader({ workspaceSlug }: DashboardHeaderProps) {
  const { t, i18n } = useTranslation("dashboard");
  const currentWorkspace = useUIStore((state) => state.currentWorkspace);

  const today = new Date().toLocaleDateString(i18n.language, {
    weekday: "long",
    year: "numeric",
    month: "long",
    day: "numeric",
  });

  const workspaceName = displayWorkspaceName(
    workspaceSlug,
    currentWorkspace?.name
  );

  return (
    <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
      <div>
        <h1 className="text-h1">{t("welcome.title")}</h1>
        <p className="text-body mt-1 text-muted-foreground">
          <span className="font-medium text-foreground">{workspaceName}</span>
          <span className="mx-2">·</span>
          <span>{today}</span>
        </p>
      </div>
    </div>
  );
}
