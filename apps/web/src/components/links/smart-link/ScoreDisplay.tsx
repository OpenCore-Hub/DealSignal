import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Warning, ShieldWarning } from "@phosphor-icons/react";
import { calculateFrictionScore, calculateSecurityScore, levelConfig } from "./levelConfig";
import type { PermissionConfig } from "@/types";
import type { PermissionLevel } from "./types";

interface ScoreDisplayProps {
  level: PermissionLevel;
  config: PermissionConfig;
}

export function ScoreDisplay({ level, config }: ScoreDisplayProps) {
  const frictionScore = calculateFrictionScore(config);
  const securityScore = calculateSecurityScore(config);
  const info = levelConfig[level];

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-h2">安全 vs 摩擦</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div>
          <div className="mb-1 flex items-center justify-between text-sm">
            <span className="flex items-center gap-1">
              <ShieldWarning size={14} /> 安全强度
            </span>
            <span className="font-medium">{securityScore}/10</span>
          </div>
          <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
            <div
              className="h-full rounded-full bg-success-500 transition-[width]"
              style={{ width: `${securityScore * 10}%` }}
            />
          </div>
        </div>
        <div>
          <div className="mb-1 flex items-center justify-between text-sm">
            <span className="flex items-center gap-1">
              <Warning size={14} /> 接收方摩擦
            </span>
            <span className="font-medium">{frictionScore}/10</span>
          </div>
          <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
            <div
              className={`h-full rounded-full transition-[width] ${
                frictionScore <= 3 ? "bg-success-500" : frictionScore <= 6 ? "bg-warm-500" : "bg-hot-500"
              }`}
              style={{ width: `${frictionScore * 10}%` }}
            />
          </div>
        </div>
        <p className="text-caption text-muted-foreground">{info.friction}</p>
      </CardContent>
    </Card>
  );
}
