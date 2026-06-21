import { NavLink, Outlet, useParams } from "react-router";
import { cn } from "@/lib/utils";
import { PageHeader } from "@/components/common/PageHeader";

const tabs = [
  { path: "overview", label: "总览" },
  { path: "pages", label: "页面分析" },
  { path: "suggestions", label: "跟进建议" },
];

export function InsightsPage() {
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();

  return (
    <div className="space-y-6">
      <PageHeader
        title="洞察"
        description="追踪文档热度、页面参与度与智能跟进建议。"
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
