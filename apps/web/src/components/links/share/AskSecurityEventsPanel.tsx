import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { ShieldWarning } from "@phosphor-icons/react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import { formatRelativeTime } from "@/lib/formatters";
import type { AskSecurityEvent } from "@/types";
import { toast } from "sonner";

const PAGE_SIZE = 20;

const EVENT_TYPE_OPTIONS = [
  "rate_limit_exceeded",
  "scope_violation",
  "blocked_email",
  "blocked_domain",
  "not_in_allow_list",
] as const;

type TimeRange = "all" | "24h" | "7d" | "30d";

export type AskSecurityEventsPanelProps =
  | { mode: "link"; linkId: string }
  | {
      mode: "room";
      roomId: string;
      links?: Array<{ id: string; name?: string }>;
    };

type LoadError = "forbidden" | "generic" | null;

type EventsPage = {
  data: AskSecurityEvent[];
  has_more: boolean;
};

function sinceForRange(range: TimeRange, now = Date.now()): string | undefined {
  if (range === "all") return undefined;
  const ms =
    range === "24h"
      ? 24 * 60 * 60 * 1000
      : range === "7d"
        ? 7 * 24 * 60 * 60 * 1000
        : 30 * 24 * 60 * 60 * 1000;
  return new Date(now - ms).toISOString();
}

export function AskSecurityEventsPanel(props: AskSecurityEventsPanelProps) {
  const { t } = useTranslation("linkShare");
  const { t: tc } = useTranslation("common");
  const [events, setEvents] = useState<AskSecurityEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const [error, setError] = useState<LoadError>(null);
  const [linkFilter, setLinkFilter] = useState<string>("all");
  const [eventTypeFilter, setEventTypeFilter] = useState<string>("all");
  const [timeRange, setTimeRange] = useState<TimeRange>("all");
  const [retryTick, setRetryTick] = useState(0);
  const nextOffsetRef = useRef(0);
  const loadMoreLock = useRef(false);

  const scopeId = props.mode === "link" ? props.linkId : props.roomId;
  const isRoom = props.mode === "room";

  const fetchPage = useCallback(
    async (offset: number): Promise<EventsPage> => {
      const eventType = eventTypeFilter === "all" ? undefined : eventTypeFilter;
      const since = sinceForRange(timeRange);
      if (isRoom) {
        return api.listRoomAskSecurityEvents(scopeId, {
          linkId: linkFilter === "all" ? undefined : linkFilter,
          eventType,
          since,
          limit: PAGE_SIZE,
          offset,
        });
      }
      return api.listLinkAskSecurityEvents(scopeId, {
        eventType,
        since,
        limit: PAGE_SIZE,
        offset,
      });
    },
    [isRoom, scopeId, linkFilter, eventTypeFilter, timeRange],
  );

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      setError(null);
      setEvents([]);
      setHasMore(false);
      nextOffsetRef.current = 0;
      try {
        const res = await fetchPage(0);
        if (cancelled) return;
        const page = res.data ?? [];
        setEvents(page);
        nextOffsetRef.current = page.length;
        setHasMore(Boolean(res.has_more));
      } catch (e) {
        if (cancelled) return;
        if (e instanceof ApiError && e.status === 403) {
          setError("forbidden");
        } else {
          setError("generic");
        }
        setEvents([]);
        setHasMore(false);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, [fetchPage, retryTick]);

  const loadMore = async () => {
    if (loadMoreLock.current || loadingMore || !hasMore || loading || error) {
      return;
    }
    loadMoreLock.current = true;
    setLoadingMore(true);
    try {
      const res = await fetchPage(nextOffsetRef.current);
      const page = res.data ?? [];
      setEvents((prev) => {
        const seen = new Set(prev.map((e) => e.id));
        const next = [...prev];
        for (const item of page) {
          if (!seen.has(item.id)) next.push(item);
        }
        nextOffsetRef.current = next.length;
        return next;
      });
      setHasMore(Boolean(res.has_more));
    } catch {
      toast.error(t("askSecurityEvents.loadMoreFailed"));
      setHasMore(true);
    } finally {
      setLoadingMore(false);
      loadMoreLock.current = false;
    }
  };

  const title =
    props.mode === "room"
      ? t("askSecurityEvents.roomTitle")
      : t("askSecurityEvents.title");
  const description =
    props.mode === "room"
      ? t("askSecurityEvents.roomDescription")
      : t("askSecurityEvents.description");

  const linkName = (linkId?: string) => {
    if (!linkId || props.mode !== "room") return null;
    const match = props.links?.find((l) => l.id === linkId);
    return match?.name || linkId;
  };

  const eventTypeLabel = (eventType: string) => {
    const key = `askSecurityEvents.eventTypes.${eventType}`;
    const translated = t(key);
    return translated === key ? eventType : translated;
  };

  const reasonLabel = (reason?: string) => {
    if (!reason) return null;
    const key = `askSecurityEvents.reasons.${reason}`;
    const translated = t(key);
    return translated === key ? reason : translated;
  };

  const selectClassName =
    "flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm";

  return (
    <Card data-testid="ask-security-events-panel">
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-h3">
          <ShieldWarning size={20} />
          {title}
        </CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div
          className={
            props.mode === "room" && (props.links?.length ?? 0) > 0
              ? "grid gap-3 sm:grid-cols-3"
              : "grid gap-3 sm:grid-cols-2"
          }
        >
          <div className="space-y-1.5">
            <Label htmlFor="ask-security-events-time-filter">
              {t("askSecurityEvents.filterByTime")}
            </Label>
            <select
              id="ask-security-events-time-filter"
              aria-label={t("askSecurityEvents.filterByTime")}
              className={selectClassName}
              value={timeRange}
              onChange={(e) => setTimeRange(e.target.value as TimeRange)}
            >
              <option value="all">{t("askSecurityEvents.filterAllTime")}</option>
              <option value="24h">{t("askSecurityEvents.filterLast24h")}</option>
              <option value="7d">{t("askSecurityEvents.filterLast7d")}</option>
              <option value="30d">{t("askSecurityEvents.filterLast30d")}</option>
            </select>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="ask-security-events-type-filter">
              {t("askSecurityEvents.filterByEventType")}
            </Label>
            <select
              id="ask-security-events-type-filter"
              aria-label={t("askSecurityEvents.filterByEventType")}
              className={selectClassName}
              value={eventTypeFilter}
              onChange={(e) => setEventTypeFilter(e.target.value)}
            >
              <option value="all">
                {t("askSecurityEvents.filterAllEventTypes")}
              </option>
              {EVENT_TYPE_OPTIONS.map((type) => (
                <option key={type} value={type}>
                  {eventTypeLabel(type)}
                </option>
              ))}
            </select>
          </div>

          {props.mode === "room" && (props.links?.length ?? 0) > 0 ? (
            <div className="space-y-1.5">
              <Label htmlFor="ask-security-events-link-filter">
                {t("askSecurityEvents.filterByLink")}
              </Label>
              <select
                id="ask-security-events-link-filter"
                aria-label={t("askSecurityEvents.filterByLink")}
                className={selectClassName}
                value={linkFilter}
                onChange={(e) => setLinkFilter(e.target.value)}
              >
                <option value="all">{t("askSecurityEvents.filterAllLinks")}</option>
                {props.links!.map((link) => (
                  <option key={link.id} value={link.id}>
                    {link.name || link.id}
                  </option>
                ))}
              </select>
            </div>
          ) : null}
        </div>

        {loading ? (
          <p className="py-4 text-center text-sm text-muted-foreground">
            {t("askSecurityEvents.loading")}
          </p>
        ) : error === "forbidden" ? (
          <p className="py-4 text-center text-sm text-muted-foreground">
            {t("askSecurityEvents.forbidden")}
          </p>
        ) : error === "generic" ? (
          <div
            className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm"
            role="alert"
          >
            <p className="text-center text-destructive">
              {t("askSecurityEvents.loadFailed")}
            </p>
            <div className="mt-2 flex justify-center">
              <Button
                size="sm"
                variant="outline"
                onClick={() => setRetryTick((n) => n + 1)}
              >
                {tc("retry")}
              </Button>
            </div>
          </div>
        ) : events.length === 0 ? (
          <p className="py-4 text-center text-sm text-muted-foreground">
            {t("askSecurityEvents.empty")}
          </p>
        ) : (
          <div className="space-y-3">
            {events.map((event) => {
              const reason = reasonLabel(event.reason);
              const identity =
                event.email || event.visitor_id || t("askSecurityEvents.anonymous");
              return (
                <div
                  key={event.id}
                  className="w-full rounded-lg border p-3 text-left"
                  data-testid="ask-security-event-row"
                >
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0 flex-1 space-y-1">
                      <div className="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                        <span>{identity}</span>
                        {linkName(event.link_id) ? (
                          <>
                            <span>·</span>
                            <span>{linkName(event.link_id)}</span>
                          </>
                        ) : null}
                      </div>
                      <p className="text-sm font-medium">
                        {eventTypeLabel(event.event_type)}
                      </p>
                      {reason ? (
                        <p className="text-xs text-muted-foreground">
                          {t("askSecurityEvents.reasonLabel")}: {reason}
                        </p>
                      ) : null}
                      <p className="text-xs text-muted-foreground">
                        {formatRelativeTime(event.created_at)}
                      </p>
                    </div>
                    <Badge variant="destructive" className="shrink-0">
                      {t("askSecurityEvents.highRiskBadge")}
                    </Badge>
                  </div>
                </div>
              );
            })}
            {hasMore ? (
              <div className="flex justify-center pt-1">
                <Button
                  size="sm"
                  variant="outline"
                  disabled={loadingMore}
                  onClick={() => {
                    void loadMore();
                  }}
                  data-testid="ask-security-events-load-more"
                >
                  {loadingMore
                    ? t("askSecurityEvents.loadingMore")
                    : t("askSecurityEvents.loadMore")}
                </Button>
              </div>
            ) : null}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
