import { useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { Sidebar } from "./Sidebar";
import { TopNav } from "./TopNav";
import { UploadDialog } from "@/components/upload/UploadDialog";
import { PageTransition } from "@/components/common/PageTransition";
import { useUIStore } from "@/stores/uiStore";
import { useWorkspaceAccess } from "@/hooks/useWorkspaceAccess";
import { cn } from "@/lib/utils";

interface AppShellProps {
  children: React.ReactNode;
}

export function AppShell({ children }: AppShellProps) {
  const { sidebarOpen } = useUIStore();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const { isGuest, canWrite, loading } = useWorkspaceAccess(workspaceSlug);
  const { t } = useTranslation("common");

  return (
    <div className="flex h-[100dvh] overflow-hidden bg-background">
      <Sidebar />
      <div
        className={cn(
          "flex h-[100dvh] flex-1 flex-col overflow-hidden transition-[padding] duration-300 ease-[cubic-bezier(0.16,1,0.3,1)]",
          sidebarOpen ? "md:pl-64" : "md:pl-20"
        )}
      >
        <TopNav />
        {!loading && isGuest ? (
          <div
            className="border-b border-border/60 bg-muted/40 px-6 py-2 text-sm text-muted-foreground md:px-8"
            data-testid="workspace-read-only-banner"
            role="status"
          >
            {t("readOnlyBanner")}
          </div>
        ) : null}
        <main className="flex-1 overflow-auto p-6 md:p-8">
          <div className="h-full w-full">
            <PageTransition>{children}</PageTransition>
          </div>
        </main>
      </div>
      {canWrite ? <UploadDialog /> : null}
    </div>
  );
}
