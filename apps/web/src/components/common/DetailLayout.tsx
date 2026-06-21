import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

interface DetailLayoutProps {
  children: ReactNode;
  sidebar?: ReactNode;
  className?: string;
}

export function DetailLayout({ children, sidebar, className }: DetailLayoutProps) {
  return (
    <div
      className={cn(
        "grid grid-cols-1 gap-6",
        sidebar ? "lg:grid-cols-[1fr_320px]" : "",
        className
      )}
    >
      <div className="min-w-0">{children}</div>
      {sidebar && (
        <aside className="space-y-6 lg:sticky lg:top-6 lg:self-start">{sidebar}</aside>
      )}
    </div>
  );
}
