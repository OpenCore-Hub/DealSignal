import type { Evidence } from "@/types";
import { cn } from "@/lib/utils";

interface HighlightOverlayProps {
  evidences: Evidence[];
  className?: string;
}

export function HighlightOverlay({ evidences, className }: HighlightOverlayProps) {
  if (evidences.length === 0) return null;

  return (
    <div className={cn("pointer-events-none absolute inset-0", className)} aria-hidden="true">
      {evidences.map((ev) => (
        <div
          key={ev.id}
          className="absolute animate-pulse rounded-sm border border-primary/60 bg-primary/20"
          style={{
            left: `${ev.bbox.x * 100}%`,
            top: `${ev.bbox.y * 100}%`,
            width: `${ev.bbox.w * 100}%`,
            height: `${ev.bbox.h * 100}%`,
          }}
          title={ev.text}
        />
      ))}
    </div>
  );
}
