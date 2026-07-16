import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { motion } from "motion/react";
import { useReducedMotion } from "@/hooks/useReducedMotion";

interface EmptyStateProps {
  icon: ReactNode;
  title: string;
  description: string;
  action?: {
    label: string;
    onClick: () => void;
  };
  className?: string;
  size?: "default" | "large" | "compact";
}

export function EmptyState({
  icon,
  title,
  description,
  action,
  className,
  size = "default",
}: EmptyStateProps) {
  const reducedMotion = useReducedMotion();

  return (
    <motion.div
      initial={reducedMotion ? false : { opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.4, ease: [0.16, 1, 0.3, 1] }}
      className={cn(
        "flex flex-col items-center justify-center rounded-xl bg-muted/30 px-6 py-10 text-center",
        size === "large" && "py-14",
        size === "compact" && "py-6",
        className
      )}
    >
      <div
        className={cn(
          "mb-3 text-muted-foreground",
          size === "large" ? "[&_svg]:size-12" : size === "compact" ? "[&_svg]:size-8" : "[&_svg]:size-10"
        )}
      >
        {icon}
      </div>
      <h3
        className={cn(
          size === "compact" ? "text-body font-medium" : "text-h3",
          size === "large" && "text-h2"
        )}
      >
        {title}
      </h3>
      <p
        className={cn(
          "mt-1 max-w-sm text-muted-foreground",
          size === "compact" ? "text-caption" : "text-body"
        )}
      >
        {description}
      </p>
      {action && (
        <Button className={cn("mt-4", size === "compact" && "h-8 px-3 text-xs")} onClick={action.onClick}>
          {action.label}
        </Button>
      )}
    </motion.div>
  );
}
