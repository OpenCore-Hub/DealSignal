import { NavLink, Outlet, useParams } from "react-router";
import { cn } from "@/lib/utils";
import { PageHeader } from "@/components/common/PageHeader";
import { useTranslation } from "react-i18next";

export function SettingsPage() {
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const { t } = useTranslation("settings");

  const sections = [
    { path: "general", label: t("nav.general") },
    { path: "language", label: t("nav.language") },
    { path: "brand", label: t("nav.brand") },
    { path: "members", label: t("nav.members") },
    { path: "integrations", label: t("nav.integrations") },
    { path: "billing", label: t("nav.billing") },
    { path: "security", label: t("nav.security") },
    { path: "compliance", label: t("nav.compliance") },
  ];

  return (
    <div className="space-y-6">
      <PageHeader title={t("page.title")} description={t("page.description")} />

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-[240px_1fr]">
        <nav className="space-y-1 lg:self-start">
          {sections.map((section) => (
            <NavLink
              key={section.path}
              to={`/${workspaceSlug}/settings/${section.path}`}
              className={({ isActive }) =>
                cn(
                  "flex items-center rounded-md px-3 py-2 text-sm font-medium transition-colors",
                  isActive
                    ? "bg-muted text-foreground"
                    : "text-muted-foreground hover:bg-muted/50 hover:text-foreground"
                )
              }
            >
              {section.label}
            </NavLink>
          ))}
        </nav>

        <div className="min-w-0">
          <Outlet />
        </div>
      </div>
    </div>
  );
}
