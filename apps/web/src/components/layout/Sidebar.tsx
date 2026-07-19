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
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import { useUIStore } from "@/stores/uiStore";
import { WorkspaceSwitcher } from "./WorkspaceSwitcher";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import type { WorkspaceSettings } from "@/types";
import { Skeleton } from "@/components/ui/skeleton";
import { useDealRoomTab } from "@/hooks/useDealRoomTab";
import type { DealRoomTab } from "@/hooks/useDealRoomTab";

interface NavItem {
  to: string;
  labelKey: string;
  icon: typeof ChartPie;
}

interface NavGroup {
  labelKey: string;
  items: NavItem[];
}

export function Sidebar() {
  const { t } = useTranslation("layout");
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const { sidebarOpen, toggleSidebar, setSidebarOpen } = useUIStore();
  const isMobile = useMediaQuery("(max-width: 767px)");
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

  const dealRoomItems: { value: DealRoomTab; labelKey: string; icon: typeof Files }[] = [
    { value: "documents", labelKey: "dealRooms:tabs.documents", icon: Files },
    { value: "participants", labelKey: "dealRooms:tabs.participants", icon: ShareNetwork },
    { value: "qa", labelKey: "dealRooms:tabs.qa", icon: ChatCircleText },
    { value: "activity", labelKey: "dealRooms:tabs.activity", icon: Clock },
    { value: "analytics", labelKey: "dealRooms:tabs.analytics", icon: ChartLineUp },
    { value: "settings", labelKey: "dealRooms:tabs.settings", icon: Gear },
  ];

  useEffect(() => {
    if (isMobile) {
      setSidebarOpen(false);
    } else {
      setSidebarOpen(true);
    }
  }, [isMobile, setSidebarOpen]);

  // Reset the navigation scroll position whenever the sidebar opens or the
  // route changes. This prevents the browser's scroll restoration or prior
  // user scrolling from hiding the top navigation items behind the fold.
  useEffect(() => {
    if (sidebarOpen && navRef.current) {
      navRef.current.scrollTop = 0;
    }
  }, [sidebarOpen, workspaceSlug]);

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
          "fixed left-0 top-0 z-50 flex h-[100dvh] flex-col border-r border-border bg-card transition-all duration-300 ease-[cubic-bezier(0.16,1,0.3,1)]",
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
            aria-label={sidebarOpen ? t("sidebar.toggle.collapse") : t("sidebar.toggle.expand")}
          >
            {sidebarOpen ? <CaretLeft size={16} /> : <CaretRight size={16} />}
          </button>
        </div>

        {/* Navigation */}
        <nav ref={navRef} className="flex-1 overflow-y-auto p-3" aria-label={t("sidebar.mainNavigation")}>
          <div className="space-y-5">
            {isDealRoom ? (
              <div>
                <ul className="space-y-1">
                  {dealRoomItems.map((item) => {
                    const Icon = item.icon;
                    const isActive = tab === item.value;
                    return (
                      <li key={item.value}>
                        <button
                          type="button"
                          onClick={() => setTab(item.value)}
                          title={t(item.labelKey)}
                          className={cn(
                            "group relative flex w-full items-center gap-3 rounded-md px-3 py-2.5 text-left text-sm font-medium transition-all duration-200 ease-[cubic-bezier(0.16,1,0.3,1)]",
                            isActive
                              ? "bg-primary/10 text-primary"
                              : "text-muted-foreground hover:bg-muted hover:text-foreground"
                          )}
                        >
                          <span
                            className={cn(
                              "absolute left-0 top-1/2 h-6 w-1 -translate-y-1/2 rounded-r-full bg-primary transition-opacity",
                              isActive ? "opacity-100" : "opacity-0"
                            )}
                          />
                          <Icon size={20} weight={isActive ? "fill" : "regular"} />
                          <span
                            className={cn(
                              "whitespace-nowrap transition-opacity",
                              sidebarOpen ? "opacity-100" : "opacity-0 md:hidden"
                            )}
                          >
                            {t(item.labelKey)}
                          </span>
                        </button>
                      </li>
                    );
                  })}
                </ul>
              </div>
            ) : (
              navGroups.map((group) => (
                <div key={group.labelKey}>
                  <div
                    className={cn(
                      "mb-2 px-3 text-xs font-medium uppercase tracking-wider text-muted-foreground transition-opacity",
                      sidebarOpen ? "opacity-100" : "opacity-0 md:hidden"
                    )}
                  >
                    {t(group.labelKey)}
                  </div>
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
                                "group relative flex items-center gap-3 rounded-md px-3 py-2.5 text-sm font-medium transition-all duration-200 ease-[cubic-bezier(0.16,1,0.3,1)]",
                                isActive
                                  ? "bg-primary/10 text-primary"
                                  : "text-muted-foreground hover:bg-muted hover:text-foreground"
                              )
                            }
                          >
                            {({ isActive }) => (
                              <>
                                <span
                                  className={cn(
                                    "absolute left-0 top-1/2 h-6 w-1 -translate-y-1/2 rounded-r-full bg-primary transition-opacity",
                                    isActive ? "opacity-100" : "opacity-0"
                                  )}
                                />
                                <Icon size={20} weight={isActive ? "fill" : "regular"} />
                                <span
                                  className={cn(
                                    "whitespace-nowrap transition-opacity",
                                    sidebarOpen ? "opacity-100" : "opacity-0 md:hidden"
                                  )}
                                >
                                  {t(item.labelKey)}
                                </span>
                              </>
                            )}
                          </NavLink>
                        </li>
                      );
                    })}
                  </ul>
                </div>
              ))
            )}
          </div>
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
