import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface EmptyStateProps {
  icon: ReactNode;
  title: string;
  description: string;
  action?: {
    label: string;
    onClick: () => void;
  };
  className?: string;
  size?: "default" | "large";
}

export function EmptyState({
  icon,
  title,
  description,
  action,
  className,
  size = "default",
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center rounded-lg border border-dashed border-border bg-muted/30 px-6 py-12 text-center",
        size === "large" && "py-16",
        className
      )}
    >
      <div
        className={cn(
          "mb-4 text-muted-foreground",
          size === "large" ? "[&_svg]:size-16" : "[&_svg]:size-12"
        )}
      >
        {icon}
      </div>
      <h3 className={cn("text-h3", size === "large" && "text-h2")}>{title}</h3>
      <p className="mt-1 max-w-sm text-body text-muted-foreground">{description}</p>
      {action && (
        <Button className="mt-5" onClick={action.onClick}>
          {action.label}
        </Button>
      )}
    </div>
  );
}
