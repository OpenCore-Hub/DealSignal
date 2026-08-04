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

const spring = { type: "spring" as const, stiffness: 480, damping: 36, mass: 0.65 };
const easeOut = [0.16, 1, 0.3, 1] as const;

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

  let itemIndex = 0;

  return (
    <>
      {isMobile && sidebarOpen && (
        <motion.div
          className="fixed inset-0 z-40 bg-foreground/25 backdrop-blur-[3px]"
          onClick={() => setSidebarOpen(false)}
          aria-hidden="true"
          initial={reducedMotion ? false : { opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 0.2 }}
        />
      )}

      <aside
        className={cn(
          "fixed left-0 top-0 z-50 flex h-[100dvh] flex-col overflow-hidden",
          "bg-background",
          "border-r border-border/40",
          "transition-[width,transform] duration-300 ease-[cubic-bezier(0.16,1,0.3,1)]",
          sidebarOpen ? "w-64 translate-x-0" : "w-0 -translate-x-full md:w-20 md:translate-x-0",
        )}
      >
        {/* Atmospheric wash — depth without nested cards */}
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 bg-gradient-to-b from-muted/70 via-background to-background"
        />
        <div
          aria-hidden
          className="pointer-events-none absolute -left-16 top-24 h-56 w-56 rounded-full bg-foreground/[0.03] blur-3xl"
        />
        <div
          aria-hidden
          className="pointer-events-none absolute inset-y-0 right-0 w-px bg-gradient-to-b from-transparent via-border/50 to-transparent"
        />

        <div
          className={cn(
            "relative z-10 flex h-16 shrink-0 items-center",
            sidebarOpen ? "justify-between px-4" : "justify-center px-2 md:px-0",
          )}
        >
          <div
            className={cn(
              "flex min-w-0 items-center gap-2.5 overflow-hidden transition-[opacity,transform] duration-300 ease-[cubic-bezier(0.16,1,0.3,1)]",
              sidebarOpen
                ? "opacity-100 translate-x-0"
                : "pointer-events-none absolute opacity-0 -translate-x-2 md:hidden",
            )}
          >
            {loading ? (
              <Skeleton className="h-8 w-8 rounded-[0.65rem]" />
            ) : settings?.logoUrl ? (
              <img
                src={settings.logoUrl}
                alt={settings.name}
                className="h-8 w-8 rounded-[0.65rem] object-contain"
              />
            ) : (
              <div className="relative flex h-8 w-8 items-center justify-center overflow-hidden rounded-[0.65rem] bg-foreground text-[13px] font-semibold tracking-tight text-background">
                <span className="relative z-10">
                  {settings?.name?.charAt(0).toUpperCase() || "D"}
                </span>
                <span
                  aria-hidden
                  className="absolute inset-0 bg-[radial-gradient(120%_80%_at_20%_10%,rgba(255,255,255,0.22),transparent_55%)]"
                />
              </div>
            )}
            <span className="truncate text-[15px] font-semibold tracking-[-0.02em] text-foreground">
              DealSignal
            </span>
          </div>

          <button
            type="button"
            onClick={toggleSidebar}
            className={cn(
              "group/toggle relative flex h-8 w-8 shrink-0 items-center justify-center rounded-full",
              "text-muted-foreground transition-[color,background-color,transform] duration-200",
              "hover:bg-foreground/[0.06] hover:text-foreground active:scale-[0.94]",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
              !sidebarOpen && "md:mx-auto",
            )}
            aria-label={sidebarOpen ? t("sidebar.toggle.collapse") : t("sidebar.toggle.expand")}
          >
            <span className="absolute inset-0 rounded-full opacity-0 transition-opacity duration-200 group-hover/toggle:opacity-100 bg-[radial-gradient(circle_at_50%_50%,hsl(var(--foreground)/0.06),transparent_70%)]" />
            {sidebarOpen ? (
              <CaretLeft size={15} weight="bold" className="relative" />
            ) : (
              <CaretRight size={15} weight="bold" className="relative" />
            )}
          </button>
        </div>

        <nav
          ref={navRef}
          className={cn(
            "relative z-10 flex-1 overflow-y-auto",
            sidebarOpen ? "px-3 pb-4 pt-1" : "px-2 pb-4 pt-1",
          )}
          aria-label={t("sidebar.mainNavigation")}
        >
          <div className="space-y-7">
            {navGroups.map((group, groupIndex) => (
              <div key={group.labelKey}>
                <div
                  className={cn(
                    "mb-2 flex items-center gap-2 px-2.5 transition-opacity duration-200",
                    sidebarOpen ? "opacity-100" : "h-0 mb-0 overflow-hidden opacity-0 md:hidden",
                  )}
                >
                  <span className="text-[10px] font-semibold uppercase tracking-[0.16em] text-muted-foreground/65">
                    {t(group.labelKey)}
                  </span>
                  <span
                    aria-hidden
                    className="h-px flex-1 bg-gradient-to-r from-border/70 to-transparent"
                  />
                </div>

                {!sidebarOpen && groupIndex > 0 ? (
                  <div
                    aria-hidden
                    className="mx-auto mb-3 hidden h-px w-6 bg-border/60 md:block"
                  />
                ) : null}

                <ul className="space-y-0.5">
                  {group.items.map((item) => {
                    const Icon = item.icon;
                    const stagger = itemIndex++;
                    // Create/edit/detail under /links stay highlighted on Document Library.
                    const linksUnderDocuments =
                      item.to === "documents" &&
                      !!workspaceSlug &&
                      location.pathname.startsWith(`/${workspaceSlug}/links`);

                    return (
                      <li key={item.to}>
                        <motion.div
                          initial={
                            reducedMotion ? false : { opacity: 0, x: -6 }
                          }
                          animate={{ opacity: 1, x: 0 }}
                          transition={{
                            duration: 0.35,
                            delay: reducedMotion ? 0 : 0.03 * stagger,
                            ease: easeOut,
                          }}
                        >
                          <NavLink
                            to={`/${workspaceSlug}/${item.to}`}
                            title={t(item.labelKey)}
                            onClick={() => {
                              if (isMobile) setSidebarOpen(false);
                            }}
                            className={({ isActive }) =>
                              cn(
                                "group relative flex items-center rounded-lg text-[13px] font-medium",
                                "transition-[color,transform] duration-200 ease-[cubic-bezier(0.16,1,0.3,1)]",
                                "active:scale-[0.985]",
                                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
                                sidebarOpen ? "gap-3 px-2.5 py-2" : "justify-center px-0 py-2.5",
                                isActive || linksUnderDocuments
                                  ? "text-foreground"
                                  : "text-muted-foreground hover:text-foreground",
                              )
                            }
                          >
                            {({ isActive }) => {
                              const active = isActive || linksUnderDocuments;
                              return (
                                <>
                                  {active ? (
                                    <motion.span
                                      layoutId={
                                        reducedMotion ? undefined : "workspace-nav-pill"
                                      }
                                      className={cn(
                                        "absolute inset-0 rounded-lg",
                                        "bg-foreground/[0.055]",
                                        "shadow-[inset_0_1px_0_rgba(255,255,255,0.65)]",
                                        "dark:bg-foreground/[0.1] dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.05)]",
                                      )}
                                      transition={
                                        reducedMotion ? { duration: 0 } : spring
                                      }
                                    />
                                  ) : null}

                                  {!active ? (
                                    <span className="absolute inset-0 rounded-lg bg-transparent transition-colors duration-200 group-hover:bg-foreground/[0.035]" />
                                  ) : null}

                                  {active ? (
                                    <motion.span
                                      layoutId={
                                        reducedMotion
                                          ? undefined
                                          : "workspace-nav-rail"
                                      }
                                      aria-hidden
                                      className="absolute left-0 top-1/2 h-4 w-[2px] -translate-y-1/2 rounded-full bg-foreground/80"
                                      transition={
                                        reducedMotion ? { duration: 0 } : spring
                                      }
                                    />
                                  ) : null}

                                  <span
                                    className={cn(
                                      "relative z-10 flex min-w-0 items-center",
                                      sidebarOpen ? "flex-1 gap-3" : "justify-center",
                                    )}
                                  >
                                    <span
                                      className={cn(
                                        "relative flex shrink-0 items-center justify-center transition-transform duration-200 ease-[cubic-bezier(0.16,1,0.3,1)]",
                                        "group-hover:scale-[1.06] group-active:scale-95",
                                        active
                                          ? "text-foreground"
                                          : "text-muted-foreground group-hover:text-foreground",
                                      )}
                                    >
                                      <Icon
                                        size={19}
                                        weight={active ? "duotone" : "regular"}
                                      />
                                    </span>
                                    <span
                                      className={cn(
                                        "truncate tracking-[-0.01em] transition-[opacity,transform] duration-200",
                                        sidebarOpen
                                          ? "translate-x-0 opacity-100"
                                          : "pointer-events-none absolute opacity-0 md:hidden",
                                      )}
                                    >
                                      {t(item.labelKey)}
                                    </span>
                                  </span>
                                </>
                              );
                            }}
                          </NavLink>
                        </motion.div>
                      </li>
                    );
                  })}
                </ul>
              </div>
            ))}
          </div>
        </nav>

        <div
          className={cn(
            "relative z-10 shrink-0 p-3 pt-2 transition-[opacity,transform] duration-200",
            sidebarOpen
              ? "opacity-100 translate-y-0"
              : "pointer-events-none opacity-0 translate-y-1 md:hidden",
          )}
        >
          <div
            aria-hidden
            className="mb-2 h-px bg-gradient-to-r from-transparent via-border/70 to-transparent"
          />
          <div className="rounded-xl transition-colors duration-200 hover:bg-foreground/[0.03]">
            <WorkspaceSwitcher />
          </div>
        </div>
      </aside>
    </>
  );
}
