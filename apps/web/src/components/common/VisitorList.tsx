import { useNavigate, useParams } from "react-router";
import { User } from "@phosphor-icons/react";
import { cn } from "@/lib/utils";
import { HeatBadge } from "./HeatBadge";
import type { HeatLevel } from "@/types";

interface Visitor {
  id: string;
  email: string;
  organization?: string;
  heatLevel: HeatLevel;
  visitCount: number;
  avgDurationSeconds: number;
  lastSeenAt: string;
}

interface VisitorListProps {
  visitors: Visitor[];
  className?: string;
}

export function VisitorList({ visitors, className }: VisitorListProps) {
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();

  if (visitors.length === 0) {
    return (
      <div className={cn("py-8 text-center text-caption text-muted-foreground", className)}>
        暂无访客数据
      </div>
    );
  }

  return (
    <ul className={cn("space-y-2", className)}>
      {visitors.map((visitor) => (
        <li
          key={visitor.id}
          className="flex cursor-pointer items-center gap-3 rounded-md border border-border p-3 transition-colors hover:bg-muted"
          onClick={() => navigate(`/${workspaceSlug}/contacts/${visitor.id}`)}
        >
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-muted">
            <User size={18} className="text-muted-foreground" />
          </div>
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium">{visitor.email}</p>
            <p className="text-caption text-muted-foreground">
              {visitor.organization} · {visitor.visitCount} 次访问 · 最后 {visitor.lastSeenAt}
            </p>
          </div>
          <HeatBadge level={visitor.heatLevel} />
        </li>
      ))}
    </ul>
  );
}
