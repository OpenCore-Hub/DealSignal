import { Sidebar } from "./Sidebar";
import { TopNav } from "./TopNav";
import { AIAssistant } from "@/components/ai/AIAssistant";

interface AppShellProps {
  children: React.ReactNode;
}

export function AppShell({ children }: AppShellProps) {
  return (
    <div className="flex min-h-[100dvh] bg-background">
      <Sidebar />
      <div className="flex h-[100dvh] flex-1 flex-col overflow-hidden">
        <TopNav />
        <main className="flex-1 overflow-auto p-6 md:p-8">
          <div className="mx-auto max-w-[1400px]">{children}</div>
        </main>
      </div>
      <AIAssistant />
    </div>
  );
}
