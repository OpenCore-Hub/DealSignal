import { Sidebar } from "./Sidebar";
import { TopNav } from "./TopNav";
import { AIAssistant } from "@/components/ai/AIAssistant";
import { UploadDialog } from "@/components/upload/UploadDialog";
import { PageTransition } from "@/components/common/PageTransition";
import { useUIStore } from "@/stores/uiStore";
import { cn } from "@/lib/utils";

interface AppShellProps {
  children: React.ReactNode;
}

export function AppShell({ children }: AppShellProps) {
  const { sidebarOpen } = useUIStore();

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
        <main className="flex-1 overflow-auto p-6 md:p-8">
          <div className="mx-auto h-full max-w-[1400px]">
            <PageTransition>{children}</PageTransition>
          </div>
        </main>
      </div>
      <AIAssistant />
      <UploadDialog />
    </div>
  );
}
