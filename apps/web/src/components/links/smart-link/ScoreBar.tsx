import { ShieldCheck, Warning } from "@phosphor-icons/react";
import { cn } from "@/lib/utils";

// ---------------------------------------------------------------------------
// Shared ScoreBar — used by both StepSecurity (compact layout) and
// ScoreDisplay (card layout). Eliminates the previous code duplication
// between the two ScoreBar implementations.
// ---------------------------------------------------------------------------

export interface ScoreBarProps {
  label: string;
  score: number;
  variant: "security" | "friction";
  /** "compact" for StepSecurity inline row, "card" for ScoreDisplay vertical card. */
  layout?: "compact" | "card";
}

/** Color mapping shared across both layout variants. */
function scoreColor(score: number, variant: "security" | "friction"): string {
  if (variant === "security") {
    if (score >= 7) return "bg-emerald-500";
    if (score >= 4) return "bg-amber-500";
    return "bg-rose-500";
  }
  // friction: lower is better (green), higher is worse (red)
  if (score <= 3) return "bg-emerald-500";
  if (score <= 6) return "bg-amber-500";
  return "bg-rose-500";
}

export function ScoreBar({
  label,
  score,
  variant,
  layout = "compact",
}: ScoreBarProps) {
  const colorClass = scoreColor(score, variant);
  const iconColor = colorClass.replace("bg-", "text-").replace("-500", "-600");
  const Icon = variant === "security" ? ShieldCheck : Warning;

  if (layout === "card") {
    return (
      <div>
        <div className="mb-2 flex items-center justify-between text-sm">
          <span className="flex items-center gap-1.5 text-muted-foreground">
            <Icon size={16} />
            {label}
          </span>
          <span className="font-semibold tabular-nums">{score}/10</span>
        </div>
        <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
          <div
            className={cn(
              "h-full rounded-full transition-[width] duration-500",
              colorClass,
            )}
            style={{ width: `${score * 10}%` }}
          />
        </div>
      </div>
    );
  }

  // compact layout (default)
  return (
    <div className="flex items-center gap-3">
      <span className="flex w-24 shrink-0 items-center gap-1.5 text-sm text-muted-foreground">
        <Icon size={15} weight="fill" className={iconColor} />
        {label}
      </span>
      <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
        <div
          className={cn(
            "h-full rounded-full transition-[width] duration-500 ease-in-out",
            colorClass,
          )}
          style={{ width: `${score * 10}%` }}
        />
      </div>
      <span className="w-10 text-right text-sm font-semibold tabular-nums">
        {score}/10
      </span>
    </div>
  );
}
