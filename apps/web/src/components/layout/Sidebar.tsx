import { NavLink, useParams, useLocation } from "react-router";
import {
  ChartPie,
  FileText,
  Scales,
  FolderOpen,
  Users,
  ChartLineUp,
  Gear,
  CaretLeft,
  CaretRight,
} from "@phosphor-icons/react";
import { motion } from "motion/react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import { useUIStore } from "@/stores/uiStore";
import { WorkspaceSwitcher } from "./WorkspaceSwitcher";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { useReducedMotion } from "@/hooks/useReducedMotion";
import { useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import type { WorkspaceSettings } from "@/types";
import { Skeleton } from "@/components/ui/skeleton";

interface NavItem {
  to: string;
  labelKey: string;
  icon: typeof ChartPie;
}

interface NavGroup {
  labelKey: string;
  items: NavItem[];
}

const spring = { type: "spring" as const, stiffness: 420, damping: 34, mass: 0.7 };

export function Sidebar() {
  const { t } = useTranslation("layout");
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const location = useLocation();
  const { sidebarOpen, toggleSidebar, setSidebarOpen } = useUIStore();
  const isMobile = useMediaQuery("(max-width: 767px)");
  const reducedMotion = useReducedMotion();
  const [settings, setSettings] = useState<WorkspaceSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const navRef = useRef<HTMLElement>(null);

  const navGroups: NavGroup[] = [
    {
      labelKey: "sidebar.groups.workspace",
      items: [
        { to: "dashboard", labelKey: "sidebar.nav.dashboard", icon: ChartPie },
        { to: "deal-rooms", labelKey: "sidebar.nav.dealRooms", icon: FolderOpen },
        { to: "documents", labelKey: "sidebar.nav.documents", icon: FileText },
      ],
    },
    {
      labelKey: "sidebar.groups.relationships",
      items: [
        { to: "contacts", labelKey: "sidebar.nav.contacts", icon: Users },
        { to: "insights", labelKey: "sidebar.nav.insights", icon: ChartLineUp },
        { to: "agreement-documents", labelKey: "sidebar.nav.agreementDocuments", icon: Scales },
      ],
    },
    {
      labelKey: "sidebar.groups.admin",
      items: [{ to: "settings", labelKey: "sidebar.nav.settings", icon: Gear }],
    },
  ];

  useEffect(() => {
    if (isMobile) {
      setSidebarOpen(false);
    } else {
      setSidebarOpen(true);
    }
  }, [isMobile, setSidebarOpen]);

  useEffect(() => {
    if (sidebarOpen && navRef.current) {
      navRef.current.scrollTop = 0;
    }
  }, [sidebarOpen, workspaceSlug, location.pathname]);

  useEffect(() => {
    async function loadSettings() {
      try {
        setLoading(true);
        const res = await api.getWorkspaceSettings();
        setSettings(res);
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
      {isMobile && sidebarOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/40 backdrop-blur-[2px]"
          onClick={() => setSidebarOpen(false)}
          aria-hidden="true"
        />
      )}

      <aside
        className={cn(
          "fixed left-0 top-0 z-50 flex h-[100dvh] flex-col border-r border-border/80",
          "bg-card/95 supports-[backdrop-filter]:bg-card/80 supports-[backdrop-filter]:backdrop-blur-xl",
          "transition-[width,transform] duration-300 ease-[cubic-bezier(0.16,1,0.3,1)]",
          sidebarOpen ? "w-64 translate-x-0" : "w-0 -translate-x-full md:w-20 md:translate-x-0"
        )}
      >
        <div className="flex h-16 shrink-0 items-center justify-between border-b border-border/70 px-4">
          <div
            className={cn(
              "flex items-center gap-2.5 overflow-hidden transition-opacity duration-200",
              sidebarOpen ? "opacity-100" : "opacity-0 md:opacity-0"
            )}
          >
            {loading ? (
              <Skeleton className="h-8 w-8 rounded-lg" />
            ) : settings?.logoUrl ? (
              <img
                src={settings.logoUrl}
                alt={settings.name}
                className="h-8 w-8 rounded-lg object-contain ring-1 ring-border/50"
              />
            ) : (
              <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-sm font-bold text-primary-foreground shadow-sm">
                {settings?.name?.charAt(0).toUpperCase() || "D"}
              </div>
            )}
            <span className="text-h3 tracking-tight whitespace-nowrap">DealSignal</span>
          </div>
          <button
            type="button"
            onClick={toggleSidebar}
            className="flex h-8 w-8 items-center justify-center rounded-lg text-muted-foreground transition-colors duration-150 hover:bg-muted hover:text-foreground active:scale-[0.96] focus-visible:ring-2 focus-visible:ring-ring"
            aria-label={sidebarOpen ? t("sidebar.toggle.collapse") : t("sidebar.toggle.expand")}
          >
            {sidebarOpen ? <CaretLeft size={16} /> : <CaretRight size={16} />}
          </button>
        </div>

        <nav
          ref={navRef}
          className="flex-1 overflow-y-auto px-3 py-4 scrollbar-hide"
          aria-label={t("sidebar.mainNavigation")}
        >
          <div className="space-y-5">
            {navGroups.map((group) => (
              <div key={group.labelKey}>
                <div
                  className={cn(
                    "mb-2 px-2.5 text-[11px] font-medium tracking-[0.14em] text-muted-foreground/70 uppercase transition-opacity duration-200",
                    sidebarOpen ? "opacity-100" : "opacity-0 md:hidden"
                  )}
                >
                  {t(group.labelKey)}
                </div>
                <div className="rounded-xl bg-muted/25 p-1 ring-1 ring-border/40">
                  <ul className="space-y-1">
                    {group.items.map((item) => {
                      const Icon = item.icon;
                      // Create/edit/detail under /links stay highlighted on Document Library.
                      const linksUnderDocuments =
                        item.to === "documents" &&
                        !!workspaceSlug &&
                        location.pathname.startsWith(`/${workspaceSlug}/links`);
                      return (
                        <li key={item.to}>
                          <NavLink
                            to={`/${workspaceSlug}/${item.to}`}
                            title={t(item.labelKey)}
                            className={({ isActive }) =>
                              cn(
                                "group relative flex items-center gap-3 rounded-lg px-2.5 py-2.5 text-sm font-medium",
                                "transition-[color,transform] duration-200 ease-[cubic-bezier(0.16,1,0.3,1)]",
                                "active:scale-[0.985]",
                                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                                isActive || linksUnderDocuments
                                  ? "text-foreground"
                                  : "text-muted-foreground hover:text-foreground"
                              )
                            }
                          >
                            {({ isActive }) => {
                              const active = isActive || linksUnderDocuments;
                              return (
                                <>
                                  {active && (
                                    <motion.span
                                      layoutId={reducedMotion ? undefined : "workspace-nav-pill"}
                                      className={cn(
                                        "absolute inset-0 rounded-lg bg-background",
                                        "shadow-[0_1px_2px_rgba(15,23,42,0.05)] ring-1 ring-border/70"
                                      )}
                                      transition={reducedMotion ? { duration: 0 } : spring}
                                    />
                                  )}
                                  {!active && (
                                    <span className="absolute inset-0 rounded-lg transition-colors duration-200 group-hover:bg-background/55" />
                                  )}
                                  <span className="relative z-10 flex min-w-0 flex-1 items-center gap-3">
                                    <span
                                      className={cn(
                                        "flex h-8 w-8 shrink-0 items-center justify-center rounded-md transition-colors duration-200",
                                        active
                                          ? "bg-primary/10 text-primary"
                                          : "text-muted-foreground group-hover:bg-muted/70 group-hover:text-foreground"
                                      )}
                                    >
                                      <Icon size={18} weight={active ? "fill" : "regular"} />
                                    </span>
                                    <span
                                      className={cn(
                                        "truncate tracking-normal leading-none transition-opacity duration-200",
                                        sidebarOpen ? "opacity-100" : "opacity-0 md:hidden"
                                      )}
                                    >
                                      {t(item.labelKey)}
                                    </span>
                                  </span>
                                </>
                              );
                            }}
                          </NavLink>
                        </li>
                      );
                    })}
                  </ul>
                </div>
              </div>
            ))}
          </div>
        </nav>

        <div
          className={cn(
            "border-t border-border/70 p-3 transition-opacity duration-200",
            sidebarOpen ? "opacity-100" : "opacity-0 md:hidden"
          )}
        >
          <WorkspaceSwitcher />
        </div>
      </aside>
    </>
  );
}
