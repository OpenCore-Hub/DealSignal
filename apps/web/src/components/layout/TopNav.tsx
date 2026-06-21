import { List, MagnifyingGlass, Bell, UploadSimple } from "@phosphor-icons/react";
import { ThemeToggle } from "@/components/common/ThemeToggle";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useUIStore } from "@/stores/uiStore";
import { cn } from "@/lib/utils";

export function TopNav() {
  const { toggleSidebar, setUploadDialogOpen } = useUIStore();

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

      {/* Search */}
      <div className="hidden flex-1 md:block md:max-w-md">
        <div className="relative">
          <MagnifyingGlass
            size={18}
            className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"
          />
          <Input
            type="search"
            placeholder="搜索文档、链接或访客..."
            className="pl-9"
            aria-label="全局搜索"
          />
        </div>
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
          className="relative"
          aria-label="通知"
        >
          <Bell size={20} />
          <span className="absolute right-2 top-2 h-2 w-2 rounded-full bg-hot-500" />
        </Button>

        <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary text-primary-foreground text-sm font-medium">
          JD
        </div>
      </div>
    </header>
  );
}
