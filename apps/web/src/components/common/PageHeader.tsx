import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

interface PageHeaderProps {
  title: string;
  description?: string;
  children?: ReactNode;
  className?: string;
}

export function PageHeader({ title, description, children, className }: PageHeaderProps) {
  return (
    <div
      className={cn(
        "flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between lg:gap-6",
        className,
      )}
    >
      <div className="min-w-0 flex-1 space-y-2">
        <h1 className="text-h1">{title}</h1>
        {description && <p className="text-body text-muted-foreground">{description}</p>}
      </div>
      {children ? (
        <div className="flex w-full shrink-0 flex-wrap items-center gap-2 lg:w-auto lg:justify-end">
          {children}
        </div>
      ) : null}
    </div>
  );
}
