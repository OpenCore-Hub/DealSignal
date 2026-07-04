import { useMemo } from "react";
import { cn } from "@/lib/utils";
import { useTranslation } from "react-i18next";
import { PRESET_ORDER, presetDef } from "./levelConfig";
import type { PermissionPreset } from "@/types";
import { Button } from "@/components/ui/button";

interface PermissionPanelProps {
  level: PermissionPreset;
  isCustomized: boolean;
  onLevelChange: (level: PermissionPreset) => void;
}

export function PermissionPanel({
  level,
  isCustomized,
  onLevelChange,
}: PermissionPanelProps) {
  const { t } = useTranslation("links");

  const activeDef = useMemo(() => presetDef[level], [level]);

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
          {t("creator.permissionStrength")}
        </span>
        {isCustomized && (
          <span className="rounded-full bg-muted px-2 py-0.5 text-caption text-muted-foreground">
            {t("preset.customized")}
          </span>
        )}
      </div>

      <div className="grid grid-cols-2 gap-2" role="radiogroup" aria-label={t("creator.permissionStrength")}>
        {PRESET_ORDER.map((preset) => {
          const def = presetDef[preset];
          const Icon = def.icon;
          const isActive = level === preset;

          return (
            <Button
              key={preset}
              type="button"
              variant="outline"
              role="radio"
              aria-checked={isActive}
              onClick={() => onLevelChange(preset)}
              className={cn(
                "h-auto flex-col items-start gap-1.5 px-3 py-3 text-left transition-all",
                isActive
                  ? "ring-2 ring-primary/50 border-primary/50 bg-primary/5"
                  : "hover:bg-muted/50",
              )}
            >
              <div className="flex w-full items-center justify-between">
                <span className="text-sm font-semibold">{t(def.label)}</span>
                <Icon
                  size={20}
                  className={cn(
                    "shrink-0",
                    isActive ? "text-primary" : "text-muted-foreground",
                  )}
                />
              </div>
              <p className="text-caption text-muted-foreground line-clamp-2">
                {t(def.description)}
              </p>
            </Button>
          );
        })}
      </div>

      {/* Active preset detail card */}
      {(() => {
        const ActiveIcon = activeDef.icon;
        return (
          <div
            className={cn(
              "flex items-start gap-3 rounded-md border p-3",
              activeDef.color,
            )}
          >
            <ActiveIcon size={20} className="mt-0.5 shrink-0" />
            <div className="min-w-0 space-y-1">
              <p className="text-sm font-medium">{t(activeDef.label)}</p>
              <p className="text-caption">{t(activeDef.usage)}</p>
              {activeDef.gates.length > 0 && (
                <div className="flex flex-wrap gap-1 pt-1">
                  {activeDef.gates.map((gate) => (
                    <span
                      key={gate}
                      className="inline-flex items-center rounded border bg-background/60 px-1.5 py-0.5 text-xs"
                    >
                      {t(`creator.${gate}`)}
                    </span>
                  ))}
                </div>
              )}
            </div>
          </div>
        );
      })()}
    </div>
  );
}
