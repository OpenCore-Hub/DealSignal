import { useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { ShieldCheck } from "@phosphor-icons/react";
import { cn } from "@/lib/utils";

const COUNTDOWN_SECONDS = 5;

interface SetupAccessRedirectOverlayProps {
  open: boolean;
  title: string;
  description?: string;
  /** Localized countdown copy; called each second with remaining seconds. */
  countdownLabel?: (seconds: number) => string;
  goNowLabel: string;
  onRedirect: () => void;
}

export function SetupAccessRedirectOverlay({
  open,
  title,
  description,
  countdownLabel,
  goNowLabel,
  onRedirect,
}: SetupAccessRedirectOverlayProps) {
  const [secondsLeft, setSecondsLeft] = useState(COUNTDOWN_SECONDS);
  const redirectedRef = useRef(false);

  const redirect = useCallback(() => {
    if (redirectedRef.current) return;
    redirectedRef.current = true;
    onRedirect();
  }, [onRedirect]);

  useEffect(() => {
    if (!open) {
      setSecondsLeft(COUNTDOWN_SECONDS);
      redirectedRef.current = false;
      return;
    }

    let left = COUNTDOWN_SECONDS;
    setSecondsLeft(COUNTDOWN_SECONDS);
    redirectedRef.current = false;

    const timer = window.setInterval(() => {
      left -= 1;
      setSecondsLeft(left);
      if (left <= 0) {
        window.clearInterval(timer);
        redirect();
      }
    }, 1000);

    return () => window.clearInterval(timer);
  }, [open, redirect]);

  if (!open || typeof document === "undefined") return null;

  const remaining = secondsLeft / COUNTDOWN_SECONDS;
  const radius = 18;
  const circumference = 2 * Math.PI * radius;
  const dashOffset = circumference * (1 - remaining);

  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      data-testid="setup-access-redirect-toast"
      role="status"
      aria-live="polite"
    >
      <div
        className="absolute inset-0 bg-background/35 backdrop-blur-md supports-[backdrop-filter]:bg-background/25"
        aria-hidden
      />

      <div
        className={cn(
          "relative w-[min(100%,18rem)] overflow-hidden rounded-3xl",
          "border-0 bg-background/55 shadow-none",
          "backdrop-blur-2xl supports-[backdrop-filter]:bg-background/40",
          "ring-0 outline-none"
        )}
      >
        <div className="flex flex-col items-center gap-4 px-6 py-7 text-center">
          <div className="relative flex size-14 items-center justify-center">
            <svg className="absolute inset-0 size-14 -rotate-90" viewBox="0 0 56 56" aria-hidden>
              <circle
                cx="28"
                cy="28"
                r={radius}
                fill="none"
                className="stroke-foreground/10"
                strokeWidth="2.5"
              />
              <circle
                cx="28"
                cy="28"
                r={radius}
                fill="none"
                className="stroke-primary/80 transition-[stroke-dashoffset] duration-1000 ease-linear"
                strokeWidth="2.5"
                strokeLinecap="round"
                strokeDasharray={circumference}
                strokeDashoffset={dashOffset}
              />
            </svg>
            <div className="flex size-10 items-center justify-center rounded-full bg-primary/15 text-primary">
              <ShieldCheck size={20} weight="duotone" aria-hidden />
            </div>
          </div>

          <p className="text-base font-semibold tracking-tight text-foreground">{title}</p>
          {description ? (
            <p className="text-sm text-muted-foreground">{description}</p>
          ) : null}
          {countdownLabel ? (
            <p className="text-xs text-muted-foreground" data-testid="setup-access-redirect-countdown">
              {countdownLabel(secondsLeft)}
            </p>
          ) : null}

          <button
            type="button"
            onClick={redirect}
            className="text-sm font-medium text-primary transition-opacity hover:opacity-80"
          >
            {goNowLabel}
          </button>
        </div>
      </div>
    </div>,
    document.body
  );
}
