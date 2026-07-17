import { useState } from "react";
import { motion, AnimatePresence } from "motion/react";
import { Check, Envelope, Phone, ShareNetwork, Warning, Clock, X, ClockCounterClockwise, DotsThree, CaretDown, CaretUp } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/common/EmptyState";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { ActionItem, ActionStatus } from "@/types";
import { useReducedMotion } from "@/hooks/useReducedMotion";
import { isOverdue, daysOverdue } from "@/lib/calculations";
import { useTranslation } from "react-i18next";

const actionConfig = {
  email: { icon: Envelope },
  call: { icon: Phone },
  share: { icon: ShareNetwork },
  review: { icon: Warning },
};

const impactBarColor = {
  high: "bg-error-500",
  medium: "bg-warning-500",
  low: "bg-muted-foreground/30",
};

interface ActionListProps {
  actions: ActionItem[];
  onStatusChange: (id: string, status: ActionStatus) => void;
}

export function ActionList({ actions, onStatusChange }: ActionListProps) {
  const reducedMotion = useReducedMotion();
  const { t } = useTranslation("dashboard");
  const { t: tCommon, i18n } = useTranslation("common");
  const [showHidden, setShowHidden] = useState(false);
  const [showCompleted, setShowCompleted] = useState(false);

  const pending = actions.filter((a) => a.status === "pending");
  const done = actions.filter((a) => a.status === "done");
  const hidden = actions.filter((a) => a.status === "snoozed" || a.status === "ignored");

  return (
    <div className="space-y-3">
      {pending.length === 0 ? (
        <EmptyState
          size="compact"
          icon={<Check size={32} />}
          title={t("empty.actions.title")}
          description={t("empty.actions.description")}
        />
      ) : (
        <div className="max-h-[340px] overflow-y-auto pr-1">
          <AnimatePresence initial={false}>
            {pending.map((action) => {
            const config = actionConfig[action.actionType] ?? { icon: Warning };
            const Icon = config.icon;
            const overdue = isOverdue(action.dueAt);
            const barColor = overdue
              ? "bg-error-500"
              : impactBarColor[action.impact];
            return (
              <motion.div
                key={action.id}
                layout={!reducedMotion}
                exit={reducedMotion ? undefined : { opacity: 0, height: 0 }}
                className="group/action relative flex items-center gap-3 overflow-hidden rounded-xl border border-border bg-card p-3 shadow-card transition-colors hover:bg-muted/50"
              >
                <div className={`absolute left-0 top-0 h-full w-[3px] ${barColor}`} />
                <div className="relative z-10 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                  <Icon size={16} />
                </div>
                <div className="relative z-10 min-w-0 flex-1">
                  <p className="text-sm font-medium">{action.title}</p>
                  <p
                    className={`text-caption flex items-center gap-1 ${
                      overdue ? "font-medium text-error-500" : "text-muted-foreground"
                    }`}
                  >
                    <Clock size={12} />
                    {overdue
                      ? tCommon("overdue.days", { count: daysOverdue(action.dueAt) })
                      : `${tCommon("dueDate")} ${new Date(action.dueAt).toLocaleDateString(i18n.language)}`}
                  </p>
                </div>
                <div className="relative z-10 flex items-center gap-1">
                  <Button
                    size="icon-sm"
                    variant="outline"
                    aria-label={tCommon("complete")}
                    onClick={() => onStatusChange(action.id, "done")}
                  >
                    <Check size={16} />
                  </Button>
                  <DropdownMenu>
                    <DropdownMenuTrigger
                      render={(props) => (
                        <Button size="icon-sm" variant="ghost" aria-label={t("actions.moreOptions")} {...props}>
                          <DotsThree size={18} />
                        </Button>
                      )}
                    />
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem onClick={() => onStatusChange(action.id, "snoozed")}>
                        <ClockCounterClockwise size={16} className="mr-1.5" />
                        {t("actions.postpone")}
                      </DropdownMenuItem>
                      <DropdownMenuItem variant="destructive" onClick={() => onStatusChange(action.id, "ignored")}>
                        <X size={16} className="mr-1.5" />
                        {t("actions.ignore")}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>

              </motion.div>
            );
          })}
        </AnimatePresence>
        </div>
      )}

      {done.length > 0 && (
        <div className="pt-2">
          <button
            type="button"
            onClick={() => setShowCompleted((v) => !v)}
            className="text-caption mb-2 flex items-center gap-1 text-muted-foreground hover:text-foreground"
            aria-expanded={showCompleted}
          >
            {t("actions.completedWithCount", { count: done.length })}
            {showCompleted ? <CaretUp size={12} /> : <CaretDown size={12} />}
          </button>
          <AnimatePresence initial={false}>
            {showCompleted && (
              <motion.div
                initial={reducedMotion ? undefined : { height: 0, opacity: 0 }}
                animate={{ height: "auto", opacity: 1 }}
                exit={reducedMotion ? undefined : { height: 0, opacity: 0 }}
                className="space-y-2 overflow-hidden opacity-60"
              >
                {done.map((action) => {
                  const config = actionConfig[action.actionType] ?? { icon: Warning };
                  const Icon = config.icon;
                  return (
                    <div key={action.id} className="flex items-center gap-3 rounded-lg border bg-muted/50 p-3 line-through">
                      <Icon size={16} className="text-muted-foreground" />
                      <p className="text-sm text-muted-foreground">{action.title}</p>
                    </div>
                  );
                })}
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      )}

      {hidden.length > 0 && (
        <div className="pt-2">
          <button
            type="button"
            onClick={() => setShowHidden((v) => !v)}
            className="text-caption mb-2 flex items-center gap-1 text-muted-foreground hover:text-foreground"
            aria-expanded={showHidden}
          >
            {t("actions.hiddenWithCount", { count: hidden.length })}
            {showHidden ? <CaretUp size={12} /> : <CaretDown size={12} />}
          </button>
          <AnimatePresence initial={false}>
            {showHidden && (
              <motion.div
                initial={reducedMotion ? undefined : { height: 0, opacity: 0 }}
                animate={{ height: "auto", opacity: 1 }}
                exit={reducedMotion ? undefined : { height: 0, opacity: 0 }}
                className="space-y-2 overflow-hidden opacity-60"
              >
                {hidden.map((action) => {
                  const config = actionConfig[action.actionType] ?? { icon: Warning };
                  const Icon = config.icon;
                  const statusLabel = action.status === "snoozed" ? tCommon("status.snoozed") : tCommon("status.ignored");
                  return (
                    <div key={action.id} className="flex items-center justify-between rounded-lg border bg-muted/50 p-3">
                      <div className="flex items-center gap-3">
                        <Icon size={16} className="text-muted-foreground" />
                        <p className="text-sm text-muted-foreground">{action.title}</p>
                      </div>
                      <div className="flex items-center gap-2">
                        <Badge variant="outline" className="text-xs">
                          {statusLabel}
                        </Badge>
                        <Button size="sm" variant="ghost" onClick={() => onStatusChange(action.id, "pending")}>
                          {t("actions.reactivate")}
                        </Button>
                      </div>
                    </div>
                  );
                })}
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      )}
    </div>
  );
}
