import { useState } from "react";
import { useNavigate, useParams } from "react-router";
import { motion, AnimatePresence } from "motion/react";
import {
  Fire,
  Thermometer,
  Snowflake,
  Warning,
  CaretDown,
  CaretUp,
  ArrowRight,
  Envelope,
  Phone,
  ShareNetwork,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import type { Signal, ActionItem } from "@/types";
import { useReducedMotion } from "@/hooks/useReducedMotion";

const typeConfig = {
  hot: { icon: Fire, label: "高热度", dot: "bg-hot-500", subtle: "bg-hot-500/8" },
  warm: { icon: Thermometer, label: "中热度", dot: "bg-warm-500", subtle: "bg-warm-500/8" },
  cold: { icon: Snowflake, label: "低热度", dot: "bg-cold-500", subtle: "bg-cold-500/8" },
  risk: { icon: Warning, label: "风险", dot: "bg-risk-500", subtle: "bg-risk-500/8" },
};

const priorityConfig = {
  high: "bg-error-500/10 text-error-500 border-error-500/20",
  medium: "bg-warning-500/10 text-warning-500 border-warning-500/20",
  low: "bg-muted text-muted-foreground",
};

interface SignalCardProps {
  signal: Signal;
  action?: ActionItem;
  onActionStatusChange?: (id: string, status: ActionItem["status"]) => void;
}

export function SignalCard({ signal, action, onActionStatusChange }: SignalCardProps) {
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const [expanded, setExpanded] = useState(false);
  const reducedMotion = useReducedMotion();
  const config = typeConfig[signal.type];
  const Icon = config.icon;

  const handleNavigate = () => {
    if (signal.documentId) navigate(`/${workspaceSlug}/documents/${signal.documentId}`);
    else if (signal.linkId) navigate(`/${workspaceSlug}/links/${signal.linkId}`);
    else if (signal.contactId) navigate(`/${workspaceSlug}/contacts/${signal.contactId}`);
  };

  const actionIcon = {
    email: Envelope,
    call: Phone,
    share: ShareNetwork,
    review: Warning,
  }[action?.actionType ?? "email"];
  const ActionIcon = actionIcon;

  return (
    <motion.div
      layout={!reducedMotion}
      className="group/signal rounded-xl border border-border bg-card p-(--card-spacing) shadow-card transition-shadow hover:shadow-md"
    >
      <div className="flex items-start gap-3">
        <div
          className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-lg ${config.subtle} text-foreground`}
        >
          <Icon size={18} weight="fill" className={config.dot.replace("bg-", "text-")} />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="text-h3 truncate">{signal.title}</h3>
            <Badge variant="outline" className={priorityConfig[signal.priority]}>
              {signal.priority === "high" ? "高优先级" : signal.priority === "medium" ? "中优先级" : "低优先级"}
            </Badge>
          </div>
          <p className="text-body mt-1 text-muted-foreground">{signal.description}</p>
          <div className="mt-3 flex flex-wrap items-center gap-2">
            <Button size="sm" variant="outline" onClick={handleNavigate}>
              查看详情 <ArrowRight size={14} className="ml-1" />
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => setExpanded((v) => !v)}
              className="text-muted-foreground"
            >
              {expanded ? "收起" : "展开分析"}
              {expanded ? <CaretUp size={14} className="ml-1" /> : <CaretDown size={14} className="ml-1" />}
            </Button>
          </div>
        </div>
      </div>

      <AnimatePresence initial={false}>
        {expanded && (
          <motion.div
            initial={reducedMotion ? undefined : { height: 0, opacity: 0 }}
            animate={{ height: "auto", opacity: 1 }}
            exit={reducedMotion ? undefined : { height: 0, opacity: 0 }}
            className="overflow-hidden"
          >
            <div className="mt-4 space-y-3 border-t border-border pt-4">
              <div>
                <p className="text-caption font-medium uppercase tracking-wide text-muted-foreground">AI 解释</p>
                <p className="text-body mt-1">{signal.explanation}</p>
              </div>
              <div>
                <p className="text-caption font-medium uppercase tracking-wide text-muted-foreground">建议行动</p>
                <p className="text-body mt-1">{signal.suggestion}</p>
              </div>
              {action && (
                <div className="flex items-center gap-3 rounded-lg bg-muted/50 p-3">
                  <div className="flex h-8 w-8 items-center justify-center rounded-md bg-primary/10 text-primary">
                    <ActionIcon size={16} />
                  </div>
                  <div className="flex-1">
                    <p className="text-sm font-medium">{action.title}</p>
                    <p className="text-caption text-muted-foreground">
                      截止 {new Date(action.dueAt).toLocaleDateString("zh-CN")}
                    </p>
                  </div>
                  <Button
                    size="sm"
                    variant={action.status === "done" ? "secondary" : "default"}
                    onClick={() =>
                      onActionStatusChange?.(action.id, action.status === "done" ? "pending" : "done")
                    }
                  >
                    {action.status === "done" ? "已完成" : "标记完成"}
                  </Button>
                </div>
              )}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </motion.div>
  );
}
