import { Label } from "@/components/ui/label";
import { Slider } from "@/components/ui/slider";
import { LEVEL_ORDER, levelConfig } from "./levelConfig";
import type { PermissionLevel } from "./types";

interface PermissionPanelProps {
  level: PermissionLevel;
  onLevelChange: (level: PermissionLevel) => void;
}

export function PermissionPanel({ level, onLevelChange }: PermissionPanelProps) {
  const handleValueChange = (value: number | readonly number[]) => {
    const index = Array.isArray(value) ? value[0] : value;
    onLevelChange(LEVEL_ORDER[index]);
  };

  const info = levelConfig[level];
  const Icon = info.icon;

  return (
    <div className="space-y-3">
      <Label>权限强度</Label>
      <Slider
        value={[LEVEL_ORDER.indexOf(level)]}
        onValueChange={handleValueChange}
        max={2}
        step={1}
      />
      <div className={`flex items-center gap-3 rounded-md border p-3 ${info.color}`}>
        <Icon size={20} />
        <div>
          <p className="text-sm font-medium">{info.label}</p>
          <p className="text-caption">{info.description}</p>
        </div>
      </div>
    </div>
  );
}
