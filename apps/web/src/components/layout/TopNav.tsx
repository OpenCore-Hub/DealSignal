import { useEffect, useMemo, useState } from "react";
import { Link, useLocation, useMatch, useNavigate, useParams } from "react-router";
import { List, SignOut, Gear, CaretRight } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { ThemeToggle } from "@/components/common/ThemeToggle";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { DashboardHeader } from "@/components/dashboard/DashboardHeader";
import { resolveWorkspaceNavBreadcrumbs } from "@/lib/workspaceBreadcrumbs";
import { useUIStore } from "@/stores/uiStore";
import { cn } from "@/lib/utils";
import { api } from "@/lib/api";
import type { WorkspaceSettings } from "@/types";

export function TopNav() {
  const { t } = useTranslation("layout");
  const { t: tc } = useTranslation("common");
  const navigate = useNavigate();
  const location = useLocation();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const isDashboard = Boolean(useMatch("/:workspaceSlug/dashboard"));
  const { toggleSidebar, reset: resetUI, breadcrumbTail } = useUIStore();
  const [settings, setSettings] = useState<WorkspaceSettings | null>(null);

  const breadcrumbs = useMemo(() => {
    if (!workspaceSlug || isDashboard) return [];
    const base = resolveWorkspaceNavBreadcrumbs(location.pathname, workspaceSlug, {
      home: tc("home"),
      section: (navKey) => t(navKey),
    });
    if (base.length === 0) return [];
    // Last base crumb is the section root; drop its link when it is the current page.
    const items = base.map((item, index) => {
      if (index === base.length - 1 && !breadcrumbTail) {
        return { label: item.label };
      }
      return item;
    });
    return breadcrumbTail ? [...items, breadcrumbTail] : items;
  }, [workspaceSlug, isDashboard, location.pathname, breadcrumbTail, t, tc]);

  useEffect(() => {
    async function loadSettings() {
      try {
        const res = await api.getWorkspaceSettings();
        setSettings(res);
      } catch {
        // Keep fallback avatar on error
      }
    }
    loadSettings();
  }, []);

  const avatarLabel = settings?.name?.charAt(0).toUpperCase()
    ?? workspaceSlug?.charAt(0).toUpperCase()
    ?? "D";

  return (
    <header className="sticky top-0 z-30 flex h-16 items-center gap-4 border-b border-border bg-background/80 px-4 shadow-[0_1px_3px_rgba(15,23,42,0.03)] backdrop-blur-xl supports-[backdrop-filter]:bg-background/50 md:px-6">
      {/* Mobile menu toggle */}
      <button
        onClick={toggleSidebar}
        className={cn(
          "flex h-9 w-9 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring md:hidden"
        )}
        aria-label={t("topNav.toggleSidebar")}
      >
        <List size={20} />
      </button>

      {/* Dashboard welcome in the global header; breadcrumbs on other pages. */}
      {isDashboard && workspaceSlug ? (
        <div className="min-w-0 flex-1">
          <DashboardHeader workspaceSlug={workspaceSlug} />
        </div>
      ) : (
        <>
          <div className="md:hidden">
            <span className="text-h3">DealSignal</span>
          </div>

          {breadcrumbs.length > 0 && (
            <nav
              aria-label={t("topNav.breadcrumb")}
              className="hidden min-w-0 flex-1 items-center gap-1.5 text-sm md:inline-flex"
              data-testid="workspace-breadcrumbs"
            >
              {breadcrumbs.map((item, index) => {
                const isLast = index === breadcrumbs.length - 1;
                return (
                  <span key={`${item.label}-${index}`} className="flex items-center gap-1.5">
                    {item.to ? (
                      <Link
                        to={item.to}
                        className="text-muted-foreground/80 transition-colors duration-200 hover:text-foreground"
                      >
                        {item.label}
                      </Link>
                    ) : (
                      <span className="font-semibold text-foreground">{item.label}</span>
                    )}
                    {!isLast && (
                      <CaretRight weight="regular" className="text-muted-foreground/40" size={14} />
                    )}
                  </span>
                );
              })}
            </nav>
          )}
        </>
      )}

      {/* Right actions — notifications control omitted until a real inbox ships */}
      <div className="ml-auto flex items-center gap-2">
        <ThemeToggle />

        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <button
                className="flex h-8 w-8 items-center justify-center rounded-full bg-primary text-primary-foreground text-sm font-medium focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
                aria-label={t("topNav.account.menu")}
              >
                {avatarLabel}
              </button>
            }
          />
          <DropdownMenuContent align="end" className="w-48">
            <DropdownMenuGroup>
              <DropdownMenuLabel className="font-normal">
                <div className="flex flex-col">
                  <span className="text-sm font-medium">{settings?.name ?? workspaceSlug ?? t("topNav.workspace.fallback")}</span>
                  <span className="text-caption text-muted-foreground">{t("topNav.account.menu")}</span>
                </div>
              </DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuItem disabled title={t("topNav.account.settingsComingSoon")}>
                <Gear size={16} className="mr-2" />
                {t("topNav.account.settings")}
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={async () => {
                  await api.logout().catch(() => {});
                  resetUI();
                  navigate("/login", { replace: true });
                }}
              >
                <SignOut size={16} className="mr-2" />
                {t("topNav.account.logout")}
              </DropdownMenuItem>
            </DropdownMenuGroup>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  );
}
