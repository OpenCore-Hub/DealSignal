import {
  Check,
  Clock,
  ClockCounterClockwise,
  DotsThree,
  ArrowRight,
  X,
} from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { isOverdue, daysOverdue } from "@/lib/calculations";
import { radarRowIdentities } from "@/lib/radarEvidencePresentation";
import {
  defaultOutcomeForProduct,
  outcomesForProduct,
  radarCtaKey,
  radarEmailContactLabel,
  radarHeadlineKey,
  radarOutcomeKey,
  radarWhyNowFallbackKey,
  radarWhyNowKey,
  type RadarOutcome,
  type RadarWorkItem,
} from "@/lib/radarQueue";
import type { ActionStatus } from "@/types";

export type SnoozeHours = 24 | 72 | 168;

interface RadarRowProps {
  item: RadarWorkItem;
  emphasized?: boolean;
  selected?: boolean;
  /** Hide product caption when a product filter is active; keep layout space. */
  hideProductLabel?: boolean;
  onPrimary: (item: RadarWorkItem) => void;
  onSelect?: (item: RadarWorkItem) => void;
  onEvidence?: (item: RadarWorkItem) => void;
  onStatusChange?: (
    actionId: string,
    status: ActionStatus,
    snoozeHours?: SnoozeHours,
    outcome?: RadarOutcome,
  ) => void;
}

function urgencyBar(item: RadarWorkItem): string {
  if (
    item.product === "leak_watch" ||
    item.product === "abuse_guard" ||
    item.priority === "high"
  ) {
    return "bg-error-500";
  }
  if (item.priority === "medium") return "bg-warning-500";
  return "bg-muted-foreground/30";
}

export function RadarRow({
  item,
  emphasized,
  selected,
  hideProductLabel,
  onPrimary,
  onSelect,
  onEvidence,
  onStatusChange,
}: RadarRowProps) {
  const { t } = useTranslation("dashboard");
  const { t: tCommon, i18n } = useTranslation("common");
  const overdue = item.slaDueAt ? isOverdue(item.slaDueAt) : false;
  const outcomes = outcomesForProduct(item.product, item.verb);
  const completeDirectly = outcomes.length === 1;
  const identities = radarRowIdentities(item);
  const emailContact = radarEmailContactLabel(item);
  const showPrimary = item.verb !== "email" || Boolean(emailContact);

  return (
    <div
      role="listitem"
      data-testid="radar-row"
      data-radar-product={item.product}
      data-radar-confidence={item.confidence || undefined}
      data-radar-next={emphasized ? "true" : undefined}
      data-radar-selected={selected ? "true" : undefined}
      className={`group relative flex items-center gap-3 border-b border-border px-3 py-3 transition-colors hover:bg-muted/40 focus-within:bg-muted/40 ${
        emphasized || selected ? "bg-muted/30" : ""
      }`}
    >
      <div className={`absolute left-0 top-0 h-full w-[3px] ${urgencyBar(item)}`} />

      <button
        type="button"
        className="min-w-0 flex-1 rounded-md text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        onClick={() => (onSelect ?? onPrimary)(item)}
      >
        <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
          {identities.primary ? (
            <span className="text-sm font-semibold text-foreground">
              {identities.primary}
            </span>
          ) : null}
          <span className="text-sm font-medium text-foreground">
            {item.headlineCode
              ? t(radarHeadlineKey(item), {
                  defaultValue: t(`radar.products.${item.product}`),
                })
              : item.headline || t(`radar.products.${item.product}`)}
          </span>
          <span
            className={`text-caption text-muted-foreground${
              hideProductLabel ? " invisible" : ""
            }`}
            data-testid="radar-product-label"
            aria-hidden={hideProductLabel || undefined}
          >
            {t(`radar.products.${item.product}`)}
          </span>
          {item.confidence ? (
            <span
              className="text-caption text-muted-foreground"
              data-testid="radar-confidence"
            >
              {t(`radar.confidence.${item.confidence}`)}
            </span>
          ) : null}
        </div>
        {item.whyNowCode ? (
          <p
            className="text-caption mt-0.5 line-clamp-2 text-muted-foreground"
            data-testid="radar-why-now"
            data-radar-scenario={item.scenario || undefined}
          >
            {t(radarWhyNowKey(item), {
              hours: item.whyNowHours ?? 1,
              count: item.whyNowHours ?? item.coalescedFrom?.length ?? 1,
              defaultValue: t(radarWhyNowFallbackKey(item), {
                hours: item.whyNowHours ?? 1,
                count: item.whyNowHours ?? item.coalescedFrom?.length ?? 1,
              }),
            })}
          </p>
        ) : null}
        {item.evidence && item.evidence.length > 0 ? (
          <div className="mt-1 flex flex-wrap gap-1" data-testid="radar-evidence-chips">
            {item.evidence.map((chip) => (
              <span
                key={`${chip.kind}-${chip.count ?? 0}`}
                className="rounded border border-border px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground"
              >
                {t(`radar.evidenceChips.${chip.kind}`, {
                  count: chip.count ?? 1,
                })}
              </span>
            ))}
          </div>
        ) : null}
        {identities.shareContact ? (
          <p
            className="text-caption mt-0.5 text-muted-foreground"
            data-testid="radar-share-contact"
          >
            {t("radar.shareContact", { name: identities.shareContact })}
          </p>
        ) : null}
        <p className="text-caption mt-1 text-muted-foreground">
          {item.dealName || t("radar.dealFallback")}
        </p>
        {item.slaDueAt ? (
          <p
            className={`text-caption mt-1 flex items-center gap-1 ${
              overdue ? "font-medium text-error-500" : "text-muted-foreground"
            }`}
          >
            <Clock size={12} />
            {overdue
              ? tCommon("overdue.days", { count: daysOverdue(item.slaDueAt) })
              : `${tCommon("dueDate")} ${new Date(item.slaDueAt).toLocaleDateString(i18n.language)}`}
          </p>
        ) : null}
      </button>

      <div className="flex shrink-0 items-center gap-1">
        {showPrimary ? (
          <Button
            size="sm"
            variant="default"
            className="gap-1.5"
            onClick={(e) => {
              e.stopPropagation();
              onPrimary(item);
            }}
          >
            {t(radarCtaKey(item.product, item.verb), {
              contact: emailContact ?? undefined,
              defaultValue: t(`radar.cta.${item.verb}`),
            })}
          </Button>
        ) : null}

        {onStatusChange ? (
          completeDirectly ? (
            <Button
              size="icon-sm"
              variant="outline"
              aria-label={tCommon("complete")}
              onClick={(e) => {
                e.stopPropagation();
                onStatusChange(
                  item.actionId,
                  "done",
                  undefined,
                  defaultOutcomeForProduct(item.product, item.verb),
                );
              }}
            >
              <Check size={16} />
            </Button>
          ) : (
            <DropdownMenu>
              <DropdownMenuTrigger
                render={(props) => (
                  <Button
                    size="icon-sm"
                    variant="outline"
                    aria-label={t("radar.outcome.choose")}
                    data-testid="radar-complete-menu"
                    {...props}
                    onClick={(e) => e.stopPropagation()}
                  >
                    <Check size={16} />
                  </Button>
                )}
              />
              <DropdownMenuContent align="end">
                {outcomes.map((outcome) => (
                  <DropdownMenuItem
                    key={outcome}
                    onClick={() =>
                      onStatusChange(item.actionId, "done", undefined, outcome)
                    }
                  >
                    {t(radarOutcomeKey(item.product, outcome, item.verb), {
                      defaultValue: t(`radar.outcome.${outcome}`),
                    })}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
          )
        ) : null}

        {onStatusChange ? (
          <DropdownMenu>
            <DropdownMenuTrigger
              render={(props) => (
                <Button
                  size="icon-sm"
                  variant="ghost"
                  aria-label={t("actions.moreOptions")}
                  {...props}
                  onClick={(e) => e.stopPropagation()}
                >
                  <DotsThree size={18} />
                </Button>
              )}
            />
            <DropdownMenuContent align="end">
              {([24, 72, 168] as SnoozeHours[]).map((hours) => (
                <DropdownMenuItem
                  key={hours}
                  onClick={() => onStatusChange(item.actionId, "snoozed", hours)}
                >
                  <ClockCounterClockwise size={16} className="mr-1.5" />
                  {t(`radar.snoozeHours.${hours}`)}
                </DropdownMenuItem>
              ))}
              <DropdownMenuItem
                variant="destructive"
                onClick={() => onStatusChange(item.actionId, "ignored")}
              >
                <X size={16} className="mr-1.5" />
                {t("actions.ignore")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        ) : null}

        {onEvidence ? (
          <Button
            size="sm"
            variant="ghost"
            className="inline-flex text-muted-foreground"
            data-testid="radar-evidence-link"
            onClick={(e) => {
              e.stopPropagation();
              onEvidence(item);
            }}
          >
            {t("radar.evidence")}
            <ArrowRight size={14} className="ml-1" />
          </Button>
        ) : null}
      </div>
    </div>
  );
}
