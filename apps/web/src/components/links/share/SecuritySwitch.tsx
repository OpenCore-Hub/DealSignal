import { Question } from "@phosphor-icons/react";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

export interface SecuritySwitchProps {
  label: string;
  description?: string;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  disabled?: boolean;
}

export function SecuritySwitch({
  label,
  description,
  checked,
  onCheckedChange,
  disabled,
}: SecuritySwitchProps) {
  return (
    <div className={cn("flex items-start justify-between gap-4", disabled && "opacity-50")}>
      <div className="flex min-w-0 items-center gap-1.5">
        <Label className="font-normal text-foreground">{label}</Label>
        {description && (
          <TooltipProvider delay={150}>
            <Tooltip>
              <TooltipTrigger
                type="button"
                delay={150}
                className="inline-flex size-5 shrink-0 items-center justify-center rounded-sm text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
                aria-label={description}
              >
                <Question size={14} weight="regular" aria-hidden />
              </TooltipTrigger>
              <TooltipContent side="top">{description}</TooltipContent>
            </Tooltip>
          </TooltipProvider>
        )}
      </div>
      <Switch
        checked={checked}
        onCheckedChange={onCheckedChange}
        disabled={disabled}
        aria-label={label}
      />
    </div>
  );
}
