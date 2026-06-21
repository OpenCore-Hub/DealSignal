import { NavLink, Outlet, useParams } from "react-router";
import { cn } from "@/lib/utils";
import { PageHeader } from "@/components/common/PageHeader";
import { useTranslation } from "react-i18next";

export function InsightsPage() {
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const { t } = useTranslation("insights");

  const tabs = [
    { path: "overview", label: t("nav.overview") },
    { path: "pages", label: t("nav.pages") },
    { path: "suggestions", label: t("nav.suggestions") },
  ];

  return (
    <div className="space-y-6">
      <PageHeader
        title={t("page.title")}
        description={t("page.description")}
      />

      <nav className="flex items-center gap-1 border-b border-border">
        {tabs.map((tab) => (
          <NavLink
            key={tab.path}
            to={`/${workspaceSlug}/insights/${tab.path}`}
            end={tab.path === "overview"}
            className={({ isActive }) =>
              cn(
                "relative px-4 py-2 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground",
                isActive && "text-foreground"
              )
            }
          >
            {({ isActive }) => (
              <>
                {tab.label}
                {isActive && (
                  <span className="absolute bottom-0 left-0 h-0.5 w-full bg-foreground" />
                )}
              </>
            )}
          </NavLink>
        ))}
      </nav>

      <Outlet />
    </div>
  );
}
