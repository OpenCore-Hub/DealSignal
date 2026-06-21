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
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { WorkspaceSettings } from "@/types";
import { Skeleton } from "@/components/ui/skeleton";

const navItems = [
  { to: "dashboard", label: "交易雷达", icon: ChartPie },
  { to: "documents", label: "文档库", icon: FileText },
  { to: "links", label: "链接", icon: LinkIcon },
  { to: "deal-rooms", label: "数据室", icon: FolderOpen },
  { to: "contacts", label: "联系人", icon: Users },
  { to: "insights", label: "洞察", icon: ChartLineUp },
  { to: "settings", label: "设置", icon: Gear },
];

export function Sidebar() {
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const { sidebarOpen, toggleSidebar, setSidebarOpen } = useUIStore();
  const isMobile = useMediaQuery("(max-width: 767px)");
  const [settings, setSettings] = useState<WorkspaceSettings | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (isMobile) {
      setSidebarOpen(false);
    } else {
      setSidebarOpen(true);
    }
  }, [isMobile, setSidebarOpen]);

  useEffect(() => {
    async function loadSettings() {
      try {
        setLoading(true);
        const res = await api.getWorkspaceSettings();
        setSettings(res.data);
      } catch {
        // Keep default logo on error
      } finally {
        setLoading(false);
      }
    }
    loadSettings();
  }, []);

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
            {loading ? (
              <Skeleton className="h-8 w-8 rounded-md" />
            ) : settings?.logoUrl ? (
              <img
                src={settings.logoUrl}
                alt={settings.name}
                className="h-8 w-8 rounded-md object-contain"
              />
            ) : (
              <div className="flex h-8 w-8 items-center justify-center rounded-md bg-primary text-primary-foreground font-bold text-sm">
                {settings?.name?.charAt(0).toUpperCase() || "D"}
              </div>
            )}
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
                    title={item.label}
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
