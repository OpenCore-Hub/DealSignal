import { useEffect, useState } from "react";
import { FileMagnifyingGlass } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { EmptyState } from "@/components/common/EmptyState";
import {
  InsightsRangeControls,
  useInsightsRange,
} from "@/components/insights/InsightsRangeControls";
import {
  api,
  type KeyPageComplianceEvent,
  type KeyPageHeatCircle,
  type KeyPageSettings,
} from "@/lib/api";
import { keyPageRulesForCircle, keywordLangFromI18n } from "@/lib/heat/heatScore";
import { KEY_PAGE_ENGAGED_MIN_SECONDS, isKeyPageEngaged } from "@/lib/heat/keyPageEngage";
import {
  draftsFromExtras,
  editorCategoriesForCircle,
  extrasFromDrafts,
} from "@/lib/heat/keyPageExtrasDraft";
import { useAsyncData } from "@/hooks/useAsyncData";
import { useTranslation } from "react-i18next";
import { displayablePageTitle } from "@/lib/insights/pageTitleDisplay";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import type { Circle } from "@/types";

const CIRCLE_OPTIONS: KeyPageHeatCircle[] = ["founder", "investor_ir", "sales"];
const PAGE_SIZE = 25;

function isHeatCircle(v: string): v is KeyPageHeatCircle {
  return v === "founder" || v === "investor_ir" || v === "sales";
}

function asCircle(v: KeyPageHeatCircle): Circle {
  return v;
}

function categoryLabel(
  t: (key: string, opts?: Record<string, unknown>) => string,
  category: string,
): string {
  const key = `keyPages.categories.${category}`;
  const labeled = t(key);
  return labeled === key ? category : labeled;
}

function PageTitleCell({ title, page }: { title: string; page: number }) {
  const { t } = useTranslation("insights");
  const label = displayablePageTitle(title);
  return (
    <td className="max-w-[16rem] px-2 py-2">
      {label ? (
        <div className="truncate" title={label}>
          {label}
        </div>
      ) : null}
      <div className="text-caption text-muted-foreground">
        {t("keyPages.pageNumber", { page })}
      </div>
    </td>
  );
}

function downloadKeyPagesCsv(events: KeyPageComplianceEvent[], filename: string) {
  const header = [
    "created_at",
    "visitor_email",
    "visitor_id",
    "document_title",
    "page_number",
    "page_title",
    "category",
    "duration_seconds",
    "deal_room_name",
    "deal_room_id",
    "link_id",
    "document_id",
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
        e.visitorEmail ?? "",
        e.visitorId ?? "",
        e.documentTitle,
        String(e.pageNumber),
        displayablePageTitle(e.pageTitle),
        e.category,
        String(e.durationSeconds),
        e.dealRoomName,
        e.dealRoomId ?? "",
        e.linkId ?? "",
        e.documentId ?? "",
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

export function InsightsKeyPagesPage() {
  const { t } = useTranslation("insights");
  const { t: tc } = useTranslation("common");
  const { i18n } = useTranslation();
  const locale = i18n.language;
  const rangeCtl = useInsightsRange(30);
  const [circle, setCircle] = useState<KeyPageHeatCircle>("founder");
  const [offset, setOffset] = useState(0);
  const [defaultCircle, setDefaultCircle] = useState<KeyPageHeatCircle>("founder");
  const [extraDrafts, setExtraDrafts] = useState<Record<string, string>>({});
  const [settingsExtra, setSettingsExtra] = useState<Record<string, string[]>>({});
  const [canEdit, setCanEdit] = useState(false);
  const [saving, setSaving] = useState(false);
  const [settingsReady, setSettingsReady] = useState(false);
  const [showKeywordsCard, setShowKeywordsCard] = useState(false);

  const {
    data: settings,
    loading: settingsLoading,
    refetch: refetchSettings,
  } = useAsyncData(() => api.getKeyPageSettings(), [i18n.language]);

  useEffect(() => {
    if (!settings) return;
    const dc = isHeatCircle(settings.defaultCircle) ? settings.defaultCircle : "founder";
    setDefaultCircle(dc);
    if (!settingsReady) {
      setCircle(dc);
      setSettingsReady(true);
    }
    const extras = settings.extraKeywords ?? {};
    setSettingsExtra(extras);
    setExtraDrafts(draftsFromExtras(editorCategoriesForCircle(asCircle(dc), extras), extras));
    setCanEdit(Boolean(settings.canEdit));
  }, [settings, settingsReady]);

  useEffect(() => {
    if (!settingsReady) return;
    setExtraDrafts((prev) => {
      const cats = editorCategoriesForCircle(asCircle(defaultCircle), settingsExtra);
      const next = draftsFromExtras(cats, settingsExtra);
      for (const cat of cats) {
        if (prev[cat] !== undefined) next[cat] = prev[cat];
      }
      return next;
    });
  }, [defaultCircle, settingsExtra, settingsReady]);

  const { data, loading, error, refetch } = useAsyncData(
    () =>
      api.getKeyPageCompliance({
        ...rangeCtl.apiParams,
        circle,
        limit: PAGE_SIZE,
        offset,
      }),
    [rangeCtl.apiParams, circle, offset, i18n.language],
  );

  const saveSettings = async () => {
    setSaving(true);
    try {
      const nextExtra = extrasFromDrafts(extraDrafts);
      const saved: KeyPageSettings = await api.saveKeyPageSettings({
        defaultCircle,
        extraKeywords: nextExtra,
      });
      const extras = saved.extraKeywords ?? {};
      setSettingsExtra(extras);
      setExtraDrafts(
        draftsFromExtras(editorCategoriesForCircle(asCircle(defaultCircle), extras), extras),
      );
      setCanEdit(Boolean(saved.canEdit));
      toast.success(t("keyPages.saveSettingsSuccess"));
      await Promise.all([refetch(), refetchSettings()]);
    } catch {
      toast.error(t("keyPages.saveSettingsFailed"));
    } finally {
      setSaving(false);
    }
  };

  const editorCategories = editorCategoriesForCircle(asCircle(defaultCircle), settingsExtra);
  const keywordLang = keywordLangFromI18n(i18n.language);
  // Prefer API builtins (already locale-filtered via Accept-Language); fall back to local mirror.
  const builtinByCategory = new Map(
    (
      settings?.builtinRules?.length
        ? settings.builtinRules
        : keyPageRulesForCircle(asCircle(defaultCircle), keywordLang)
    ).map((r) => [r.category, r.keywords]),
  );

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
        <p className="text-body text-muted-foreground">{error}</p>
        <Button onClick={refetch}>{tc("retry")}</Button>
      </div>
    );
  }

  if (loading || settingsLoading || !data) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-64" />
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-4">
          <Skeleton className="h-28" />
          <Skeleton className="h-28" />
          <Skeleton className="h-28" />
          <Skeleton className="h-28" />
        </div>
        <Skeleton className="h-64" />
      </div>
    );
  }

  const hasViews = data.totalViews > 0;

  return (
    <div className="space-y-6">
      <div
        className="flex items-center justify-between gap-4 rounded-lg border border-border bg-card px-4 py-3"
        data-testid="insights-key-pages-keywords-toggle"
      >
        <div className="min-w-0 space-y-0.5">
          <Label
            htmlFor="insights-key-pages-keywords-switch"
            className="text-sm font-medium text-foreground"
          >
            {t("keyPages.settingsTitle")}
          </Label>
          <p className="text-caption text-muted-foreground">{t("keyPages.settingsToggleHint")}</p>
        </div>
        <Switch
          id="insights-key-pages-keywords-switch"
          checked={showKeywordsCard}
          onCheckedChange={setShowKeywordsCard}
          aria-label={t("keyPages.settingsTitle")}
        />
      </div>

      {showKeywordsCard ? (
        <div
          className="rounded-lg border border-border bg-card px-4 py-3 space-y-3"
          data-testid="insights-key-pages-settings"
        >
          <div>
            <p className="text-sm font-medium text-foreground">{t("keyPages.settingsTitle")}</p>
            <p className="text-caption text-muted-foreground">{t("keyPages.settingsHint")}</p>
          </div>
          <div className="space-y-1.5">
            <p className="text-caption text-muted-foreground">{t("keyPages.defaultCircleLabel")}</p>
            <div className="inline-flex rounded-md border border-border p-0.5" role="group">
              {CIRCLE_OPTIONS.map((c) => (
                <button
                  key={c}
                  type="button"
                  disabled={!canEdit || saving}
                  onClick={() => setDefaultCircle(c)}
                  className={cn(
                    "rounded px-3 py-1.5 text-sm font-medium transition-colors disabled:opacity-50",
                    defaultCircle === c
                      ? "bg-rose-100 text-rose-800"
                      : "text-muted-foreground hover:bg-rose-50 hover:text-rose-700",
                  )}
                >
                  {t(`keyPages.circles.${c}`)}
                </button>
              ))}
            </div>
          </div>
          <div className="space-y-2" data-testid="insights-key-pages-category-extras">
            <div>
              <p className="text-caption font-medium text-foreground">
                {t("keyPages.categoryExtrasTitle")}
              </p>
              <p className="text-caption text-muted-foreground">
                {t("keyPages.categoryExtrasHint")}
              </p>
            </div>
            <div className="grid gap-3 md:grid-cols-2">
              {editorCategories.map((cat) => {
                const builtins = builtinByCategory.get(cat) ?? [];
                const inputId = `key-pages-extra-${cat}`;
                return (
                  <div
                    key={cat}
                    className="space-y-2 rounded-md border border-border/80 bg-background/60 px-3 py-2.5"
                    data-testid={`insights-key-pages-extra-${cat}`}
                  >
                    <p className="text-sm font-medium text-foreground">{categoryLabel(t, cat)}</p>
                    <div className="space-y-1">
                      <p className="text-caption text-muted-foreground">
                        {t("keyPages.builtinKeywordsLabel")}
                      </p>
                      {builtins.length > 0 ? (
                        <div className="flex flex-wrap gap-1">
                          {builtins.map((kw) => (
                            <span
                              key={kw}
                              className="rounded bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground"
                            >
                              {kw}
                            </span>
                          ))}
                        </div>
                      ) : (
                        <p className="text-caption text-muted-foreground">
                          {t("keyPages.noBuiltinKeywords")}
                        </p>
                      )}
                    </div>
                    <div className="space-y-1">
                      <label className="text-caption text-muted-foreground" htmlFor={inputId}>
                        {t("keyPages.extraKeywordsLabel")}
                      </label>
                      <textarea
                        id={inputId}
                        data-testid={`insights-key-pages-extra-input-${cat}`}
                        className="min-h-[56px] w-full rounded-md border border-border bg-background px-2.5 py-1.5 text-sm"
                        value={extraDrafts[cat] ?? ""}
                        disabled={!canEdit || saving}
                        placeholder={
                          cat === "custom"
                            ? t("keyPages.customKeywordsPlaceholder")
                            : t("keyPages.extraKeywordsPlaceholder")
                        }
                        onChange={(e) =>
                          setExtraDrafts((prev) => ({ ...prev, [cat]: e.target.value }))
                        }
                      />
                    </div>
                  </div>
                );
              })}
            </div>
            <p className="text-caption text-muted-foreground">{t("keyPages.customKeywordsHint")}</p>
          </div>
          {canEdit ? (
            <Button size="sm" disabled={saving} onClick={saveSettings}>
              {t("keyPages.saveSettings")}
            </Button>
          ) : (
            <p className="text-caption text-muted-foreground">{t("keyPages.settingsReadOnly")}</p>
          )}
        </div>
      ) : null}

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
        <div className="flex flex-wrap items-center gap-2 self-start sm:self-center">
          <div
            className="inline-flex rounded-md border border-border p-0.5"
            role="group"
            aria-label={t("keyPages.circleLabel")}
          >
            {CIRCLE_OPTIONS.map((c) => (
              <button
                key={c}
                type="button"
                onClick={() => {
                  setCircle(c);
                  setOffset(0);
                }}
                className={cn(
                  "rounded px-3 py-1.5 text-sm font-medium transition-colors",
                  circle === c
                    ? "bg-rose-100 text-rose-800"
                    : "text-muted-foreground hover:bg-rose-50 hover:text-rose-700",
                )}
              >
                {t(`keyPages.circles.${c}`)}
              </button>
            ))}
          </div>
          <Button
            variant="outline"
            className="shrink-0"
            disabled={data.events.length === 0}
            onClick={() => {
              const name =
                data.rangeFrom && data.rangeTo
                  ? `key-pages-${circle}-${data.rangeFrom}_${data.rangeTo}-offset-${offset}.csv`
                  : `key-pages-${circle}-${data.rangeDays}d-offset-${offset}.csv`;
              downloadKeyPagesCsv(data.events, name);
            }}
          >
            {t("keyPages.exportCsv")}
          </Button>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("keyPages.kpiViews")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold tabular-nums">{data.totalViews}</p>
            <p className="text-caption text-muted-foreground">
              {t("keyPages.kpiViewsHint", {
                engaged: data.engagedViews,
                days: data.rangeDays,
              })}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("keyPages.kpiVisitors")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold tabular-nums">{data.uniqueVisitors}</p>
            <p className="text-caption text-muted-foreground">
              {t("keyPages.kpiVisitorsHint")}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("keyPages.kpiPages")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold tabular-nums">{data.distinctPages}</p>
            <p className="text-caption text-muted-foreground">
              {t("keyPages.kpiPagesHint")}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("keyPages.byCategoryTitle")}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {data.byCategory.length === 0 ? (
              <p className="text-caption text-muted-foreground">{t("keyPages.bucketsEmpty")}</p>
            ) : (
              data.byCategory.slice(0, 5).map((row) => (
                <div
                  key={row.category}
                  className="flex w-full items-center justify-between px-1 text-sm"
                >
                  <span className="truncate">{categoryLabel(t, row.category)}</span>
                  <span className="tabular-nums text-muted-foreground">{row.count}</span>
                </div>
              ))
            )}
          </CardContent>
        </Card>
      </div>

      {!hasViews ? (
        <EmptyState
          icon={<FileMagnifyingGlass size={48} />}
          title={t("keyPages.emptyTitle")}
          description={t("keyPages.emptyDescription")}
          size="large"
        />
      ) : (
        <>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-h2">{t("keyPages.pagesTitle")}</CardTitle>
            </CardHeader>
            <CardContent className="overflow-x-auto">
              <table className="w-full min-w-[640px] text-left text-sm">
                <thead>
                  <tr className="border-b border-border text-muted-foreground">
                    <th className="px-2 py-2 font-medium">{t("keyPages.colDocument")}</th>
                    <th className="px-2 py-2 font-medium">{t("keyPages.colPage")}</th>
                    <th className="px-2 py-2 font-medium">{t("keyPages.colCategory")}</th>
                    <th className="px-2 py-2 font-medium" title={t("keyPages.colViewsHint")}>
                      {t("keyPages.colViews")}
                    </th>
                    <th className="px-2 py-2 font-medium">{t("keyPages.colVisitors")}</th>
                    <th className="px-2 py-2 font-medium">{t("keyPages.colAvgDwell")}</th>
                  </tr>
                </thead>
                <tbody>
                  {data.pages.map((p) => (
                    <tr
                      key={`${p.documentId}-${p.pageNumber}`}
                      className="border-b border-border/60 last:border-0"
                    >
                      <td className="max-w-[220px] truncate px-2 py-2 font-medium">
                        {p.documentTitle || t("access.unknownDocument")}
                      </td>
                      <PageTitleCell title={p.pageTitle} page={p.pageNumber} />
                      <td className="px-2 py-2">{categoryLabel(t, p.category)}</td>
                      <td className="px-2 py-2 tabular-nums" title={t("keyPages.colViewsHint")}>
                        {t("keyPages.colViewsSplit", {
                          engaged: p.engagedViews ?? 0,
                          total: p.views,
                        })}
                      </td>
                      <td className="px-2 py-2 tabular-nums">{p.uniqueVisitors}</td>
                      <td className="px-2 py-2 tabular-nums text-muted-foreground">
                        {t("keyPages.avgDwellValue", {
                          seconds: Math.round(p.avgDurationSeconds),
                        })}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-h2">{t("keyPages.eventsTitle")}</CardTitle>
            </CardHeader>
            <CardContent className="overflow-x-auto">
              <table className="w-full min-w-[720px] text-left text-sm">
                <thead>
                  <tr className="border-b border-border text-muted-foreground">
                    <th className="px-2 py-2 font-medium">{t("access.colTime")}</th>
                    <th className="px-2 py-2 font-medium">{t("access.colActor")}</th>
                    <th className="px-2 py-2 font-medium">{t("keyPages.colDocument")}</th>
                    <th className="px-2 py-2 font-medium">{t("keyPages.colPage")}</th>
                    <th className="px-2 py-2 font-medium">{t("keyPages.colCategory")}</th>
                    <th
                      className="px-2 py-2 font-medium"
                      title={t("keyPages.colDwellHint", { seconds: KEY_PAGE_ENGAGED_MIN_SECONDS })}
                    >
                      {t("keyPages.colDwell")}
                    </th>
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
                      <td className="px-2 py-2">
                        {e.visitorEmail || e.visitorId || t("access.anonymous")}
                      </td>
                      <td className="px-2 py-2">
                        <div className="truncate font-medium">
                          {e.documentTitle || e.dealRoomName || t("access.unknownDocument")}
                        </div>
                        <div className="truncate text-caption text-muted-foreground">
                          {e.dealRoomName || t("access.libraryScope")}
                        </div>
                      </td>
                      <PageTitleCell title={e.pageTitle} page={e.pageNumber} />
                      <td className="px-2 py-2">{categoryLabel(t, e.category)}</td>
                      <td className="px-2 py-2 tabular-nums text-muted-foreground">
                        <div>{t("keyPages.avgDwellValue", { seconds: e.durationSeconds })}</div>
                        {!isKeyPageEngaged(e.durationSeconds) ? (
                          <div
                            className="text-caption"
                            title={t("keyPages.eventSkimHint", {
                              seconds: KEY_PAGE_ENGAGED_MIN_SECONDS,
                            })}
                          >
                            {t("keyPages.eventSkim")}
                          </div>
                        ) : null}
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
        </>
      )}
    </div>
  );
}
