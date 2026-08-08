import { motion } from "motion/react";
import { useTranslation } from "react-i18next";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";
import {
  DEFAULT_VISITOR_ASK_EXPERIENCE,
  VISITOR_ASK_EXPERIENCE_OPTIONS,
  type VisitorAskExperience,
} from "./visitorAskExperience";

const LABEL_KEYS: Record<VisitorAskExperience, string> = {
  host_only: "accessRules.advanced.visitorAskExperience.hostOnly",
  ai_supervised: "accessRules.advanced.visitorAskExperience.aiSupervised",
  ai_self_serve: "accessRules.advanced.visitorAskExperience.aiSelfServe",
  formal: "accessRules.advanced.visitorAskExperience.formal",
};

interface VisitorAskExperienceFieldProps {
  value: VisitorAskExperience;
  onChange: (value: VisitorAskExperience) => void;
  disabled?: boolean;
  /** Experience values that cannot be selected (e.g. Formal when plan is not entitled). */
  disabledValues?: VisitorAskExperience[];
  disabledHint?: string;
  highlighted?: boolean;
  testId?: string;
  labelKey?: string;
}

function RadioIndicator({ selected }: { selected: boolean }) {
  return (
    <span
      aria-hidden
      className={cn(
        "relative flex h-[18px] w-[18px] shrink-0 items-center justify-center rounded-full border-2 transition-[border-color,box-shadow] duration-200",
        selected
          ? "border-primary shadow-[0_0_0_3px_hsl(var(--primary)/0.12)]"
          : "border-muted-foreground/35 bg-background/80",
      )}
    >
      <motion.span
        initial={false}
        animate={{ scale: selected ? 1 : 0, opacity: selected ? 1 : 0 }}
        transition={{ type: "spring", stiffness: 520, damping: 32, mass: 0.6 }}
        className="h-2 w-2 rounded-full bg-primary"
      />
    </span>
  );
}

export function VisitorAskExperienceField({
  value,
  onChange,
  disabled,
  disabledValues = [],
  disabledHint,
  highlighted,
  testId = "visitor-ask-experience",
  labelKey = "accessRules.advanced.visitorAskExperience.label",
}: VisitorAskExperienceFieldProps) {
  const { t } = useTranslation("linkShare");
  const effectiveValue = value || DEFAULT_VISITOR_ASK_EXPERIENCE;
  const blocked = new Set(disabledValues);

  return (
    <div
      className={cn(
        "space-y-2.5",
        highlighted && "rounded-xl ring-2 ring-primary/40 ring-offset-2 ring-offset-background",
      )}
      data-testid={testId}
    >
      <Label className="text-sm font-normal text-foreground">{t(labelKey)}</Label>
      <div
        role="radiogroup"
        aria-label={t(labelKey)}
        className={cn(
          "overflow-hidden rounded-xl border border-border/70 bg-gradient-to-b from-muted/35 via-background to-muted/15",
          "shadow-[inset_0_1px_0_0_hsl(var(--background)/0.6),0_1px_2px_hsl(var(--foreground)/0.04)]",
          disabled && "opacity-60",
        )}
      >
        {VISITOR_ASK_EXPERIENCE_OPTIONS.map((opt, index) => {
          const selected = effectiveValue === opt.value;
          const optionDisabled = disabled || blocked.has(opt.value);
          return (
            <button
              key={opt.value}
              type="button"
              role="radio"
              aria-checked={selected}
              disabled={optionDisabled}
              data-testid={`${testId}-${opt.value}`}
              onClick={() => onChange(opt.value)}
              className={cn(
                "group relative flex w-full items-center gap-3 px-3.5 py-3 text-left text-sm",
                "transition-colors duration-200",
                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-inset",
                index > 0 && "border-t border-border/45",
                selected ? "text-foreground" : "text-muted-foreground",
                !optionDisabled && !selected && "hover:bg-muted/25 hover:text-foreground",
                optionDisabled && "cursor-not-allowed opacity-60",
              )}
            >
              {selected ? (
                <motion.span
                  layoutId={`${testId}-selection`}
                  className="absolute inset-y-0 left-0 w-[3px] bg-primary"
                  transition={{ type: "spring", stiffness: 420, damping: 34 }}
                />
              ) : null}
              {selected ? (
                <motion.span
                  layoutId={`${testId}-selection-bg`}
                  className="absolute inset-0 bg-primary/[0.045] dark:bg-primary/10"
                  transition={{ type: "spring", stiffness: 420, damping: 34 }}
                />
              ) : null}
              <RadioIndicator selected={selected} />
              <span className="relative z-[1] leading-snug font-normal">
                {t(LABEL_KEYS[opt.value])}
              </span>
            </button>
          );
        })}
      </div>
      {disabledHint && disabledValues.length > 0 ? (
        <p className="text-xs text-muted-foreground" data-testid={`${testId}-disabled-hint`}>
          {disabledHint}
        </p>
      ) : null}
    </div>
  );
}
