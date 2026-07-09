import { Question } from "@phosphor-icons/react";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
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
      <div className="space-y-0.5">
        <Label className="flex items-center gap-1.5 font-normal text-foreground">
          {label}
          {description && (
            <span title={description}>
              <Question size={14} className="text-muted-foreground" />
            </span>
          )}
        </Label>
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
