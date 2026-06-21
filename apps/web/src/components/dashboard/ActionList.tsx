import { motion } from "motion/react";
import { Check, Envelope, Phone, ShareNetwork, Warning, Clock } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import type { ActionItem } from "@/types";

const actionConfig = {
  email: { icon: Envelope, label: "发邮件" },
  call: { icon: Phone, label: "电话" },
  share: { icon: ShareNetwork, label: "分享资料" },
  review: { icon: Warning, label: "审查" },
};

const impactConfig = {
  high: "bg-error-500/10 text-error-500 border-error-500/20",
  medium: "bg-warning-500/10 text-warning-500 border-warning-500/20",
  low: "bg-neutral-500/10 text-neutral-500 border-neutral-500/20",
};

interface ActionListProps {
  actions: ActionItem[];
  onStatusChange: (id: string, status: ActionItem["status"]) => void;
}

export function ActionList({ actions, onStatusChange }: ActionListProps) {
  const pending = actions.filter((a) => a.status !== "done");
  const done = actions.filter((a) => a.status === "done");

  return (
    <div className="space-y-3">
      {pending.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border p-6 text-center">
          <Check size={32} className="mx-auto text-success-500" />
          <p className="mt-2 text-sm font-medium">暂无待办行动</p>
          <p className="text-caption text-muted-foreground">所有信号都已处理完毕</p>
        </div>
      ) : (
        pending.map((action) => {
          const config = actionConfig[action.actionType];
          const Icon = config.icon;
          return (
            <motion.div
              key={action.id}
              layout
              className="flex items-start gap-3 rounded-xl border bg-card p-3 transition-shadow hover:shadow-sm"
            >
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                <Icon size={18} />
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <p className="text-sm font-medium">{action.title}</p>
                  <Badge variant="outline" className={impactConfig[action.impact]}>
                    {action.impact === "high" ? "高影响" : action.impact === "medium" ? "中影响" : "低影响"}
                  </Badge>
                </div>
                <p className="text-caption mt-1 flex items-center gap-1 text-muted-foreground">
                  <Clock size={12} />
                  截止 {new Date(action.dueAt).toLocaleDateString("zh-CN")}
                </p>
              </div>
              <Button size="sm" variant="outline" onClick={() => onStatusChange(action.id, "done")}>
                完成
              </Button>
            </motion.div>
          );
        })
      )}

      {done.length > 0 && (
        <div className="pt-2">
          <p className="text-caption mb-2 text-muted-foreground">已完成 ({done.length})</p>
          <div className="space-y-2 opacity-60">
            {done.map((action) => {
              const config = actionConfig[action.actionType];
              const Icon = config.icon;
              return (
                <div key={action.id} className="flex items-center gap-3 rounded-lg border bg-muted/50 p-3 line-through">
                  <Icon size={16} className="text-muted-foreground" />
                  <p className="text-sm text-muted-foreground">{action.title}</p>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
