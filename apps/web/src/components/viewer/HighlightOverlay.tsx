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
      {evidences.map((ev) =>
        ev.boxes.map((box, index) => (
          <div
            key={`${ev.chunk_id}-${index}`}
            className="absolute animate-pulse rounded-sm border border-primary/60 bg-primary/20"
            style={{
              left: `${box.x * 100}%`,
              top: `${box.y * 100}%`,
              width: `${box.w * 100}%`,
              height: `${box.h * 100}%`,
            }}
            title={ev.quote}
          />
        ))
      )}
    </div>
  );
}
