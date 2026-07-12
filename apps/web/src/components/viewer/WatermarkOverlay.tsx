import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";

export interface WatermarkInfo {
  email?: string;
  ip?: string;
  viewedAt?: string;
  watermarkText?: string;
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
  const { t } = useTranslation("documents");
  const containerRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [tampered, setTampered] = useState(false);

  // Refresh the timestamp every second so the watermark reflects the current
  // access time rather than the mount time.
  const [viewedAt, setViewedAt] = useState(() => formatTimestamp(new Date()));
  useEffect(() => {
    const interval = setInterval(() => setViewedAt(formatTimestamp(new Date())), 1000);
    return () => clearInterval(interval);
  }, []);

  const text = watermark?.watermarkText ?? t("viewer.watermarkFallback");

  useEffect(() => {
    const canvas = canvasRef.current;
    const container = containerRef.current;
    if (!canvas || !container) return;

    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const dpr = window.devicePixelRatio || 1;
    const rect = container.getBoundingClientRect();
    const width = Math.max(1, rect.width);
    const height = Math.max(1, rect.height);

    canvas.width = Math.floor(width * dpr);
    canvas.height = Math.floor(height * dpr);
    canvas.style.width = `${width}px`;
    canvas.style.height = `${height}px`;

    ctx.resetTransform();
    ctx.scale(dpr, dpr);
    ctx.clearRect(0, 0, width, height);

    ctx.save();
    ctx.font = "bold 20px system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif";
    ctx.fillStyle = "rgba(0, 0, 0, 0.12)";
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";

    if (tiled) {
      ctx.translate(width / 2, height / 2);
      ctx.rotate((-30 * Math.PI) / 180);

      const metrics = ctx.measureText(text);
      const stepX = metrics.width + 80;
      const stepY = 120;
      const cols = Math.ceil((width * 1.5) / stepX) + 2;
      const rows = Math.ceil((height * 1.5) / stepY) + 2;

      const startX = -(cols * stepX) / 2;
      const startY = -(rows * stepY) / 2;

      for (let row = 0; row < rows; row++) {
        for (let col = 0; col < cols; col++) {
          const x = startX + col * stepX;
          const y = startY + row * stepY + (col % 2) * (stepY / 2);
          ctx.fillText(text, x, y);
        }
      }
    } else {
      ctx.translate(width - 16, height - 16);
      ctx.rotate((-30 * Math.PI) / 180);
      ctx.textAlign = "right";
      ctx.fillText(text, 0, 0);
    }

    ctx.restore();
  }, [text, viewedAt, tiled, tampered]);

  // Detect DOM tampering: if the canvas is removed from its container,
  // force a re-render so the watermark is restored.
  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const observer = new MutationObserver((mutations) => {
      for (const mutation of mutations) {
        if (mutation.type === "childList") {
          const canvas = canvasRef.current;
          if (canvas && !container.contains(canvas)) {
            setTampered((prev) => !prev);
          }
        }
      }
    });

    observer.observe(container, { childList: true, subtree: false });
    return () => observer.disconnect();
  }, []);

  if (!watermark) return null;

  return (
    <div
      ref={containerRef}
      className={cn(
        "pointer-events-none absolute inset-0 overflow-hidden",
        className
      )}
      aria-hidden="true"
    >
      <canvas ref={canvasRef} className="block h-full w-full" />
    </div>
  );
}
