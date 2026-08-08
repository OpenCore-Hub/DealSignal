import { useState } from "react";
import { ShieldWarning } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/common/EmptyState";
import {
  InsightsRangeControls,
  useInsightsRange,
} from "@/components/insights/InsightsRangeControls";
import { api, type AccessAuditEvent } from "@/lib/api";
import { useAsyncData } from "@/hooks/useAsyncData";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";

const PAGE_SIZE = 25;

function eventTypeLabel(
  t: (key: string, opts?: Record<string, unknown>) => string,
  eventType: string,
): string {
  const key = `access.eventTypes.${eventType}`;
  const labeled = t(key);
  return labeled === key ? eventType : labeled;
}

function downloadAccessCsv(events: AccessAuditEvent[], filename: string) {
  const header = [
    "created_at",
    "event_type",
    "email",
    "visitor_id",
    "document_title",
    "deal_room_name",
    "deal_room_id",
    "folder_path",
    "member_email",
    "member_id",
    "link_id",
    "reason",
  ];
  const escape = (v: string) => {
    if (/[",\n]/.test(v)) return `"${v.replace(/"/g, '""')}"`;
    return v;
  };
  const lines = [header.join(",")];
  for (const e of events) {
    lines.push(
      [
        e.createdAt,
        e.eventType,
        e.email ?? "",
        e.visitorId ?? "",
        e.documentTitle,
        e.dealRoomName,
        e.dealRoomId ?? "",
        e.folderPath ?? "",
        e.memberEmail ?? "",
        e.memberId ?? "",
        e.linkId ?? "",
        e.reason ?? "",
      ]
        .map((c) => escape(String(c)))
        .join(","),
    );
  }
  const blob = new Blob([`${lines.join("\n")}\n`], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

export function InsightsAccessPage() {
  const { t } = useTranslation("insights");
  const { t: tc } = useTranslation("common");
  const { i18n } = useTranslation();
  const locale = i18n.language;
  const rangeCtl = useInsightsRange(30);
  const [eventType, setEventType] = useState<string>("");
  const [memberId, setMemberId] = useState<string>("");
  const [offset, setOffset] = useState(0);

  const { data, loading, error, refetch } = useAsyncData(
    () =>
      api.getAccessAudit({
        ...rangeCtl.apiParams,
        eventType: eventType || undefined,
        memberId: memberId || undefined,
        limit: PAGE_SIZE,
        offset,
      }),
    [rangeCtl.apiParams, eventType, memberId, offset],
  );

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
        <p className="text-body text-muted-foreground">{error}</p>
        <Button onClick={refetch}>{tc("retry")}</Button>
      </div>
    );
  }

  if (loading || !data) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-64" />
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Skeleton className="h-40" />
          <Skeleton className="h-40" />
          <Skeleton className="h-40" />
        </div>
        <Skeleton className="h-64" />
      </div>
    );
  }

  const hasEvents = data.totalEvents > 0;
  const byMember = data.byMember ?? [];
  const hasActiveFilters = Boolean(eventType || memberId);

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <InsightsRangeControls
          variant="chips"
          className="w-full max-w-xl"
          range={rangeCtl.range}
          customOpen={rangeCtl.customOpen}
          draftFrom={rangeCtl.draftFrom}
          draftTo={rangeCtl.draftTo}
          rangeError={rangeCtl.rangeError}
          onSelectPreset={(days) => {
            rangeCtl.selectPreset(days);
            setOffset(0);
          }}
          onOpenCustom={rangeCtl.openCustom}
          onDraftFromChange={rangeCtl.setDraftFrom}
          onDraftToChange={rangeCtl.setDraftTo}
          onApplyCustom={() => {
            if (rangeCtl.applyCustom()) setOffset(0);
          }}
        />
        <Button
          variant="outline"
          className="shrink-0 self-start sm:self-center"
          disabled={data.events.length === 0}
          onClick={() => {
            const name =
              data.rangeFrom && data.rangeTo
                ? `access-audit-${data.rangeFrom}_${data.rangeTo}-offset-${offset}.csv`
                : `access-audit-${data.rangeDays}d-offset-${offset}.csv`;
            downloadAccessCsv(data.events, name);
          }}
        >
          {t("access.exportCsv")}
        </Button>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("access.kpiTotal")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold tabular-nums">{data.totalEvents}</p>
            <p className="text-caption text-muted-foreground">
              {t("access.kpiTotalHint", { days: data.rangeDays })}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("access.byTypeTitle")}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {data.byType.length === 0 ? (
              <p className="text-caption text-muted-foreground">{t("access.bucketsEmpty")}</p>
            ) : (
              data.byType.slice(0, 5).map((row) => (
                <button
                  key={row.eventType}
                  type="button"
                  onClick={() => {
                    setEventType((prev) => (prev === row.eventType ? "" : row.eventType));
                    setOffset(0);
                  }}
                  className={cn(
                    "flex w-full items-center justify-between rounded px-2 py-1 text-left text-sm transition-colors hover:bg-muted/60",
                    eventType === row.eventType && "bg-muted",
                  )}
                >
                  <span className="truncate">{eventTypeLabel(t, row.eventType)}</span>
                  <span className="tabular-nums text-muted-foreground">{row.count}</span>
                </button>
              ))
            )}
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("access.byMemberTitle")}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {byMember.length === 0 ? (
              <p className="text-caption text-muted-foreground">{t("access.bucketsEmpty")}</p>
            ) : (
              byMember.slice(0, 5).map((row) => {
                const id = row.memberId ?? "";
                const name =
                  row.scope === "unknown" || !row.memberId
                    ? t("access.unknownMember")
                    : row.memberEmail || t("access.unknownMember");
                return (
                  <button
                    key={id || "unknown-member"}
                    type="button"
                    disabled={!id}
                    onClick={() => {
                      if (!id) return;
                      setMemberId((prev) => (prev === id ? "" : id));
                      setOffset(0);
                    }}
                    className={cn(
                      "flex w-full items-center justify-between rounded px-2 py-1 text-left text-sm transition-colors",
                      id ? "hover:bg-muted/60" : "cursor-default opacity-90",
                      memberId === id && id && "bg-muted",
                    )}
                  >
                    <span className="truncate">{name}</span>
                    <span className="tabular-nums text-muted-foreground">{row.count}</span>
                  </button>
                );
              })
            )}
          </CardContent>
        </Card>
      </div>

      {hasActiveFilters && (
        <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
          <span>{t("access.activeFilters")}</span>
          {eventType ? (
            <Button
              size="sm"
              variant="secondary"
              onClick={() => {
                setEventType("");
                setOffset(0);
              }}
            >
              {eventTypeLabel(t, eventType)}
            </Button>
          ) : null}
          {memberId ? (
            <Button
              size="sm"
              variant="secondary"
              onClick={() => {
                setMemberId("");
                setOffset(0);
              }}
            >
              {byMember.find((r) => r.memberId === memberId)?.memberEmail || memberId}
            </Button>
          ) : null}
          <Button
            size="sm"
            variant="ghost"
            onClick={() => {
              setEventType("");
              setMemberId("");
              setOffset(0);
            }}
          >
            {t("access.clearFilters")}
          </Button>
        </div>
      )}

      {!hasEvents ? (
        <EmptyState
          icon={<ShieldWarning size={48} />}
          title={t("access.emptyTitle")}
          description={t("access.emptyDescription")}
          size="large"
        />
      ) : (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-h2">{t("access.eventsTitle")}</CardTitle>
          </CardHeader>
          <CardContent className="overflow-x-auto">
            <table className="w-full min-w-[800px] text-left text-sm">
              <thead>
                <tr className="border-b border-border text-muted-foreground">
                  <th className="px-2 py-2 font-medium">{t("access.colTime")}</th>
                  <th className="px-2 py-2 font-medium">{t("access.colType")}</th>
                  <th className="px-2 py-2 font-medium">{t("access.colActor")}</th>
                  <th className="px-2 py-2 font-medium">{t("access.colTarget")}</th>
                  <th className="px-2 py-2 font-medium">{t("access.colMember")}</th>
                  <th className="px-2 py-2 font-medium">{t("access.colReason")}</th>
                </tr>
              </thead>
              <tbody>
                {data.events.map((e) => (
                  <tr key={e.id} className="border-b border-border/60 last:border-0">
                    <td className="whitespace-nowrap px-2 py-2 text-muted-foreground">
                      {new Date(e.createdAt).toLocaleString(locale, {
                        month: "short",
                        day: "numeric",
                        hour: "2-digit",
                        minute: "2-digit",
                      })}
                    </td>
                    <td className="px-2 py-2">{eventTypeLabel(t, e.eventType)}</td>
                    <td className="px-2 py-2">
                      {e.email || e.visitorId || t("access.anonymous")}
                    </td>
                    <td className="px-2 py-2">
                      <div className="truncate font-medium">
                        {e.documentTitle || e.dealRoomName || t("access.unknownDocument")}
                      </div>
                      <div className="truncate text-caption text-muted-foreground">
                        {e.dealRoomName || t("access.libraryScope")}
                        {e.folderPath ? ` · ${e.folderPath}` : ""}
                      </div>
                    </td>
                    <td className="max-w-[160px] truncate px-2 py-2 text-muted-foreground">
                      {e.memberEmail || t("access.unknownMember")}
                    </td>
                    <td className="max-w-[220px] truncate px-2 py-2 text-muted-foreground">
                      {e.reason || "—"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            <div className="mt-4 flex items-center justify-between gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={offset === 0}
                onClick={() => setOffset((o) => Math.max(0, o - PAGE_SIZE))}
              >
                {t("access.prevPage")}
              </Button>
              <span className="text-caption text-muted-foreground">
                {t("access.pageHint", { offset: offset + 1, limit: PAGE_SIZE })}
              </span>
              <Button
                variant="outline"
                size="sm"
                disabled={!data.hasMore}
                onClick={() => setOffset((o) => o + PAGE_SIZE)}
              >
                {t("access.nextPage")}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
