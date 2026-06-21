import { motion, AnimatePresence } from "motion/react";
import { Check, Envelope, Phone, ShareNetwork, Warning, Clock } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import type { ActionItem } from "@/types";
import { useReducedMotion } from "@/hooks/useReducedMotion";
import { isOverdue, daysOverdue } from "@/lib/calculations";
import { useTranslation } from "react-i18next";

const actionConfig = {
  email: { icon: Envelope },
  call: { icon: Phone },
  share: { icon: ShareNetwork },
  review: { icon: Warning },
};

const impactConfig = {
  high: "bg-error-500/10 text-error-500 border-error-500/20",
  medium: "bg-warning-500/10 text-warning-500 border-warning-500/20",
  low: "bg-muted text-muted-foreground",
};

interface ActionListProps {
  actions: ActionItem[];
  onStatusChange: (id: string, status: ActionItem["status"]) => void;
}

export function ActionList({ actions, onStatusChange }: ActionListProps) {
  const reducedMotion = useReducedMotion();
  const { t } = useTranslation("dashboard");
  const { t: tCommon, i18n } = useTranslation("common");
  const pending = actions.filter((a) => a.status !== "done");
  const done = actions.filter((a) => a.status === "done");

  return (
    <div className="space-y-3">
      {pending.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border p-6 text-center">
          <Check size={32} className="mx-auto text-success-500" />
          <p className="mt-2 text-sm font-medium">{t("empty.actions.title")}</p>
          <p className="text-caption text-muted-foreground">{t("empty.actions.description")}</p>
        </div>
      ) : (
        <AnimatePresence initial={false}>
          {pending.map((action) => {
            const config = actionConfig[action.actionType];
            const Icon = config.icon;
            const overdue = isOverdue(action.dueAt);
            const impactLabel =
              action.impact === "high"
                ? tCommon("impact.high")
                : action.impact === "medium"
                  ? tCommon("impact.medium")
                  : tCommon("impact.low");
            return (
              <motion.div
                key={action.id}
                layout={!reducedMotion}
                exit={reducedMotion ? undefined : { opacity: 0, height: 0 }}
                className="flex items-start gap-3 rounded-xl border border-border bg-card p-(--card-spacing) shadow-card transition-colors hover:bg-muted/50 hover:border-muted-foreground/20"
              >
                <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                  <Icon size={18} />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="text-sm font-medium">{t(action.title)}</p>
                    <Badge variant="outline" className={impactConfig[action.impact]}>
                      {impactLabel}
                    </Badge>
                  </div>
                  <p
                    className={`text-caption mt-1 flex items-center gap-1 ${
                      overdue ? "font-medium text-error-500" : "text-muted-foreground"
                    }`}
                  >
                    <Clock size={12} />
                    {overdue
                      ? tCommon("overdue.days", { count: daysOverdue(action.dueAt) })
                      : `${tCommon("dueDate")} ${new Date(action.dueAt).toLocaleDateString(i18n.language)}`}
                  </p>
                </div>
                <Button size="sm" variant="outline" onClick={() => onStatusChange(action.id, "done")}>
                  {tCommon("complete")}
                </Button>
              </motion.div>
            );
          })}
        </AnimatePresence>
      )}

      {done.length > 0 && (
        <div className="pt-2">
          <p className="text-caption mb-2 text-muted-foreground">{t("actions.completedWithCount", { count: done.length })}</p>
          <div className="space-y-2 opacity-60">
            {done.map((action) => {
              const config = actionConfig[action.actionType];
              const Icon = config.icon;
              return (
                <div key={action.id} className="flex items-center gap-3 rounded-lg border bg-muted/50 p-3 line-through">
                  <Icon size={16} className="text-muted-foreground" />
                  <p className="text-sm text-muted-foreground">{t(action.title)}</p>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
