import { useEffect, useState } from "react";
import { useParams } from "react-router";
import { List, Bell, UploadSimple, SignOut, Gear } from "@phosphor-icons/react";
import { ThemeToggle } from "@/components/common/ThemeToggle";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
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
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const { toggleSidebar, setUploadDialogOpen } = useUIStore();
  const [settings, setSettings] = useState<WorkspaceSettings | null>(null);

  useEffect(() => {
    async function loadSettings() {
      try {
        const res = await api.getWorkspaceSettings();
        setSettings(res.data);
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
    <header className="sticky top-0 z-30 flex h-16 items-center gap-4 border-b border-border bg-background/95 px-4 backdrop-blur supports-[backdrop-filter]:bg-background/60 md:px-6">
      {/* Mobile menu toggle */}
      <button
        onClick={toggleSidebar}
        className={cn(
          "flex h-9 w-9 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring md:hidden"
        )}
        aria-label="切换侧边栏"
      >
        <List size={20} />
      </button>

      {/* Workspace breadcrumb on mobile when sidebar collapsed */}
      <div className="md:hidden">
        <span className="text-h3">DealSignal</span>
      </div>

      {/* Right actions */}
      <div className="ml-auto flex items-center gap-2">
        <Button
          size="sm"
          className="hidden gap-1.5 md:inline-flex"
          onClick={() => setUploadDialogOpen(true)}
        >
          <UploadSimple size={16} weight="bold" />
          上传文档
        </Button>

        <ThemeToggle />

        <Button
          size="icon"
          variant="ghost"
          aria-label="通知"
          disabled
          title="通知中心即将上线"
        >
          <Bell size={20} />
        </Button>

        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <button
                className="flex h-8 w-8 items-center justify-center rounded-full bg-primary text-primary-foreground text-sm font-medium focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
                aria-label="账户菜单"
              >
                {avatarLabel}
              </button>
            }
          />
          <DropdownMenuContent align="end" className="w-48">
            <DropdownMenuLabel className="font-normal">
              <div className="flex flex-col">
                <span className="text-sm font-medium">{settings?.name ?? workspaceSlug ?? "工作区"}</span>
                <span className="text-caption text-muted-foreground">账户菜单</span>
              </div>
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem disabled title="账户设置需后端支持">
              <Gear size={16} className="mr-2" />
              账户设置
            </DropdownMenuItem>
            <DropdownMenuItem disabled title="登出需后端支持">
              <SignOut size={16} className="mr-2" />
              登出
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  );
}
