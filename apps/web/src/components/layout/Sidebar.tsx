import { NavLink, useParams } from "react-router";
import {
  ChartPie,
  FileText,
  Link as LinkIcon,
  FolderOpen,
  Users,
  ChartLineUp,
  Gear,
  CaretLeft,
  CaretRight,
} from "@phosphor-icons/react";
import { cn } from "@/lib/utils";
import { useUIStore } from "@/stores/uiStore";
import { WorkspaceSwitcher } from "./WorkspaceSwitcher";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { useEffect } from "react";

const navItems = [
  { to: "dashboard", label: "Dashboard", icon: ChartPie },
  { to: "documents", label: "Documents", icon: FileText },
  { to: "links", label: "Links", icon: LinkIcon },
  { to: "deal-rooms", label: "Deal Rooms", icon: FolderOpen },
  { to: "contacts", label: "Contacts", icon: Users },
  { to: "insights", label: "Insights", icon: ChartLineUp },
  { to: "settings", label: "Settings", icon: Gear },
];

export function Sidebar() {
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const { sidebarOpen, toggleSidebar, setSidebarOpen } = useUIStore();
  const isMobile = useMediaQuery("(max-width: 767px)");

  useEffect(() => {
    if (isMobile) {
      setSidebarOpen(false);
    } else {
      setSidebarOpen(true);
    }
  }, [isMobile, setSidebarOpen]);

  return (
    <>
      {/* Mobile overlay */}
      {isMobile && sidebarOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/40"
          onClick={() => setSidebarOpen(false)}
          aria-hidden="true"
        />
      )}

      <aside
        className={cn(
          "fixed left-0 top-0 z-50 flex h-[100dvh] flex-col border-r border-border bg-card transition-all duration-300 ease-[cubic-bezier(0.16,1,0.3,1)] md:relative",
          sidebarOpen ? "w-64 translate-x-0" : "w-0 -translate-x-full md:w-20 md:translate-x-0"
        )}
      >
        {/* Header / Toggle */}
        <div className="flex h-16 items-center justify-between border-b border-border px-4">
          <div
            className={cn(
              "flex items-center gap-2 overflow-hidden transition-opacity",
              sidebarOpen ? "opacity-100" : "opacity-0 md:opacity-0"
            )}
          >
            <div className="flex h-8 w-8 items-center justify-center rounded-md bg-primary text-primary-foreground font-bold text-sm">
              D
            </div>
            <span className="text-h3 whitespace-nowrap">DealSignal</span>
          </div>
          <button
            onClick={toggleSidebar}
            className="flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
            aria-label={sidebarOpen ? "收起侧边栏" : "展开侧边栏"}
          >
            {sidebarOpen ? <CaretLeft size={16} /> : <CaretRight size={16} />}
          </button>
        </div>

        {/* Navigation */}
        <nav className="flex-1 overflow-y-auto p-3" aria-label="主导航">
          <ul className="space-y-1">
            {navItems.map((item) => {
              const Icon = item.icon;
              return (
                <li key={item.to}>
                  <NavLink
                    to={`/${workspaceSlug}/${item.to}`}
                    className={({ isActive }) =>
                      cn(
                        "group flex items-center gap-3 rounded-md px-3 py-2.5 text-sm font-medium transition-colors",
                        isActive
                          ? "bg-primary text-primary-foreground"
                          : "text-muted-foreground hover:bg-muted hover:text-foreground"
                      )
                    }
                  >
                    <Icon size={20} weight="regular" />
                    <span
                      className={cn(
                        "whitespace-nowrap transition-opacity",
                        sidebarOpen ? "opacity-100" : "opacity-0 md:hidden"
                      )}
                    >
                      {item.label}
                    </span>
                  </NavLink>
                </li>
              );
            })}
          </ul>
        </nav>

        {/* Workspace Switcher */}
        <div
          className={cn(
            "border-t border-border p-3 transition-opacity",
            sidebarOpen ? "opacity-100" : "opacity-0 md:hidden"
          )}
        >
          <WorkspaceSwitcher />
        </div>
      </aside>
    </>
  );
}
