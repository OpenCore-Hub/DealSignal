import { cn } from "@/lib/utils";
import { useBundlePipeline, type BundlePipelineState } from "./BundlePipelineContext";
import { useTranslation } from "react-i18next";
import { CheckIcon } from "@phosphor-icons/react";

const STEPS: { step: BundlePipelineState["step"]; key: string }[] = [
  { step: 1, key: "bundle.stepDocuments" },
  { step: 2, key: "bundle.stepSecurity" },
  { step: 3, key: "bundle.stepReview" },
];

interface PipelineProgressProps {
  className?: string;
}

export function PipelineProgress({ className }: PipelineProgressProps) {
  const { state, dispatch } = useBundlePipeline();
  const { t } = useTranslation("links");

  return (
    <div className={cn("flex items-center justify-center", className)}>
      <div className="flex items-center">
        {STEPS.map((s, i) => {
          const isCurrent = state.step === s.step;
          const isPast = state.step > s.step;
          const isFuture = state.step < s.step;
          const canClick = isPast;

          return (
            <div key={s.step} className="flex items-center">
              {/* Connector line (before, except first) */}
              {i > 0 && (
                <div
                  className={cn(
                    "h-px w-10 transition-colors duration-300 sm:w-14",
                    isPast ? "bg-primary" : "bg-border"
                  )}
                  aria-hidden="true"
                />
              )}

              {/* Step indicator */}
              <button
                type="button"
                disabled={!canClick}
                onClick={() => canClick && dispatch({ type: "GO_STEP", step: s.step })}
                className={cn(
                  "group flex flex-col items-center gap-1.5 px-1 transition-colors",
                  canClick ? "cursor-pointer" : "cursor-default"
                )}
                aria-current={isCurrent ? "step" : undefined}
                aria-label={t(s.key)}
              >
                <span
                  className={cn(
                    "flex h-7 w-7 items-center justify-center rounded-full text-xs font-semibold transition-all duration-300",
                    isPast && "bg-primary text-primary-foreground",
                    isCurrent && "bg-primary text-primary-foreground ring-2 ring-primary/20",
                    isFuture && "border border-border bg-background text-muted-foreground"
                  )}
                >
                  {isPast ? <CheckIcon size={14} weight="bold" /> : s.step}
                </span>
                <span
                  className={cn(
                    "text-xs whitespace-nowrap transition-colors",
                    isCurrent && "font-medium text-foreground",
                    (isPast || isFuture) && "text-muted-foreground",
                    canClick && "group-hover:text-foreground"
                  )}
                >
                  {t(s.key)}
                </span>
              </button>
            </div>
          );
        })}
      </div>
    </div>
  );
}
