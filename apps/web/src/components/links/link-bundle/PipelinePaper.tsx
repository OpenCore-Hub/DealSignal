import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

/** Shared dossier shell for Documents / Security / Review pipeline steps. */
export function PipelinePaper({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "rounded-[1.65rem] p-[5px]",
        "bg-[color-mix(in_oklch,var(--muted)_72%,var(--background))]",
        "ring-1 ring-foreground/[0.06]",
        className,
      )}
    >
      <div
        className={cn(
          "overflow-hidden rounded-[calc(1.65rem-5px)] bg-background",
          "shadow-[inset_0_1px_0_rgba(255,255,255,0.72)]",
          "dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.06)]",
        )}
      >
        {children}
      </div>
    </div>
  );
}
