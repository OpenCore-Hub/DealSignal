import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ShieldWarning } from "@phosphor-icons/react";
import { Badge } from "@/components/ui/badge";
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

export type AskSecurityEventsPanelProps =
  | { mode: "link"; linkId: string }
  | {
      mode: "room";
      roomId: string;
      links?: Array<{ id: string; name?: string }>;
    };

type LoadError = "forbidden" | "generic" | null;

export function AskSecurityEventsPanel(props: AskSecurityEventsPanelProps) {
  const { t } = useTranslation("linkShare");
  const [events, setEvents] = useState<AskSecurityEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<LoadError>(null);
  const [linkFilter, setLinkFilter] = useState<string>("all");

  const scopeId = props.mode === "link" ? props.linkId : props.roomId;
  const isRoom = props.mode === "room";

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      setError(null);
      try {
        const res = isRoom
          ? await api.listRoomAskSecurityEvents(scopeId, {
              linkId: linkFilter === "all" ? undefined : linkFilter,
            })
          : await api.listLinkAskSecurityEvents(scopeId);
        if (cancelled) return;
        setEvents(res.data ?? []);
      } catch (e) {
        if (cancelled) return;
        if (e instanceof ApiError && e.status === 403) {
          setError("forbidden");
        } else {
          setError("generic");
        }
        setEvents([]);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, [isRoom, scopeId, linkFilter]);

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
        {props.mode === "room" && (props.links?.length ?? 0) > 0 ? (
          <div className="space-y-1.5">
            <Label htmlFor="ask-security-events-link-filter">
              {t("askSecurityEvents.filterByLink")}
            </Label>
            <select
              id="ask-security-events-link-filter"
              aria-label={t("askSecurityEvents.filterByLink")}
              className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
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

        {loading ? (
          <p className="py-4 text-center text-sm text-muted-foreground">
            {t("askSecurityEvents.loading")}
          </p>
        ) : error === "forbidden" ? (
          <p className="py-4 text-center text-sm text-muted-foreground">
            {t("askSecurityEvents.forbidden")}
          </p>
        ) : error === "generic" ? (
          <p className="py-4 text-center text-sm text-muted-foreground">
            {t("askSecurityEvents.loadFailed")}
          </p>
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
          </div>
        )}
      </CardContent>
    </Card>
  );
}
