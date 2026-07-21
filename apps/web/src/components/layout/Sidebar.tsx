import { NavLink, useParams, useMatch } from "react-router";
import {
  ChartPie,
  FileText,
  Scales,
  Link as LinkIcon,
  FolderOpen,
  Users,
  ChartLineUp,
  Gear,
  CaretLeft,
  CaretRight,
  Files,
  ChatCircleText,
  Clock,
  ShareNetwork,
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
import { useDealRoomTab } from "@/hooks/useDealRoomTab";
import type { DealRoomTab } from "@/hooks/useDealRoomTab";
import { DEAL_ROOM_TAB_ROLE } from "@/lib/dealRoomNav";
import { badgeCountForTab, useDealRoomNavStore } from "@/stores/dealRoomNavStore";

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
  const { t: td } = useTranslation("dealRooms");
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
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
        { to: "links", labelKey: "sidebar.nav.links", icon: LinkIcon },
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

  const isDealRoom = useMatch("/:workspaceSlug/deal-rooms/:roomId");
  const { tab, setTab } = useDealRoomTab();
  const navSignals = useDealRoomNavStore();

  const dealRoomItems: { value: DealRoomTab; labelKey: string; icon: typeof Files }[] = [
    { value: "documents" as const, labelKey: "dealRooms:tabs.documents", icon: Files },
    { value: "participants" as const, labelKey: "dealRooms:tabs.participants", icon: ShareNetwork },
    { value: "qa" as const, labelKey: "dealRooms:tabs.qa", icon: ChatCircleText },
    { value: "activity" as const, labelKey: "dealRooms:tabs.activity", icon: Clock },
    { value: "analytics" as const, labelKey: "dealRooms:tabs.analytics", icon: ChartLineUp },
    { value: "settings" as const, labelKey: "dealRooms:tabs.settings", icon: Gear },
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
  }, [sidebarOpen, workspaceSlug, isDealRoom]);

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
            {isDealRoom ? (
              <div className="space-y-3">
                {sidebarOpen && (
                  <div className="space-y-2 px-1">
                    <NavLink
                      to={`/${workspaceSlug}/deal-rooms`}
                      className="group inline-flex items-center gap-1 text-[11px] font-medium tracking-wide text-muted-foreground transition-colors duration-150 hover:text-foreground"
                    >
                      <CaretLeft
                        size={12}
                        className="transition-transform duration-200 ease-[cubic-bezier(0.16,1,0.3,1)] group-hover:-translate-x-0.5"
                      />
                      {td("sidebar.backToRooms")}
                    </NavLink>
                    <p className="text-[11px] font-medium tracking-[0.14em] text-muted-foreground/70 uppercase">
                      {td("sidebar.label")}
                    </p>
                  </div>
                )}

                <div
                  className={cn(
                    "rounded-xl p-1.5",
                    "bg-muted/35 ring-1 ring-border/50",
                    "shadow-[inset_0_1px_0_0_rgba(255,255,255,0.55)] dark:shadow-[inset_0_1px_0_0_rgba(255,255,255,0.04)]"
                  )}
                >
                  <ul className="space-y-1" aria-label={td("sidebar.label")}>
                    {dealRoomItems.map((item, index) => {
                      const Icon = item.icon;
                      const isActive = tab === item.value;
                      const badge = badgeCountForTab(item.value, navSignals);
                      return (
                        <motion.li
                          key={item.value}
                          initial={reducedMotion ? false : { opacity: 0, x: -6 }}
                          animate={{ opacity: 1, x: 0 }}
                          transition={
                            reducedMotion
                              ? { duration: 0 }
                              : { ...spring, delay: index * 0.035 }
                          }
                        >
                          <button
                            type="button"
                            onClick={() => setTab(item.value)}
                            title={t(item.labelKey)}
                            data-role={DEAL_ROOM_TAB_ROLE[item.value]}
                            className={cn(
                              "group relative flex w-full items-center gap-3 rounded-lg px-2.5 py-2.5 text-left text-sm font-medium",
                              "transition-[color,transform] duration-200 ease-[cubic-bezier(0.16,1,0.3,1)]",
                              "active:scale-[0.985]",
                              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-card",
                              isActive
                                ? "text-foreground"
                                : "text-muted-foreground hover:text-foreground"
                            )}
                          >
                            {isActive && (
                              <motion.span
                                layoutId={reducedMotion ? undefined : "deal-room-nav-pill"}
                                className={cn(
                                  "absolute inset-0 rounded-lg bg-background",
                                  "shadow-[0_1px_2px_rgba(15,23,42,0.05)] ring-1 ring-border/70"
                                )}
                                transition={reducedMotion ? { duration: 0 } : spring}
                              />
                            )}
                            {!isActive && (
                              <span className="absolute inset-0 rounded-lg bg-transparent transition-colors duration-200 group-hover:bg-background/55" />
                            )}

                            <span className="relative z-10 flex min-w-0 flex-1 items-center gap-3">
                              <span
                                className={cn(
                                  "flex h-8 w-8 shrink-0 items-center justify-center rounded-md transition-colors duration-200",
                                  isActive
                                    ? "bg-primary/10 text-primary"
                                    : "bg-transparent text-muted-foreground group-hover:bg-muted/70 group-hover:text-foreground"
                                )}
                              >
                                <Icon size={18} weight={isActive ? "fill" : "regular"} />
                              </span>
                              <span
                                className={cn(
                                  "min-w-0 flex-1 truncate tracking-[0.35em] leading-none transition-opacity duration-200",
                                  sidebarOpen ? "opacity-100" : "opacity-0 md:hidden"
                                )}
                              >
                                {t(item.labelKey)}
                              </span>
                              {badge > 0 && sidebarOpen && (
                                <motion.span
                                  initial={reducedMotion ? false : { scale: 0.7, opacity: 0 }}
                                  animate={{ scale: 1, opacity: 1 }}
                                  transition={reducedMotion ? { duration: 0 } : spring}
                                  className="ml-auto inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-destructive px-1.5 text-[11px] font-semibold text-destructive-foreground"
                                  data-testid={`deal-room-nav-badge-${item.value}`}
                                >
                                  {badge > 99 ? "99+" : badge}
                                </motion.span>
                              )}
                              {badge > 0 && !sidebarOpen && (
                                <span
                                  className="absolute top-1.5 right-1.5 h-2 w-2 rounded-full bg-destructive ring-2 ring-card"
                                  aria-hidden
                                />
                              )}
                            </span>
                          </button>
                        </motion.li>
                      );
                    })}
                  </ul>
                </div>
              </div>
            ) : (
              navGroups.map((group) => (
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
                                  isActive
                                    ? "text-foreground"
                                    : "text-muted-foreground hover:text-foreground"
                                )
                              }
                            >
                              {({ isActive }) => (
                                <>
                                  {isActive && (
                                    <motion.span
                                      layoutId={reducedMotion ? undefined : "workspace-nav-pill"}
                                      className={cn(
                                        "absolute inset-0 rounded-lg bg-background",
                                        "shadow-[0_1px_2px_rgba(15,23,42,0.05)] ring-1 ring-border/70"
                                      )}
                                      transition={reducedMotion ? { duration: 0 } : spring}
                                    />
                                  )}
                                  {!isActive && (
                                    <span className="absolute inset-0 rounded-lg transition-colors duration-200 group-hover:bg-background/55" />
                                  )}
                                  <span className="relative z-10 flex min-w-0 flex-1 items-center gap-3">
                                    <span
                                      className={cn(
                                        "flex h-8 w-8 shrink-0 items-center justify-center rounded-md transition-colors duration-200",
                                        isActive
                                          ? "bg-primary/10 text-primary"
                                          : "text-muted-foreground group-hover:bg-muted/70 group-hover:text-foreground"
                                      )}
                                    >
                                      <Icon size={18} weight={isActive ? "fill" : "regular"} />
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
                              )}
                            </NavLink>
                          </li>
                        );
                      })}
                    </ul>
                  </div>
                </div>
              ))
            )}
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
