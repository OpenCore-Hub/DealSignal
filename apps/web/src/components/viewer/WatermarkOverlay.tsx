import { useState } from "react";
import { cn } from "@/lib/utils";

export interface WatermarkInfo {
  email?: string;
  ip?: string;
  viewedAt?: string;
}

interface WatermarkOverlayProps {
  watermark?: WatermarkInfo;
  tiled?: boolean;
  className?: string;
}

function formatTimestamp(date: Date): string {
  const y = date.getFullYear();
  const mo = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  const h = String(date.getHours()).padStart(2, "0");
  const mi = String(date.getMinutes()).padStart(2, "0");
  const s = String(date.getSeconds()).padStart(2, "0");
  return `${y}-${mo}-${d} ${h}:${mi}:${s}`;
}

export function WatermarkOverlay({ watermark, tiled = true, className }: WatermarkOverlayProps) {
  // Capture the mount-time timestamp once (ms precision) and never update.
  const [mountedAt] = useState(() => formatTimestamp(new Date()));

  if (!watermark) return null;

  const text = [watermark.email, mountedAt].filter(Boolean).join(" · ") || "CONFIDENTIAL";

  if (tiled) {
    return (
      <div
        className={cn(
          "pointer-events-none absolute inset-0 flex rotate-[-30deg] flex-wrap items-center justify-center gap-16 overflow-hidden opacity-[0.08]",
          className
        )}
        aria-hidden="true"
      >
        {Array.from({ length: 12 }).map((_, i) => (
          <span key={i} className="whitespace-nowrap text-2xl font-bold text-foreground">
            {text}
          </span>
        ))}
      </div>
    );
  }

  return (
    <div
      className={cn(
        "pointer-events-none absolute right-4 bottom-4 rotate-[-30deg] whitespace-nowrap text-xl font-bold text-foreground opacity-10",
        className
      )}
      aria-hidden="true"
    >
      {text}
    </div>
  );
}
