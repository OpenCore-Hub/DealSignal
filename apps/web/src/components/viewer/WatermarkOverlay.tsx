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

export function WatermarkOverlay({ watermark, tiled = true, className }: WatermarkOverlayProps) {
  if (!watermark) return null;

  const text = [watermark.email, watermark.ip, watermark.viewedAt].filter(Boolean).join(" · ") || "CONFIDENTIAL";

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
