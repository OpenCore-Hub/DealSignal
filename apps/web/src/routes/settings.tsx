import { NavLink, Outlet, useParams } from "react-router";
import { cn } from "@/lib/utils";
import { PageHeader } from "@/components/common/PageHeader";

const sections = [
  { path: "general", label: "通用" },
  { path: "brand", label: "品牌" },
  { path: "members", label: "成员" },
  { path: "integrations", label: "集成" },
  { path: "billing", label: "账单" },
  { path: "security", label: "安全" },
];

export function SettingsPage() {
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();

  return (
    <div className="space-y-6">
      <PageHeader title="设置" description="管理工作区、品牌、成员与账户安全。" />

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
