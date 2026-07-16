import { useState } from "react";
import { useLocation, useNavigate, useParams } from "react-router";
import { motion, AnimatePresence } from "motion/react";
import {
  Fire,
  Thermometer,
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
import { useTranslation } from "react-i18next";

const typeConfig = {
  hot_signal: { icon: Fire, dot: "bg-hot-500", subtle: "bg-hot-500/8", bar: "bg-hot-500" },
  follow_up: { icon: Thermometer, dot: "bg-warm-500", subtle: "bg-warm-500/8", bar: "bg-warm-500" },
  risk_alert: { icon: Warning, dot: "bg-risk-500", subtle: "bg-risk-500/8", bar: "bg-risk-500" },
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
  const location = useLocation();
  const [expanded, setExpanded] = useState(false);
  const reducedMotion = useReducedMotion();
  const { t } = useTranslation("dashboard");
  const { t: tCommon, i18n } = useTranslation("common");
  const config = typeConfig[signal.type] ?? typeConfig.follow_up;
  const Icon = config.icon;

  const handleNavigate = () => {
    const state = { returnTo: location.pathname + location.search, returnLabel: tCommon("back") };
    if (signal.documentId) navigate(`/${workspaceSlug}/documents/${signal.documentId}`, { state });
    else if (signal.linkId) navigate(`/${workspaceSlug}/links/${signal.linkId}`, { state });
    else if (signal.contactId) navigate(`/${workspaceSlug}/contacts/${signal.contactId}`, { state });
  };

  const actionIcon = {
    email: Envelope,
    call: Phone,
    share: ShareNetwork,
    review: Warning,
  }[action?.actionType ?? "email"];
  const ActionIcon = actionIcon;

  const priorityLabel =
    signal.priority === "high"
      ? tCommon("priority.high")
      : signal.priority === "medium"
        ? tCommon("priority.medium")
        : tCommon("priority.low");

  return (
    <motion.div
      layout={!reducedMotion}
      className="group/signal spotlight relative overflow-hidden rounded-xl border border-border bg-card p-(--card-spacing) shadow-card transition-colors hover:bg-muted/50 hover:border-muted-foreground/20"
    >
      <div className={`absolute left-0 top-0 h-full w-[3px] ${config.bar}`} />
      <div className="relative z-10 flex items-start gap-3">
        <div
          className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-lg ${config.subtle} text-foreground`}
        >
          <Icon size={18} weight="fill" className={config.dot.replace("bg-", "text-")} />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="text-h3 truncate">{t(`signal.types.${signal.type}`, { defaultValue: signal.title })}</h3>
            <Badge variant="outline" className={priorityConfig[signal.priority]}>
              {priorityLabel}
            </Badge>
          </div>
          {signal.suggestion && (
            <p className="text-caption mt-1 text-muted-foreground">
              <span className="font-medium text-foreground">{t("signal.suggestedAction")}:</span> {signal.suggestion}
            </p>
          )}
          <div className="mt-3 flex flex-wrap items-center gap-2">
            <Button size="sm" variant="outline" onClick={handleNavigate}>
              {tCommon("viewDetails")} <ArrowRight size={14} className="ml-1" />
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => setExpanded((v) => !v)}
              className="text-muted-foreground"
              aria-expanded={expanded}
              aria-label={expanded ? t("signal.collapse") : t("signal.expand")}
            >
              {expanded ? t("signal.collapse") : t("signal.expand")}
              {expanded ? <CaretUp size={14} className="ml-1" /> : <CaretDown size={14} className="ml-1" />}
            </Button>
            {action && (
              <Button
                size="sm"
                variant={action.status === "done" ? "secondary" : "default"}
                onClick={() =>
                  onActionStatusChange?.(action.id, action.status === "done" ? "pending" : "done")
                }
                className="ml-auto"
              >
                {action.status === "done" ? tCommon("status.done") : t("signal.markDone")}
              </Button>
            )}
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
            <div className="relative z-10 mt-4 space-y-3 border-t border-border pt-4">
              <p className="text-body text-muted-foreground">{signal.description}</p>
              <div>
                <p className="text-caption font-medium text-foreground">{t("signal.aiExplanation")}</p>
                <p className="text-body mt-0.5 text-muted-foreground">{signal.explanation}</p>
              </div>
              {action && (
                <div className="flex items-center gap-3 rounded-lg bg-muted/50 p-3">
                  <div className="flex h-8 w-8 items-center justify-center rounded-md bg-primary/10 text-primary">
                    <ActionIcon size={16} />
                  </div>
                  <div className="flex-1">
                    <p className="text-sm font-medium">{action.title}</p>
                    <p className="text-caption text-muted-foreground">
                      {tCommon("dueDate")} {new Date(action.dueAt).toLocaleDateString(i18n.language)}
                    </p>
                  </div>
                </div>
              )}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </motion.div>
  );
}
