import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { ClipboardText, WarningCircle } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { EmptyState } from "@/components/common/EmptyState";
import { DiligencePackEditor } from "@/components/deal-rooms/DiligencePackEditor";
import { useAsyncData } from "@/hooks/useAsyncData";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import type {
  DDCoverageRow,
  DDCoverageRowStatus,
  DDCoverageRun,
  DDCrossCheck,
  DDCrossCheckClaimStatus,
  Evidence,
  Link,
} from "@/types";

const SCOPE_ROOM = "room";
const POLL_MS = 2000;
const PACK_FINANCING = "financing_dd_v1";
const PACK_MA = "ma_redflag_v1";

interface DealRoomDiligenceTabProps {
  roomId: string;
}

function scanLang(i18nLang: string): string {
  return i18nLang.toLowerCase().startsWith("zh") ? "zh-CN" : "en";
}

function statusVariant(status: DDCoverageRowStatus): "default" | "secondary" | "destructive" | "outline" {
  switch (status) {
    case "supported":
      return "default";
    case "absent_in_scope":
      return "secondary";
    case "insufficient":
      return "destructive";
    default:
      return "outline";
  }
}

function claimVariant(
  status: DDCrossCheckClaimStatus,
): "default" | "secondary" | "destructive" | "outline" {
  switch (status) {
    case "aligned":
      return "default";
    case "conflict":
      return "destructive";
    case "absent_in_scope":
      return "secondary";
    case "insufficient":
      return "outline";
    default:
      return "outline";
  }
}

export function DealRoomDiligenceTab({ roomId }: DealRoomDiligenceTabProps) {
  const { t, i18n } = useTranslation("dealRooms");
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const navigate = useNavigate();
  const [scope, setScope] = useState<string>(SCOPE_ROOM);
  const [packId, setPackId] = useState<string>(PACK_FINANCING);
  const [scanning, setScanning] = useState(false);
  const [activeRun, setActiveRun] = useState<DDCoverageRun | null>(null);
  const [docA, setDocA] = useState<string>("");
  const [docB, setDocB] = useState<string>("");
  const [crossChecking, setCrossChecking] = useState(false);
  const [crossCheck, setCrossCheck] = useState<DDCrossCheck | null>(null);

  const linkId = scope === SCOPE_ROOM ? undefined : scope;

  const { data, loading, error, refetch } = useAsyncData(async () => {
    const [linksRes, docsRes, kb] = await Promise.all([
      api.getDealRoomLinks(roomId),
      api.getDealRoomDocuments(roomId),
      api.getDealRoomKnowledgeBase(roomId).catch(() => null),
    ]);
    const links = linksRes.data ?? [];
    const folders = docsRes.data ?? [];
    const titleByDocId = new Map<string, string>();
    for (const folder of folders) {
      for (const doc of folder.documents ?? []) {
        titleByDocId.set(doc.document_id, doc.title);
      }
    }
    const activeIds = kb?.active_document_ids ?? kb?.document_ids ?? [];
    const kbDocs = activeIds.map((id) => ({
      id,
      title: titleByDocId.get(id) || id,
    }));
    let packs: Array<{ pack_id: string; pack_version: string }> = [
      { pack_id: PACK_FINANCING, pack_version: "1" },
      { pack_id: PACK_MA, pack_version: "1" },
    ];
    try {
      const listed = await api.listDDCoveragePacks(roomId);
      if (listed.data?.length) {
        packs = listed.data.map((p) => ({
          pack_id: p.pack_id,
          pack_version: p.pack_version,
        }));
      }
    } catch {
      // keep defaults when packs endpoint unavailable
    }
    try {
      const snapshot = await api.getDDCoverageSnapshot(roomId, {
        pack_id: packId,
        link_id: linkId,
      });
      return { links, snapshot, kbDocs, packs, disabled: false as const };
    } catch (e) {
      if (e instanceof ApiError && e.code === "dd_coverage_disabled") {
        return { links, snapshot: null, kbDocs, packs, disabled: true as const };
      }
      if (e instanceof ApiError && (e.status === 404 || e.code === "not_found")) {
        return { links, snapshot: null, kbDocs, packs, disabled: false as const };
      }
      throw e;
    }
  }, [roomId, linkId, packId]);

  const links = useMemo(() => data?.links ?? [], [data?.links]);
  const snapshot = data?.snapshot ?? null;
  const disabled = data?.disabled ?? false;
  const kbDocs = useMemo(() => data?.kbDocs ?? [], [data?.kbDocs]);
  const packs = useMemo(() => data?.packs ?? [], [data?.packs]);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const latest = await api.getDDCrossCheckLatest(roomId, packId);
        if (!cancelled) setCrossCheck(latest);
      } catch {
        if (!cancelled) setCrossCheck(null);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [roomId, packId]);

  const counts = useMemo(() => {
    const rows = snapshot?.coverage_rows ?? [];
    return {
      supported: rows.filter((r) => r.status === "supported").length,
      absent: rows.filter((r) => r.status === "absent_in_scope").length,
      insufficient: rows.filter((r) => r.status === "insufficient").length,
      total: rows.length,
    };
  }, [snapshot?.coverage_rows]);

  useEffect(() => {
    const runId = activeRun?.id;
    const status = activeRun?.status;
    if (!runId || (status !== "queued" && status !== "running")) {
      return;
    }
    let cancelled = false;
    const tick = async () => {
      try {
        const run = await api.getDDCoverageRun(roomId, runId);
        if (cancelled) return;
        setActiveRun(run);
        if (run.status === "succeeded") {
          setScanning(false);
          toast.success(t("diligence.scanSucceeded"));
          await refetch();
        } else if (run.status === "failed") {
          setScanning(false);
          toast.error(run.error_message || t("diligence.scanFailed"));
        }
      } catch {
        if (!cancelled) {
          setScanning(false);
          toast.error(t("diligence.scanFailed"));
        }
      }
    };
    const id = setInterval(() => {
      void tick();
    }, POLL_MS);
    void tick();
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [activeRun?.id, activeRun?.status, roomId, refetch, t]);

  const handleScan = useCallback(async () => {
    setScanning(true);
    try {
      const res = await api.startDDCoverageScan(roomId, {
        pack_id: packId,
        link_id: linkId,
        lang: scanLang(i18n.language),
      });
      setActiveRun(res.run);
      toast.message(t("diligence.scanStarted"));
    } catch (e) {
      setScanning(false);
      if (e instanceof ApiError && e.code === "dd_coverage_disabled") {
        toast.error(t("diligence.disabled"));
        await refetch();
        return;
      }
      if (e instanceof ApiError && e.code === "scan_in_progress") {
        toast.error(t("diligence.scanInProgress"));
        return;
      }
      toast.error(e instanceof Error ? e.message : t("diligence.scanFailed"));
    }
  }, [roomId, packId, linkId, i18n.language, t, refetch]);

  const handleCrossCheck = useCallback(async () => {
    if (!docA || !docB || docA === docB) {
      toast.error(t("diligence.crossCheckDocsRequired"));
      return;
    }
    setCrossChecking(true);
    try {
      const view = await api.startDDCrossCheck(roomId, {
        pack_id: packId,
        document_a_id: docA,
        document_b_id: docB,
        lang: scanLang(i18n.language),
      });
      setCrossCheck(view);
      toast.success(t("diligence.crossCheckSucceeded"));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("diligence.crossCheckFailed"));
    } finally {
      setCrossChecking(false);
    }
  }, [roomId, packId, docA, docB, i18n.language, t]);

  const openClue = useCallback(
    (ev: Evidence) => {
      if (!ev.document_id || !workspaceSlug) return;
      navigate(`/viewer/${ev.document_id}?page=${ev.page_number}`);
    },
    [navigate, workspaceSlug],
  );

  if (disabled) {
    return (
      <Card data-testid="deal-room-diligence-tab">
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <ClipboardText size={20} />
            {t("diligence.title")}
          </CardTitle>
          <CardDescription>{t("diligence.disabled")}</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  const runBusy =
    scanning || activeRun?.status === "queued" || activeRun?.status === "running";
  const showPackEditor = packId === PACK_FINANCING;

  return (
    <div className="space-y-4" data-testid="deal-room-diligence-tab">
      <Card>
        <CardHeader className="pb-3">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
            <div className="space-y-1">
              <CardTitle className="text-h2 flex items-center gap-2">
                <ClipboardText size={20} />
                {t("diligence.title")}
              </CardTitle>
              <CardDescription>{t("diligence.description")}</CardDescription>
            </div>
            <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:flex-wrap">
              <Select
                value={packId}
                onValueChange={(value) => {
                  if (value) setPackId(value);
                }}
              >
                <SelectTrigger className="w-[220px]" aria-label={t("diligence.packSelectLabel")}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {(packs.length
                    ? packs
                    : [
                        { pack_id: PACK_FINANCING, pack_version: "1" },
                        { pack_id: PACK_MA, pack_version: "1" },
                      ]
                  ).map((p) => (
                    <SelectItem key={p.pack_id} value={p.pack_id}>
                      {p.pack_id === PACK_FINANCING
                        ? t("diligence.packs.financing_dd_v1")
                        : p.pack_id === PACK_MA
                          ? t("diligence.packs.ma_redflag_v1")
                          : p.pack_id}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select
                value={scope}
                onValueChange={(value) => {
                  if (value) setScope(value);
                }}
              >
                <SelectTrigger className="w-[240px]" aria-label={t("diligence.scopeLabel")}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={SCOPE_ROOM}>{t("diligence.scopeRoom")}</SelectItem>
                  {links.map((link: Link) => (
                    <SelectItem key={link.id} value={link.id}>
                      {link.name?.trim() || link.documentTitle || link.id}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button
                type="button"
                onClick={() => void handleScan()}
                disabled={runBusy}
                data-testid="diligence-run-scan"
              >
                {runBusy ? t("diligence.scanning") : t("diligence.runScan")}
              </Button>
            </div>
          </div>
          <p className="text-xs text-muted-foreground" data-testid="diligence-scope-hint">
            {linkId ? t("diligence.scopeLinkHint") : t("diligence.scopeRoomHint")}
          </p>
        </CardHeader>
      </Card>

      {showPackEditor ? (
        <DiligencePackEditor roomId={roomId} onPackChanged={() => void refetch()} />
      ) : (
        <p className="text-sm text-muted-foreground" data-testid="diligence-pack-readonly">
          {t("diligence.packReadonly")}
        </p>
      )}

      <Card data-testid="diligence-cross-check">
        <CardHeader className="pb-2">
          <CardTitle className="text-h3">{t("diligence.crossCheckTitle")}</CardTitle>
          <CardDescription>{t("diligence.crossCheckDescription")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:flex-wrap">
            <Select
              value={docA}
              onValueChange={(value) => {
                if (value) setDocA(value);
              }}
            >
              <SelectTrigger className="w-[240px]" aria-label={t("diligence.crossCheckDocA")}>
                <SelectValue placeholder={t("diligence.crossCheckDocA")} />
              </SelectTrigger>
              <SelectContent>
                {kbDocs.map((d) => (
                  <SelectItem key={`a-${d.id}`} value={d.id}>
                    {d.title}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select
              value={docB}
              onValueChange={(value) => {
                if (value) setDocB(value);
              }}
            >
              <SelectTrigger className="w-[240px]" aria-label={t("diligence.crossCheckDocB")}>
                <SelectValue placeholder={t("diligence.crossCheckDocB")} />
              </SelectTrigger>
              <SelectContent>
                {kbDocs.map((d) => (
                  <SelectItem key={`b-${d.id}`} value={d.id}>
                    {d.title}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button
              type="button"
              variant="outline"
              onClick={() => void handleCrossCheck()}
              disabled={crossChecking || kbDocs.length < 2}
              data-testid="diligence-run-cross-check"
            >
              {crossChecking ? t("diligence.crossChecking") : t("diligence.runCrossCheck")}
            </Button>
          </div>
          {kbDocs.length < 2 ? (
            <p className="text-sm text-muted-foreground">{t("diligence.crossCheckNeedDocs")}</p>
          ) : null}
          {crossCheck ? (
            <div className="space-y-2" data-testid="diligence-cross-check-results">
              <p className="text-sm text-muted-foreground">
                {t("diligence.crossCheckSummary", {
                  conflict: crossCheck.claims.filter((c) => c.status === "conflict").length,
                  total: crossCheck.claims.length,
                })}
              </p>
              {crossCheck.claims.map((claim) => (
                <div
                  key={claim.item_id}
                  className="rounded-lg border border-border p-3"
                  data-testid={`diligence-claim-${claim.item_id}`}
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <p className="text-sm font-medium">{claim.label}</p>
                    <Badge variant={claimVariant(claim.status)}>
                      {t(`diligence.claimStatus.${claim.status}`)}
                    </Badge>
                  </div>
                  {claim.error ? (
                    <p className="mt-1 text-xs text-destructive">{claim.error}</p>
                  ) : null}
                  <div className="mt-2 grid gap-2 md:grid-cols-2">
                    <div>
                      <p className="text-xs text-muted-foreground mb-1">{t("diligence.crossCheckDocA")}</p>
                      {(claim.clues_a ?? []).map((ev) => (
                        <button
                          key={ev.chunk_id}
                          type="button"
                          className="mb-1 w-full rounded-md border border-border/80 bg-muted/30 px-2 py-1.5 text-left text-xs hover:bg-muted/60"
                          onClick={() => openClue(ev)}
                        >
                          {ev.quote}
                        </button>
                      ))}
                    </div>
                    <div>
                      <p className="text-xs text-muted-foreground mb-1">{t("diligence.crossCheckDocB")}</p>
                      {(claim.clues_b ?? []).map((ev) => (
                        <button
                          key={ev.chunk_id}
                          type="button"
                          className="mb-1 w-full rounded-md border border-border/80 bg-muted/30 px-2 py-1.5 text-left text-xs hover:bg-muted/60"
                          onClick={() => openClue(ev)}
                        >
                          {ev.quote}
                        </button>
                      ))}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          ) : null}
        </CardContent>
      </Card>

      {snapshot?.stale ? (
        <div
          className="flex items-start gap-2 rounded-lg border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-sm"
          data-testid="diligence-stale-banner"
          role="status"
        >
          <WarningCircle size={18} className="mt-0.5 shrink-0 text-amber-700 dark:text-amber-400" />
          <div>
            <p className="font-medium">{t("diligence.staleTitle")}</p>
            <p className="text-muted-foreground">{t("diligence.staleDescription")}</p>
          </div>
        </div>
      ) : null}

      {runBusy && activeRun ? (
        <p className="text-sm text-muted-foreground" data-testid="diligence-run-status">
          {t("diligence.runStatus", { status: t(`diligence.run.${activeRun.status}`) })}
        </p>
      ) : null}

      {loading ? (
        <p className="text-sm text-muted-foreground">{t("diligence.loading")}</p>
      ) : error ? (
        <div className="flex flex-col items-start gap-2">
          <p className="text-sm text-destructive">{error}</p>
          <Button type="button" variant="outline" size="sm" onClick={() => void refetch()}>
            {t("diligence.retry")}
          </Button>
        </div>
      ) : !snapshot ? (
        <EmptyState
          icon={<ClipboardText size={40} />}
          title={t("diligence.emptyTitle")}
          description={t("diligence.emptyDescription")}
        />
      ) : (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-h3">{t("diligence.resultsTitle")}</CardTitle>
            <CardDescription>
              {t("diligence.resultsSummary", {
                supported: counts.supported,
                absent: counts.absent,
                insufficient: counts.insufficient,
                total: counts.total,
              })}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {(snapshot.coverage_rows ?? []).map((row) => (
              <CoverageRowCard key={row.item_id} row={row} onOpenClue={openClue} t={t} />
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  );
}

function CoverageRowCard({
  row,
  onOpenClue,
  t,
}: {
  row: DDCoverageRow;
  onOpenClue: (ev: Evidence) => void;
  t: (key: string, opts?: Record<string, unknown>) => string;
}) {
  return (
    <div
      className="rounded-lg border border-border p-3"
      data-testid={`diligence-row-${row.item_id}`}
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-sm font-medium">{row.label}</p>
        <Badge variant={statusVariant(row.status)}>
          {t(`diligence.status.${row.status}`)}
        </Badge>
      </div>
      {row.extracted_value ? (
        <p className="mt-1 text-sm text-foreground" data-testid={`diligence-extracted-${row.item_id}`}>
          <span className="text-muted-foreground">{t("diligence.extractedValue")}: </span>
          <span className="font-medium tabular-nums">{row.extracted_value}</span>
          {row.value_type === "percent" ||
          row.value_type === "money" ||
          row.value_type === "share" ? (
            <span className="ml-1 text-xs text-muted-foreground">
              ({t(`diligence.valueType.${row.value_type}`)})
            </span>
          ) : null}
        </p>
      ) : null}
      {row.error ? (
        <p className="mt-1 text-xs text-destructive">{row.error}</p>
      ) : null}
      {row.clues?.length > 0 ? (
        <ul className="mt-2 space-y-2">
          {row.clues.map((ev) => (
            <li key={ev.chunk_id}>
              <button
                type="button"
                className="w-full rounded-md border border-border/80 bg-muted/30 px-3 py-2 text-left text-sm transition-colors hover:bg-muted/60"
                onClick={() => onOpenClue(ev)}
                data-testid={`diligence-clue-${ev.chunk_id}`}
              >
                <p className="text-xs text-muted-foreground">
                  {t("diligence.cluePage", { page: ev.page_number })}
                </p>
                <p className="mt-1 line-clamp-3">{ev.quote}</p>
              </button>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
