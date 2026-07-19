import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { List, Bell, SignOut, Gear, CaretRight } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { ThemeToggle } from "@/components/common/ThemeToggle";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useUIStore } from "@/stores/uiStore";
import { cn } from "@/lib/utils";
import { api } from "@/lib/api";
import type { WorkspaceSettings } from "@/types";

export function TopNav() {
  const { t } = useTranslation("layout");
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const { toggleSidebar, reset: resetUI, breadcrumbs } = useUIStore();
  const [settings, setSettings] = useState<WorkspaceSettings | null>(null);

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

      {/* Workspace breadcrumb on mobile when sidebar collapsed */}
      <div className="md:hidden">
        <span className="text-h3">DealSignal</span>
      </div>

      {/* Page breadcrumb */}
      {breadcrumbs.length > 0 && (
        <nav
          aria-label={t("topNav.breadcrumb")}
          className="hidden items-center gap-1.5 text-sm md:inline-flex"
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

      {/* Right actions */}
      <div className="ml-auto flex items-center gap-2">
        <ThemeToggle />

        <Button
          size="icon"
          variant="ghost"
          aria-label={t("topNav.notifications.title")}
          disabled
          title={t("topNav.notifications.comingSoon")}
        >
          <Bell size={20} />
        </Button>

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
