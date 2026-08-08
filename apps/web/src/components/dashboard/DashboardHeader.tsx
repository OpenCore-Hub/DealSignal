import { useTranslation } from "react-i18next";
import { useUIStore } from "@/stores/uiStore";
import { useRadarStore } from "@/stores/radarStore";

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

/** Compact Deal Radar identity block for the global TopNav. */
export function DashboardHeader({ workspaceSlug }: DashboardHeaderProps) {
  const { t, i18n } = useTranslation("dashboard");
  const currentWorkspace = useUIStore((state) => state.currentWorkspace);
  const pendingCount = useRadarStore((state) => state.openCount);

  const today = new Date().toLocaleDateString(i18n.language, {
    weekday: "long",
    year: "numeric",
    month: "long",
    day: "numeric",
  });

  const workspaceName = displayWorkspaceName(
    workspaceSlug,
    currentWorkspace?.name,
  );

  return (
    <div className="min-w-0" data-testid="dashboard-welcome-header">
      <h1 className="truncate text-base font-semibold leading-tight tracking-tight text-foreground md:text-lg">
        {t("radar.title")}
      </h1>
      <p className="mt-0.5 truncate text-xs text-muted-foreground md:text-sm">
        <span className="font-medium text-foreground/80">{workspaceName}</span>
        <span className="mx-1.5 text-muted-foreground/50">·</span>
        <span>{t("radar.openCount", { count: pendingCount })}</span>
        <span className="mx-1.5 text-muted-foreground/50">·</span>
        <span>{today}</span>
      </p>
    </div>
  );
}
