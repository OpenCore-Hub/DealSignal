import { useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { FloppyDisk } from "@phosphor-icons/react";
import { cn } from "@/lib/utils";

export const UNSAVED_TAB_LEAVE_SECONDS = 5;

interface UnsavedTabLeaveOverlayProps {
  open: boolean;
  title: string;
  description?: string;
  countdownLabel: (seconds: number) => string;
  stayLabel: string;
  leaveNowLabel: string;
  onStay: () => void;
  onLeave: () => void;
}

/**
 * Premium 5s countdown when leaving Access Policy with unsaved edits.
 * Auto-leaves after the timer; Stay cancels.
 */
export function UnsavedTabLeaveOverlay({
  open,
  title,
  description,
  countdownLabel,
  stayLabel,
  leaveNowLabel,
  onStay,
  onLeave,
}: UnsavedTabLeaveOverlayProps) {
  const reducedMotion = useReducedMotion();
  const [secondsLeft, setSecondsLeft] = useState(UNSAVED_TAB_LEAVE_SECONDS);
  const leftRef = useRef(false);

  const leave = useCallback(() => {
    if (leftRef.current) return;
    leftRef.current = true;
    onLeave();
  }, [onLeave]);

  useEffect(() => {
    if (!open) {
      setSecondsLeft(UNSAVED_TAB_LEAVE_SECONDS);
      leftRef.current = false;
      return;
    }

    leftRef.current = false;
    let left = UNSAVED_TAB_LEAVE_SECONDS;
    setSecondsLeft(left);

    const tick = window.setInterval(() => {
      left -= 1;
      setSecondsLeft(left);
      if (left <= 0) {
        window.clearInterval(tick);
        leave();
      }
    }, 1000);

    return () => window.clearInterval(tick);
  }, [open, leave]);

  if (typeof document === "undefined") return null;

  const progress = secondsLeft / UNSAVED_TAB_LEAVE_SECONDS;
  const radius = 22;
  const circumference = 2 * Math.PI * radius;
  const dashOffset = circumference * (1 - progress);

  return createPortal(
    <AnimatePresence>
      {open ? (
        <motion.div
          className="fixed inset-0 z-50 flex items-center justify-center p-4"
          data-testid="unsaved-tab-leave-overlay"
          role="alertdialog"
          aria-modal="true"
          aria-labelledby="unsaved-tab-leave-title"
          aria-describedby="unsaved-tab-leave-desc"
          initial={reducedMotion ? { opacity: 1 } : { opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.28, ease: [0.16, 1, 0.3, 1] }}
        >
          <motion.button
            type="button"
            className="absolute inset-0 border-0 bg-background/40 backdrop-blur-md supports-[backdrop-filter]:bg-background/30"
            aria-label={stayLabel}
            data-testid="unsaved-tab-leave-backdrop"
            onClick={onStay}
            initial={reducedMotion ? undefined : { opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ duration: 0.35 }}
          />

          <motion.div
            className={cn(
              "relative w-[min(100%,20.5rem)] overflow-hidden rounded-[1.75rem]",
              "border border-white/20 bg-background/70 shadow-[0_24px_80px_-24px_rgba(15,23,42,0.45)]",
              "backdrop-blur-2xl supports-[backdrop-filter]:bg-background/55",
            )}
            initial={
              reducedMotion
                ? { opacity: 1 }
                : { opacity: 0, y: 16, scale: 0.94, filter: "blur(8px)" }
            }
            animate={{ opacity: 1, y: 0, scale: 1, filter: "blur(0px)" }}
            exit={
              reducedMotion
                ? { opacity: 0 }
                : { opacity: 0, y: 8, scale: 0.97, filter: "blur(4px)" }
            }
            transition={{ duration: 0.4, ease: [0.16, 1, 0.3, 1] }}
          >
            <div
              className="pointer-events-none absolute -left-10 -top-16 size-40 rounded-full bg-primary/15 blur-3xl"
              aria-hidden
            />
            <div
              className="pointer-events-none absolute -bottom-16 -right-8 size-36 rounded-full bg-foreground/[0.06] blur-3xl"
              aria-hidden
            />

            <div className="relative flex flex-col items-center gap-4 px-7 pb-6 pt-8 text-center">
              <div className="relative flex size-[4.25rem] items-center justify-center">
                {!reducedMotion ? (
                  <motion.div
                    className="absolute inset-0 rounded-full bg-primary/10"
                    animate={{ scale: [1, 1.08, 1], opacity: [0.55, 0.2, 0.55] }}
                    transition={{ duration: 2.4, repeat: Infinity, ease: "easeInOut" }}
                    aria-hidden
                  />
                ) : null}
                <svg className="absolute inset-0 size-[4.25rem] -rotate-90" viewBox="0 0 68 68" aria-hidden>
                  <circle
                    cx="34"
                    cy="34"
                    r={radius}
                    fill="none"
                    className="stroke-foreground/10"
                    strokeWidth="2.5"
                  />
                  <circle
                    cx="34"
                    cy="34"
                    r={radius}
                    fill="none"
                    className="stroke-primary"
                    strokeWidth="2.5"
                    strokeLinecap="round"
                    strokeDasharray={circumference}
                    strokeDashoffset={dashOffset}
                    style={{
                      transition: reducedMotion
                        ? undefined
                        : "stroke-dashoffset 1s linear",
                    }}
                  />
                </svg>
                <div className="relative flex size-11 items-center justify-center rounded-full bg-primary/12 text-primary shadow-[inset_0_1px_0_rgba(255,255,255,0.35)]">
                  <FloppyDisk size={22} weight="duotone" aria-hidden />
                  <span
                    className="absolute -bottom-0.5 -right-0.5 flex size-5 items-center justify-center rounded-full bg-background text-[10px] font-semibold tabular-nums text-foreground shadow-sm ring-1 ring-border/60"
                    data-testid="unsaved-tab-leave-seconds"
                  >
                    {secondsLeft}
                  </span>
                </div>
              </div>

              <div className="space-y-1.5">
                <p
                  id="unsaved-tab-leave-title"
                  className="text-[0.95rem] font-medium tracking-tight text-foreground"
                >
                  {title}
                </p>
                {description ? (
                  <p
                    id="unsaved-tab-leave-desc"
                    className="text-xs leading-relaxed text-muted-foreground"
                  >
                    {description}
                  </p>
                ) : null}
                <p
                  className="text-[11px] tabular-nums tracking-wide text-muted-foreground/90"
                  data-testid="unsaved-tab-leave-countdown"
                >
                  {countdownLabel(secondsLeft)}
                </p>
              </div>

              <div className="mt-1 flex w-full items-center justify-center gap-5">
                <button
                  type="button"
                  onClick={onStay}
                  data-testid="unsaved-tab-leave-stay"
                  className="text-sm text-muted-foreground transition-colors hover:text-foreground"
                >
                  {stayLabel}
                </button>
                <button
                  type="button"
                  onClick={leave}
                  className="text-sm font-medium text-primary transition-opacity hover:opacity-80"
                  data-testid="unsaved-tab-leave-now"
                >
                  {leaveNowLabel}
                </button>
              </div>
            </div>

            <div className="relative h-[2px] w-full overflow-hidden bg-foreground/[0.06]" aria-hidden>
              {open ? (
                <motion.div
                  className="absolute inset-y-0 left-0 bg-primary/80"
                  initial={{ width: "100%" }}
                  animate={{ width: "0%" }}
                  transition={
                    reducedMotion
                      ? { duration: 0 }
                      : { duration: UNSAVED_TAB_LEAVE_SECONDS, ease: "linear" }
                  }
                />
              ) : null}
            </div>
          </motion.div>
        </motion.div>
      ) : null}
    </AnimatePresence>,
    document.body,
  );
}
